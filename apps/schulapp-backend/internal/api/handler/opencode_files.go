package handler

// Upload → Sandbox, Outputs → Download (Lern-KI-Dateien).
//
// Layout (pro Backend-Session-Unterordner, da sich mehrere Sessions einen
// User-Workspace teilen):
//
//	<workspace>/uploads/s{id}/<uuid>-<safe>   (User-Uploads, nur lesen)
//	<workspace>/outputs/s{id}/<name>          (Agent-Outputs, Download)
//
// Alle Zugriffe: JWT + openCodeSessionOwned + filepath.Clean + Prefix-Check
// gegen Traversal. Uploads werden nie ausgeführt, nur via read-Tool gelesen.
// Outputs-Cap: max 50 Dateien / 100 MB pro Session (Liste). Session-Delete
// räumt beide Unterordner best-effort weg (kein TTL per Entscheidung).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	openCodeMaxUploadBytes   = 10 << 20 // 10 MB/Datei
	openCodeMaxUploadFiles   = 5
	openCodeMaxUploadRequest = 55 << 20 // 5×10 MB + Overhead
	openCodeMaxOutputFiles   = 50
	openCodeMaxOutputBytes   = 100 << 20 // 100 MB/Session
)

// openCodeUploadExts ist die Allowlist für Uploads (kleingeschrieben, ohne Punkt).
var openCodeUploadExts = map[string]bool{
	"pdf": true, "txt": true, "md": true, "png": true, "jpg": true,
	"jpeg": true, "csv": true, "docx": true, "xlsx": true, "pptx": true,
}

// openCodeInlineMIME darf inline (Browser-Ansicht), Rest als Attachment.
var openCodeInlineMIME = map[string]bool{
	"application/pdf": true, "text/plain": true, "text/markdown": true,
	"text/csv": true, "image/png": true, "image/jpeg": true,
}

func openCodeUploadsDir(workspace string, sessionID int) string {
	return filepath.Join(workspace, "uploads", "s"+strconv.Itoa(sessionID))
}

func openCodeOutputsDir(workspace string, sessionID int) string {
	return filepath.Join(workspace, "outputs", "s"+strconv.Itoa(sessionID))
}

// openCodeSafeName sanitized Dateinamen: Base + Whitelist-Zeichen, Rest "_".
func openCodeSafeName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == "/" {
		return ""
	}
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('_')
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		return ""
	}
	if len(out) > 120 {
		ext := strings.ToLower(filepath.Ext(out))
		out = out[:120-len(ext)] + ext
	}
	return out
}

func openCodeUploadExtOK(name string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	return openCodeUploadExts[ext]
}

func openCodeNewSuffix() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

// openCodeResolveInDir löst name gegen dir auf (Traversal-Guard).
func openCodeResolveInDir(dir, name string) (string, bool) {
	safe := filepath.Base(strings.TrimSpace(name))
	if safe == "" || safe == "." || safe == "/" {
		return "", false
	}
	cleanDir := filepath.Clean(dir)
	joined := filepath.Join(cleanDir, safe)
	rel, err := filepath.Rel(cleanDir, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return joined, true
}

// openCodeFileView ist der JSON-Vertrag für Uploads/Outputs:
// {name,size,mime,created_at,download_url?}.
type openCodeFileView struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Mime        string `json:"mime"`
	CreatedAt   string `json:"created_at"`
	DownloadURL string `json:"download_url,omitempty"`
}

func openCodeMimeFor(name string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")) {
	case "pdf":
		return "application/pdf"
	case "txt", "md":
		return "text/plain"
	case "csv":
		return "text/csv"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}

// listSessionOutputs liest outputs/s{id}/ (sortiert newest-first, Caps).
func (h *Server) listSessionOutputs(workspace string, sessionID int) []openCodeFileView {
	dir := openCodeOutputsDir(workspace, sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []openCodeFileView{}
	}
	type row struct {
		v   openCodeFileView
		mod time.Time
	}
	rows := []row{}
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		rows = append(rows, row{
			v: openCodeFileView{
				Name:        e.Name(),
				Size:        info.Size(),
				Mime:        openCodeMimeFor(e.Name()),
				CreatedAt:   info.ModTime().UTC().Format(time.RFC3339),
				DownloadURL: "/api/v1/integrations/opencode/sessions/" + strconv.Itoa(sessionID) + "/files/" + e.Name(),
			},
			mod: info.ModTime(),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].mod.After(rows[j].mod) })
	_ = total
	if len(rows) > openCodeMaxOutputFiles {
		rows = rows[:openCodeMaxOutputFiles]
	}
	out := make([]openCodeFileView, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.v)
	}
	return out
}

// sendOpenCodeFiles pusht die Output-Liste transient (WS-Event opencode_files).
func (h *Server) sendOpenCodeFiles(userID, backendSessionID int, files []openCodeFileView) {
	if h.Hub == nil {
		return
	}
	payload, err := json.Marshal(struct {
		Type      string             `json:"type"`
		SessionID int                `json:"session_id"`
		Files     []openCodeFileView `json:"files"`
	}{Type: "opencode_files", SessionID: backendSessionID, Files: files})
	if err != nil {
		return
	}
	h.Hub.SendToUser(userID, payload)
}

// openCodeCleanupSessionFiles löscht uploads/s{id}/ + outputs/s{id}/ best-effort.
func openCodeCleanupSessionFiles(workspace string, sessionID int) {
	_ = os.RemoveAll(openCodeUploadsDir(workspace, sessionID))
	_ = os.RemoveAll(openCodeOutputsDir(workspace, sessionID))
}

// uploadPromptPaths baut den Pfad-Anhang für den Agent-Prompt (Fallback:
// absolute Workspace-Pfade im Text — native File-Parts per Spike gegen
// GET /doc verifizieren, Format in v1.18 offen).
func uploadPromptPaths(workspace string, sessionID int, names []string) string {
	if len(names) == 0 {
		return ""
	}
	lines := []string{}
	for _, n := range names {
		if p, ok := openCodeResolveInDir(openCodeUploadsDir(workspace, sessionID), n); ok {
			if _, err := os.Stat(p); err == nil {
				lines = append(lines, "- "+p)
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\nHochgeladene Dateien (lies sie ausschließlich mit dem read-Tool, führe sie nie aus):\n" + strings.Join(lines, "\n")
}

// PostApiV1IntegrationsOpencodeSessionsIdUploads: multipart files[] →
// <workspace>/uploads/s{id}/<uuid>-<safe>. Guards: 10 MB/Datei, max 5,
// Allowlist-Ext, Traversal-Guard. Response: [{name,size,mime}].
func (h *Server) PostApiV1IntegrationsOpencodeSessionsIdUploads(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "ungültige ID")
		return
	}
	s, ok := h.openCodeSessionOwned(w, r, id)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, openCodeMaxUploadRequest)
	if err := r.ParseMultipartForm(openCodeMaxUploadRequest); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger Upload (max. 5 Dateien à 10 MB)")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "keine Dateien (Feld files[])")
		return
	}
	if len(files) > openCodeMaxUploadFiles {
		writeError(w, http.StatusBadRequest, "max. 5 Dateien pro Upload")
		return
	}
	dir := openCodeUploadsDir(s.Workspace, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "Speicher nicht verfügbar")
		return
	}
	out := []openCodeFileView{}
	for _, fh := range files {
		if fh.Size > openCodeMaxUploadBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "Datei zu groß (max. 10 MB): "+fh.Filename)
			return
		}
		safe := openCodeSafeName(fh.Filename)
		if safe == "" || !openCodeUploadExtOK(safe) {
			writeError(w, http.StatusBadRequest, "Dateityp nicht erlaubt (pdf, txt, md, png, jpg, jpeg, csv, docx, xlsx, pptx): "+fh.Filename)
			return
		}
		src, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "Datei konnte nicht gelesen werden")
			return
		}
		stored := openCodeNewSuffix() + "-" + safe
		dstPath, ok := openCodeResolveInDir(dir, stored)
		if !ok {
			_ = src.Close()
			writeError(w, http.StatusBadRequest, "ungültiger Dateiname")
			return
		}
		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			_ = src.Close()
			writeError(w, http.StatusInternalServerError, "Upload konnte nicht gespeichert werden")
			return
		}
		n, copyErr := io.Copy(dst, io.LimitReader(src, openCodeMaxUploadBytes+1))
		_ = src.Close()
		closeErr := dst.Close()
		if copyErr != nil || closeErr != nil || n > openCodeMaxUploadBytes {
			_ = os.Remove(dstPath)
			writeError(w, http.StatusRequestEntityTooLarge, "Datei zu groß (max. 10 MB): "+fh.Filename)
			return
		}
		out = append(out, openCodeFileView{
			Name:      stored,
			Size:      n,
			Mime:      openCodeMimeFor(safe),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusCreated, out)
}

// GetApiV1IntegrationsOpencodeSessionsIdFiles listet outputs/s{id}/.
func (h *Server) GetApiV1IntegrationsOpencodeSessionsIdFiles(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "ungültige ID")
		return
	}
	s, ok := h.openCodeSessionOwned(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.listSessionOutputs(s.Workspace, id))
}

// GetApiV1IntegrationsOpencodeSessionsIdFileDownload liefert eine Output-Datei.
func (h *Server) GetApiV1IntegrationsOpencodeSessionsIdFileDownload(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "ungültige ID")
		return
	}
	s, ok := h.openCodeSessionOwned(w, r, id)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	p, ok := openCodeResolveInDir(openCodeOutputsDir(s.Workspace, id), name)
	if !ok {
		writeError(w, http.StatusBadRequest, "ungültiger Dateiname")
		return
	}
	f, err := os.Open(p)
	if err != nil {
		writeError(w, http.StatusNotFound, "Datei nicht gefunden")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusNotFound, "Datei nicht gefunden")
		return
	}
	mime := openCodeMimeFor(info.Name())
	w.Header().Set("Content-Type", mime)
	disp := "attachment"
	if openCodeInlineMIME[mime] {
		disp = "inline"
	}
	w.Header().Set("Content-Disposition", disp+`; filename="`+info.Name()+`"`)
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
