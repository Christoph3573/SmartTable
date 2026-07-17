# Projektstruktur

## Frontend

```
codeclub-ui/src/
├── api/              Axios-Client + Endpunkte pro Feature
├── store/            Zustand Stores (Auth, Chat/Socket)
├── hooks/            React Query Hooks pro Feature
├── router/           Routen, ProtectedRoute, RoleRoute
├── layouts/          AuthLayout, AppLayout (mit Sidebar)
├── components/
│   ├── ui/           Button, Input, Modal, Badge, Avatar …
│   ├── layout/       Sidebar, Header, NotificationBell
│   └── shared/       FileUpload, DatePicker, RichTextEditor
└── pages/
    ├── auth/         LoginPage, ProfilePage
    ├── dashboard/    DashboardPage (rollenspezifisch)
    ├── substitutions/
    ├── calendar/
    ├── files/
    ├── chat/
    ├── homework/
    └── admin/        UsersPage, ClassesPage
```

## Backend

```
schulapp-backend/
├── openapi.yaml                    API-Spezifikation (Quelle der Wahrheit)
├── Makefile                        make generate, make migrate, make run
├── cmd/
│   └── server/
│       └── main.go                 Entry Point, Dependency Wiring
├── internal/
│   ├── api/
│   │   ├── generated.go            oapi-codegen Output — nicht manuell bearbeiten
│   │   └── handler/                Implementierungen der generierten Interfaces
│   │       ├── auth.go
│   │       ├── users.go
│   │       ├── classes.go
│   │       ├── substitutions.go
│   │       ├── events.go
│   │       ├── files.go
│   │       ├── homework.go
│   │       └── chat.go
│   ├── db/
│   │   ├── query/                  SQL-Queries für sqlc
│   │   └── generated/              sqlc Output — nicht manuell bearbeiten
│   ├── middleware/
│   │   ├── auth.go                 JWT verifizieren, User in Context setzen
│   │   └── role.go                 Rollen prüfen
│   └── ws/
│       └── hub.go                  WebSocket Hub (Chat-Echtzeit)
├── migrations/                     SQL-Migrationsdateien (golang-migrate)
├── uploads/                        Lokaler Datei-Speicher
└── seed/
    └── main.go                     Testdaten: 1 Admin, 3 Lehrer, 10 Schüler
```