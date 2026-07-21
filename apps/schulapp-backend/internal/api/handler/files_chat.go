package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
)

func scanFolder(row interface{ Scan(...any) error }) (api.FileFolder, error) {
	var v api.FileFolder
	var parent, classID sql.NullInt64
	var created time.Time
	err := row.Scan(&v.Id, &v.Name, &parent, &classID, &created)
	if err == nil {
		if parent.Valid {
			x := int(parent.Int64)
			v.ParentId = &x
		}
		if classID.Valid {
			x := int(classID.Int64)
			v.ClassId = &x
		}
		v.CreatedAt = &created
	}
	return v, err
}
func scanFile(row interface{ Scan(...any) error }) (api.File, string, error) {
	var v api.File
	var path string
	var uploader, classID, folder sql.NullInt64
	var created time.Time
	err := row.Scan(&v.Id, &v.Name, &path, &v.Size, &v.MimeType, &uploader, &classID, &folder, &created)
	if err == nil {
		if uploader.Valid {
			x := int(uploader.Int64)
			v.UploaderId = &x
		}
		if classID.Valid {
			x := int(classID.Int64)
			v.ClassId = &x
		}
		if folder.Valid {
			x := int(folder.Int64)
			v.FolderId = &x
		}
		v.CreatedAt = &created
	}
	return v, path, err
}
func (h *Server) GetApiV1ClassesIdFolders(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,name,parent_id,class_id,created_at FROM file_folders WHERE class_id=$1 ORDER BY name`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.FileFolder{}
	for rows.Next() {
		v, err := scanFolder(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ClassesIdFolders(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassManage(w, r, id) {
		return
	}
	var req api.CreateFolderRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, 400, "Ordnername ist erforderlich")
		return
	}
	if req.ParentId != nil {
		var parentClass int
		err := h.DB.QueryRowContext(r.Context(), `SELECT class_id FROM file_folders WHERE id=$1`, *req.ParentId).Scan(&parentClass)
		if notFound(w, err, "Überordneter Ordner") {
			return
		}
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		if parentClass != id {
			writeError(w, 400, "Ordner gehört zu einer anderen Klasse")
			return
		}
	}
	v, err := scanFolder(h.DB.QueryRowContext(r.Context(), `INSERT INTO file_folders(name,parent_id,class_id) VALUES($1,$2,$3) RETURNING id,name,parent_id,class_id,created_at`, strings.TrimSpace(req.Name), nullableInt(req.ParentId), id))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Server) folderClass(r *http.Request, id int) (int, error) {
	var classID int
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id FROM file_folders WHERE id=$1`, id).Scan(&classID)
	return classID, err
}
func (h *Server) DeleteApiV1FoldersId(w http.ResponseWriter, r *http.Request, id int) {
	classID, err := h.folderClass(r, id)
	if notFound(w, err, "Ordner") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !h.requireClassManage(w, r, classID) {
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `DELETE FROM file_folders WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}
func (h *Server) GetApiV1ClassesIdFiles(w http.ResponseWriter, r *http.Request, id int, p api.GetApiV1ClassesIdFilesParams) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,name,path,size,mime_type,uploader_id,class_id,folder_id,created_at FROM files WHERE class_id=$1 AND (folder_id=$2 OR $2 IS NULL) ORDER BY created_at DESC`, id, nullableInt(p.FolderId))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.File{}
	for rows.Next() {
		v, _, err := scanFile(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ClassesIdFiles(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !h.canReadClass(r, id) {
		writeError(w, 403, "kein Zugriff auf diese Klasse")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, 400, "ungültiger Upload (maximal 20 MB)")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "Datei ist erforderlich")
		return
	}
	defer file.Close()
	folderID := 0
	if raw := r.FormValue("folder_id"); raw != "" {
		_, err = fmt.Sscanf(raw, "%d", &folderID)
		if err != nil || folderID < 1 {
			writeError(w, 400, "ungültige folder_id")
			return
		}
		classID, err := h.folderClass(r, folderID)
		if err != nil || classID != id {
			writeError(w, 400, "Ordner gehört nicht zu dieser Klasse")
			return
		}
	}
	if err = os.MkdirAll(h.UploadDir, 0750); err != nil {
		writeError(w, 500, "Speicher nicht verfügbar")
		return
	}
	safe := filepath.Base(header.Filename)
	if safe == "." || safe == "" {
		writeError(w, 400, "ungültiger Dateiname")
		return
	}
	tmp, err := os.CreateTemp(h.UploadDir, "upload-*")
	if err != nil {
		writeError(w, 500, "Upload konnte nicht gespeichert werden")
		return
	}
	size, copyErr := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp.Name())
		writeError(w, 500, "Upload konnte nicht gespeichert werden")
		return
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	var folder any
	if folderID > 0 {
		folder = folderID
	}
	v, stored, err := scanFile(h.DB.QueryRowContext(r.Context(), `INSERT INTO files(name,path,size,mime_type,uploader_id,class_id,folder_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,name,path,size,mime_type,uploader_id,class_id,folder_id,created_at`, safe, tmp.Name(), size, mime, c.UserID, id, folder))
	_ = stored
	if err != nil {
		os.Remove(tmp.Name())
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Server) fileByID(r *http.Request, id int) (api.File, string, error) {
	return scanFile(h.DB.QueryRowContext(r.Context(), `SELECT id,name,path,size,mime_type,uploader_id,class_id,folder_id,created_at FROM files WHERE id=$1`, id))
}
func (h *Server) GetApiV1FilesId(w http.ResponseWriter, r *http.Request, id int) {
	v, path, err := h.fileByID(r, id)
	if notFound(w, err, "Datei") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if v.ClassId == nil || !h.requireClassRead(w, r, *v.ClassId) {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, 404, "Datei nicht im Speicher")
		return
	}
	defer f.Close()
	createdAt := time.Time{}
	if v.CreatedAt != nil {
		createdAt = *v.CreatedAt
	}
	w.Header().Set("Content-Type", v.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", v.Name))
	http.ServeContent(w, r, v.Name, createdAt, f)
}
func (h *Server) DeleteApiV1FilesId(w http.ResponseWriter, r *http.Request, id int) {
	v, path, err := h.fileByID(r, id)
	if notFound(w, err, "Datei") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if v.ClassId == nil || !h.requireClassManage(w, r, *v.ClassId) {
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `DELETE FROM files WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		writeError(w, 500, "Datei konnte nicht gelöscht werden")
		return
	}
	w.WriteHeader(204)
}

func scanChannel(row interface{ Scan(...any) error }) (api.ChatChannel, error) {
	var v api.ChatChannel
	var name sql.NullString
	var classID sql.NullInt64
	var created time.Time
	var typ string
	err := row.Scan(&v.Id, &name, &typ, &classID, &created)
	if err == nil {
		if name.Valid {
			x := name.String
			v.Name = &x
		}
		if classID.Valid {
			x := int(classID.Int64)
			v.ClassId = &x
		}
		v.Type = api.ChatChannelType(typ)
		v.CreatedAt = &created
	}
	return v, err
}
func (h *Server) GetApiV1Channels(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT c.id,c.name,c.type,c.class_id,c.created_at FROM chat_channels c JOIN chat_members cm ON cm.channel_id=c.id WHERE cm.user_id=$1 ORDER BY c.created_at DESC`, c.UserID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.ChatChannel{}
	for rows.Next() {
		v, err := scanChannel(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (h *Server) GetApiV1ChatContacts(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	query := `SELECT u.id,u.email,u.first_name,u.last_name,u.role
		FROM users u
		WHERE u.active AND u.id <> $1`
	if c.Role != "admin" {
		query += ` AND EXISTS (
			SELECT 1
			FROM (
				SELECT class_id FROM class_members WHERE user_id=$1
				UNION SELECT class_id FROM class_teachers WHERE user_id=$1
			) own_classes
			JOIN (
				SELECT class_id FROM class_members WHERE user_id=u.id
				UNION SELECT class_id FROM class_teachers WHERE user_id=u.id
			) other_classes USING (class_id)
		)`
	}
	query += ` ORDER BY u.last_name,u.first_name`
	rows, err := h.DB.QueryContext(r.Context(), query, c.UserID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	contacts := []api.ChatContact{}
	for rows.Next() {
		var contact api.ChatContact
		var role string
		if err := rows.Scan(&contact.Id, &contact.Email, &contact.FirstName, &contact.LastName, &role); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		contact.Role = api.ChatContactRole(role)
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, contacts)
}

func (h *Server) canChatWith(r *http.Request, targetID int) bool {
	c := appmw.GetClaims(r)
	if c == nil || c.UserID == targetID {
		return false
	}
	var ok bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(
		SELECT 1 FROM users u
		WHERE u.id=$2 AND u.active AND (
			$3='admin' OR EXISTS (
				SELECT 1 FROM (
					SELECT class_id FROM class_members WHERE user_id=$1
					UNION SELECT class_id FROM class_teachers WHERE user_id=$1
				) own_classes
				JOIN (
					SELECT class_id FROM class_members WHERE user_id=u.id
					UNION SELECT class_id FROM class_teachers WHERE user_id=u.id
				) other_classes USING (class_id)
			)
		)
	)`, c.UserID, targetID, c.Role).Scan(&ok)
	return err == nil && ok
}

func (h *Server) PostApiV1Channels(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req api.CreateChannelRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || !req.Type.Valid() {
		writeError(w, 400, "ungültiger Channel")
		return
	}
	if req.Type == api.CreateChannelRequestTypeClass && (req.ClassId == nil || !h.canManageClass(r, *req.ClassId)) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	if req.Type == api.CreateChannelRequestTypeDirect && (req.MemberIds == nil || len(*req.MemberIds) != 1) {
		writeError(w, 400, "Direktchat benötigt genau ein Mitglied")
		return
	}
	if req.Type == api.CreateChannelRequestTypeGroup && (req.MemberIds == nil || len(*req.MemberIds) == 0 || strings.TrimSpace(ptrString(req.Name)) == "") {
		writeError(w, 400, "Gruppenchat benötigt einen Namen und mindestens ein Mitglied")
		return
	}
	memberIDs := []int{}
	seen := map[int]bool{}
	if req.MemberIds != nil {
		for _, userID := range *req.MemberIds {
			if seen[userID] || !h.canChatWith(r, userID) {
				writeError(w, 403, "eine ausgewählte Person ist nicht erreichbar")
				return
			}
			seen[userID] = true
			memberIDs = append(memberIDs, userID)
		}
	}
	if req.Type == api.CreateChannelRequestTypeDirect {
		v, err := scanChannel(h.DB.QueryRowContext(r.Context(), `SELECT c.id,c.name,c.type,c.class_id,c.created_at
			FROM chat_channels c
			WHERE c.type='direct'
			AND EXISTS(SELECT 1 FROM chat_members WHERE channel_id=c.id AND user_id=$1)
			AND EXISTS(SELECT 1 FROM chat_members WHERE channel_id=c.id AND user_id=$2)
			AND (SELECT COUNT(*) FROM chat_members WHERE channel_id=c.id)=2
			LIMIT 1`, c.UserID, memberIDs[0]))
		if err == nil {
			writeJSON(w, 201, v)
			return
		}
		if err != sql.ErrNoRows {
			writeError(w, 500, "Datenbankfehler")
			return
		}
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	v, err := scanChannel(tx.QueryRowContext(r.Context(), `INSERT INTO chat_channels(name,type,class_id) VALUES($1,$2,$3) RETURNING id,name,type,class_id,created_at`, nullableString(req.Name), req.Type, nullableInt(req.ClassId)))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	members := append([]int{c.UserID}, memberIDs...)
	if req.Type == api.CreateChannelRequestTypeClass {
		rows, err := tx.QueryContext(r.Context(), `SELECT user_id FROM class_members WHERE class_id=$1 UNION SELECT user_id FROM class_teachers WHERE class_id=$1`, *req.ClassId)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		for rows.Next() {
			var uid int
			rows.Scan(&uid)
			members = append(members, uid)
		}
		rows.Close()
	}
	for _, uid := range members {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO chat_members(channel_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, v.Id, uid); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func (h *Server) channelMember(r *http.Request, channelID int, userID int) bool {
	var ok bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM chat_members WHERE channel_id=$1 AND user_id=$2)`, channelID, userID).Scan(&ok)
	return err == nil && ok
}
func (h *Server) GetApiV1ChannelsIdMessages(w http.ResponseWriter, r *http.Request, id int, p api.GetApiV1ChannelsIdMessagesParams) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !h.channelMember(r, id, c.UserID) {
		writeError(w, 403, "kein Zugriff auf diesen Channel")
		return
	}
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	if limit < 1 || limit > 100 {
		writeError(w, 400, "limit muss zwischen 1 und 100 liegen")
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,channel_id,sender_id,content,file_id,created_at FROM messages WHERE channel_id=$1 AND (id<$2 OR $2 IS NULL) ORDER BY id DESC LIMIT $3`, id, nullableInt(p.Before), limit)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.Message{}
	for rows.Next() {
		var v api.Message
		var fileID sql.NullInt64
		if err = rows.Scan(&v.Id, &v.ChannelId, &v.SenderId, &v.Content, &fileID, &v.CreatedAt); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		if fileID.Valid {
			x := int(fileID.Int64)
			v.FileId = &x
		}
		out = append(out, v)
	}
	_, _ = h.DB.ExecContext(r.Context(), `UPDATE chat_members SET last_read_at=NOW() WHERE channel_id=$1 AND user_id=$2`, id, c.UserID)
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ChannelsIdMessages(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !h.channelMember(r, id, c.UserID) {
		writeError(w, 403, "kein Zugriff auf diesen Channel")
		return
	}
	var req api.SendMessageRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Content) == "" {
		writeError(w, 400, "Nachricht darf nicht leer sein")
		return
	}
	var v api.Message
	var fileID sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `INSERT INTO messages(channel_id,sender_id,content,file_id) VALUES($1,$2,$3,$4) RETURNING id,channel_id,sender_id,content,file_id,created_at`, id, c.UserID, strings.TrimSpace(req.Content), nullableInt(req.FileId)).Scan(&v.Id, &v.ChannelId, &v.SenderId, &v.Content, &fileID, &v.CreatedAt)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if fileID.Valid {
		x := int(fileID.Int64)
		v.FileId = &x
	}
	writeJSON(w, 201, v)
}
