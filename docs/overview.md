# System-Überblick

```
Browser → Reverse Proxy → Frontend (React) → Backend (Go) → Datenbank (PostgreSQL)
```

| Teil | Aufgabe |
|---|---|
| **Browser** | Führt die React-App aus, ruft das Backend per `fetch` auf |
| **Frontend** | Zeigt Seiten und Komponenten, liest Formulareingaben, lädt Daten |
| **Backend** | Validiert Eingaben, setzt Regeln durch, stellt API-Endpunkte bereit |
| **Datenbank** | Speichert alle Daten dauerhaft (Nutzer, Klassen, Nachrichten, Dateien …) |
| **Reverse Proxy** | Nimmt HTTP-Requests an und leitet sie an Frontend oder Backend weiter |

---

## Tech-Stack

| Schicht | Technologie |
|---|---|
| Frontend | React + TypeScript + Vite + Tailwind CSS |
| Routing | React Router v7 |
| State | Zustand + React Query |
| Backend | Go + chi (HTTP-Router) |
| API-Vertrag | OpenAPI 3.1 + oapi-codegen |
| Datenbank | PostgreSQL + sqlc (typsichere SQL-Queries) |
| Migrationen | golang-migrate |
| Auth | JWT (Access Token im Memory + Refresh Token als HttpOnly-Cookie) |
| Echtzeit | gorilla/websocket (Chat) |
| Datei-Upload | Go stdlib `multipart` |

---

## Rendering-Ansatz: CSR

Diese App nutzt **Client Side Rendering**:

1. Browser lädt `index.html`, CSS und JavaScript aus `dist/`
2. React startet im Browser
3. React ruft per `fetch` das Backend auf
4. Seite aktualisiert sich mit den geladenen Daten

Das Frontend ist ein **statisches Deployment-Artefakt**: `npm run build` erzeugt `dist/` mit HTML, CSS und JavaScript — kein Server-Prozess nötig, nur ein Webserver der die Dateien ausliefert.

Das Backend ist ein **laufender Server-Prozess** auf einem Port. Dieser muss gestartet werden und auf seine Konfiguration (Datenbankverbindung, Secrets) zugreifen können.

---

## Wie Frontend und Backend kommunizieren

Der Browser ruft das Backend direkt per `fetch` auf:

```ts
const response = await fetch(`${apiBase}/api/v1/projects`);
const projects = await response.json();
```

Worauf geachtet werden muss:
- Die API-URL muss im Frontend bekannt sein (Umgebungsvariable)
- Der Browser braucht Zugriff auf die API (CORS konfigurieren)
- Secrets gehören nicht ins Frontend — sie bleiben auf dem Server

### OpenAPI als Vertrag

Die `openapi.yaml` ist die **einzige Quelle der Wahrheit** für alle Endpunkte, Request-Bodies und Response-Typen.

```
openapi.yaml
    ↓ oapi-codegen
schulapp-backend/internal/api/generated.go   ← Go Handler-Interfaces + Typen
    ↓ openapi-typescript-codegen (oder orval)
codeclub-ui/src/api/generated.ts             ← TypeScript-Typen + fetch-Wrapper
```

**Workflow:**
1. Endpunkt in `openapi.yaml` beschreiben
2. `make generate` läuft beide Codegeneratoren
3. Go: generierten Interface implementieren
4. TypeScript: generierten Client im Hook nutzen

**Vorteile gegenüber händisch geschriebenen Clients:**
- Kein manuelles Synchronisieren von Typen zwischen Go und TypeScript
- Automatische API-Dokumentation (z. B. mit Swagger UI oder Scalar)
- Neue Teammitglieder sehen sofort alle Endpunkte

**Wann es aufwendiger wird:**
- Die Spezifikation muss bei jedem Endpunkt-Umbau aktualisiert werden
- Generierter Code sieht manchmal ungewohnt aus — nicht anfassen, neu generieren

---

## Statische Dateien vs. laufender Prozess

| Art | Beispiel | Läuft wo? | Deployment |
|---|---|---|---|
| Statische Dateien | Vite/React `dist/` | im Browser | Dateien kopieren und ausliefern |
| Backend/API | Go Binary | auf dem Server | `go build` → Binary kopieren und starten |
| Full-Stack-Framework | React Router v7, Next.js | Browser und Server | Server-Prozess + Assets |

---

## Ausblick: Was noch zu einem echten System gehört

| Thema | Kurz erklärt |
|---|---|
| **Reverse Proxy / Webserver** | Nimmt HTTP-Requests an und leitet sie an Frontend oder Backend weiter |
| **Bare Metal vs. Container** | Container isolieren Programme mit eigener Laufzeitumgebung (z. B. Docker) |
| **Horizontale / vertikale Skalierung** | Vertikal: ein Server wird stärker. Horizontal: mehrere Server teilen die Last |
| **Load Balancing** | Verteilt Requests auf mehrere laufende Instanzen |
| **File Storage / Buckets** | Für Uploads (Bilder, PDFs) statt Datenbankablage — z. B. S3-kompatibel |
| **Caching** | Häufig benötigte Daten zwischenspeichern, damit nicht jede Anfrage alles neu lädt |
| **Queues** | Aufgaben in Warteschlange legen und asynchron verarbeiten (z. B. E-Mails senden) |
| **Authentication / Authorization** | Auth prüft Identität, Authorization prüft Berechtigungen (z. B. Keycloak) |
| **Logging, Monitoring, Metrics** | Logging sammelt Fehler, Monitoring überwacht ob Systeme laufen, Metrics messen Laufzeiten |