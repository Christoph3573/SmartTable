// Package handler — SchoolConnect-Integration.
//
// Das SmartTable-Backend proxied ausgewählte Aufrufe an die REST-API von
// SchoolConnect (v0.1.0, "schoolconnect serve", siehe
// https://github.com/Christoph3573/SchoolConnect/releases/tag/v0.1.0).
// Jede Plugin-Funktion ist dort automatisch ein Endpunkt:
//
//	GET|POST /api/<plugin>/<funktion>?param=...
//
// Wir spiegeln das unter (eigenes Auth via JWT bleibt davor):
//
//	GET  /api/v1/integrations/schoolconnect/status
//	POST /api/v1/integrations/schoolconnect/{plugin}/auth
//	POST /api/v1/integrations/schoolconnect/{plugin}/logout
//	GET|POST /api/v1/integrations/schoolconnect/{plugin}/{funktion}
//
// Der generische Call ist auf lesende Funktionen begrenzt (Allowlist);
// "fetch"-Funktionen (serverseitiger Dateidownload ins SchoolConnect-
// Dateisystem) und auth/logout (eigene Endpunkte) sind ausgenommen.
// Credentials gehen nur als POST-Body an SchoolConnect und werden hier
// weder geloggt noch gespeichert — die SchoolConnect-Runtime verwaltet
// ihre Sessions selbst (einmal auth, überall angemeldet).
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const schoolConnectDefaultBaseURL = "http://host.docker.internal:8081"

// Hinweis, wenn SchoolConnect nicht läuft. Eigener Port 8081, weil das
// SmartTable-Backend selbst schon auf :8080 hört. Auf dem Pi läuft
// SchoolConnect als systemd-Service (deploy/roles/schoolconnect),
// lokal per `REST_ADDR=:8081 schoolconnect serve` auf dem Host.
const schoolConnectHint = "SchoolConnect-REST starten: auf dem Pi via Ansible-Rolle " +
	"schoolconnect (deploy.yml), lokal via `REST_ADDR=:8081 schoolconnect serve` — Binary aus " +
	"https://github.com/Christoph3573/SchoolConnect/releases/tag/v0.1.0"

// scAuthPlugins sind die Plugins mit Login (Runtime generiert auth/logout).
var scAuthPlugins = map[string]bool{
	"schuelerportal": true,
	"mebis":          true,
	"bycs-drive":     true,
}

// scPluginNames sind Anzeigenamen für den Status-Endpunkt.
var scPluginNames = map[string]string{
	"schuelerportal":  "Schülerportal",
	"mebis":           "mebis (ByCS-Lernplattform)",
	"bycs-drive":      "ByCS Drive (Dateicloud)",
	"lernplan-bayern": "Lernplan Bayern (LehrplanPLUS)",
}

// scAllowedCalls begrenzt den generischen Call auf lesende Funktionen.
// fetch (Dateidownload ins SchoolConnect-Dateisystem) ist ausgenommen.
var scAllowedCalls = map[string]map[string]bool{
	"schuelerportal": {
		"profil": true, "stundenplan": true, "hausaufgaben": true, "vertretungsplan": true,
	},
	"mebis": {
		"courses": true, "abschnitte": true, "inhalt": true,
	},
	"bycs-drive": {
		"spaces": true, "list": true,
	},
	"lernplan-bayern": {
		"search": true, "details": true,
	},
}

func schoolConnectClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

// schoolConnectBase liefert die konfigurierte SchoolConnect-Adresse
// (ohne trailing slash). Env schlägt Server-Feld, Default ist 8081.
func (h *Server) schoolConnectBase() string {
	base := h.SchoolConnectBaseURL
	if env := strings.TrimSpace(os.Getenv("SCHOOLCONNECT_BASE_URL")); env != "" {
		base = env
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = schoolConnectDefaultBaseURL
	}
	return strings.TrimRight(base, "/")
}

// schoolConnectDo schickt einen Request an SchoolConnect und gibt
// Statuscode + Body (max. 8 MB) zurück.
func schoolConnectDo(client *http.Client, method, rawURL string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}

func schoolConnectUnreachable(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadGateway, map[string]string{
		"error": "SchoolConnect ist nicht erreichbar",
		"hint":  schoolConnectHint,
	})
}

// HandleSchoolConnectStatus meldet Erreichbarkeit + verfügbare Plugins.
// Immer HTTP 200 mit reachable-Flag, damit das Frontend zwischen
// "nicht konfiguriert" und "Fehler" unterscheiden und eine
// Setup-Anleitung zeigen kann.
func (h *Server) HandleSchoolConnectStatus(w http.ResponseWriter, r *http.Request) {
	if h.claims(w, r) == nil {
		return
	}
	base := h.schoolConnectBase()
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodGet, base+"/api", nil)
	if err != nil || status != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": true,
			"reachable":  false,
			"base_url":   base,
			"hint":       schoolConnectHint,
		})
		return
	}
	var items []struct {
		Plugin      string `json:"plugin"`
		Function    string `json:"function"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": true,
			"reachable":  false,
			"base_url":   base,
			"hint":       schoolConnectHint,
		})
		return
	}
	byPlugin := map[string][]string{}
	for _, item := range items {
		byPlugin[item.Plugin] = append(byPlugin[item.Plugin], item.Function)
	}
	plugins := []map[string]any{}
	for id, fns := range byPlugin {
		name := scPluginNames[id]
		if name == "" {
			name = id
		}
		plugins = append(plugins, map[string]any{
			"id":         id,
			"name":       name,
			"needs_auth": scAuthPlugins[id],
			"functions":  fns,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"reachable":  true,
		"base_url":   base,
		"plugins":    plugins,
	})
}

// HandleSchoolConnectAuth leitet einen Login an SchoolConnect weiter
// (POST /api/{plugin}/auth). Antwort enthält nie Secret-Werte,
// nur Key-Namen — das garantiert die SchoolConnect-Runtime.
func (h *Server) HandleSchoolConnectAuth(w http.ResponseWriter, r *http.Request) {
	if h.claims(w, r) == nil {
		return
	}
	plugin := chi.URLParam(r, "plugin")
	if !scAuthPlugins[plugin] {
		writeError(w, http.StatusBadRequest, "Plugin "+plugin+" braucht keine Anmeldung")
		return
	}
	var creds map[string]any
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodPost, h.schoolConnectBase()+"/api/"+plugin+"/auth", creds)
	if err != nil {
		schoolConnectUnreachable(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(raw)
}

// HandleSchoolConnectLogout verwirft die SchoolConnect-Session
// (POST /api/{plugin}/logout).
func (h *Server) HandleSchoolConnectLogout(w http.ResponseWriter, r *http.Request) {
	if h.claims(w, r) == nil {
		return
	}
	plugin := chi.URLParam(r, "plugin")
	if !scAuthPlugins[plugin] {
		writeError(w, http.StatusBadRequest, "Plugin "+plugin+" braucht keine Anmeldung")
		return
	}
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodPost, h.schoolConnectBase()+"/api/"+plugin+"/logout", map[string]any{})
	if err != nil {
		schoolConnectUnreachable(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(raw)
}

// HandleSchoolConnectCall proxied eine lesende Plugin-Funktion:
// GET-Query bzw. POST-JSON-Body werden 1:1 an SchoolConnect gereicht,
// Antwort (Result-Envelope {plugin, function, data}) kommt unverändert zurück.
func (h *Server) HandleSchoolConnectCall(w http.ResponseWriter, r *http.Request) {
	if h.claims(w, r) == nil {
		return
	}
	plugin := chi.URLParam(r, "plugin")
	function := chi.URLParam(r, "function")
	allowed, ok := scAllowedCalls[plugin]
	if !ok || !allowed[function] {
		writeError(w, http.StatusNotFound, "Funktion "+plugin+"/"+function+" ist nicht freigegeben")
		return
	}

	target := h.schoolConnectBase() + "/api/" + url.PathEscape(plugin) + "/" + url.PathEscape(function)
	var status int
	var raw []byte
	var err error
	if r.Method == http.MethodPost {
		var body map[string]any
		if r.Header.Get("Content-Type") != "" && !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, http.StatusBadRequest, "nur application/json wird unterstützt")
			return
		}
		if r.ContentLength != 0 {
			if derr := json.NewDecoder(r.Body).Decode(&body); derr != nil && derr != io.EOF {
				writeError(w, http.StatusBadRequest, "ungültiges JSON")
				return
			}
		}
		status, raw, err = schoolConnectDo(schoolConnectClient(), http.MethodPost, target, body)
	} else {
		if query := r.URL.RawQuery; query != "" {
			target += "?" + query
		}
		status, raw, err = schoolConnectDo(schoolConnectClient(), http.MethodGet, target, nil)
	}
	if err != nil {
		schoolConnectUnreachable(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(raw)
}
