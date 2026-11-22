# Design: Neues Template anlegen (`/create-new`)

**Datum:** 2026-03-29
**Status:** Approved

## Überblick

User-Flow zum Anlegen eines neuen Templates über eine dedizierte Seite `/create-new`. Der Benutzer füllt ein Formular aus, das Template-Verzeichnis wird sofort angelegt, und der bestehende E-Mail-Token-Flow übernimmt den Zugang zum Editor.

---

## Routen

| Methode | Pfad | Beschreibung |
|---|---|---|
| `GET` | `/create-new` | Formular anzeigen |
| `POST` | `/create-new` | Formular verarbeiten |
| `GET` | `/create-new/check` | Async-ID-Check: `?id=foo` → JSON |

Alle Routen werden auf dem Haupt-Mux registriert (kein Pre-Mux-Dispatch nötig).

---

## Formularfelder

| Feld | Typ | Validierung |
|---|---|---|
| E-Mail | `<input type="email">` | `emailRe` Regex (wie bestehender Edit-Flow) |
| Display-Größe | Radio | `1.05"` (240×240) oder `0.96"` (160×160); Default: 1.05" |
| Template-ID | `<input type="text">` | `^[a-z0-9-]+$`, max 50 Zeichen; Async-Uniqueness-Check |
| Titel | `<input type="text">` | Nicht leer, max 80 Zeichen |
| Beschreibung | `<textarea>` | Optional, max 300 Zeichen |

---

## POST-Flow (serverseitig)

1. Alle Felder validieren (E-Mail, Template-ID Format, Titel nicht leer)
2. Template-ID Uniqueness prüfen: Verzeichnis darf nicht existieren UND kein DB-Eintrag vorhanden
3. `editor.CreateTemplate()` aufrufen → Verzeichnis + angepasste `template.yaml` + Starter `default.json`
4. `editor.RequestToken()` aufrufen → DB-Eintrag + Token
5. E-Mail senden (wie `handleEditSubmit`, mit Dev-Fallback auf Log)
6. `create-sent.html` rendern

---

## Async-ID-Check (`GET /create-new/check?id=foo`)

**Response:**
```json
{"available": true}
{"available": false, "reason": "bereits vergeben"}
{"available": false, "reason": "ungültiges Format"}
```

- Prüft Format gegen `^[a-z0-9-]+$` und max 50 Zeichen
- Prüft ob Verzeichnis `{templatesDir}/{id}` existiert
- Prüft ob DB-Eintrag für `id` existiert
- Submit-Button im Frontend deaktiviert während Check läuft oder Fehler vorliegt
- Debounce im Frontend (ca. 300ms)

---

## Neue Go-Funktion: `editor.CreateTemplate`

```go
func CreateTemplate(templatesDir, templateName string, meta renderer.Meta) error
```

- Legt Verzeichnis an via `os.Mkdir` (nicht `MkdirAll`) — schlägt mit Fehler fehl wenn bereits vorhanden (Race-Condition-Schutz)
- Schreibt angepasste `template.yaml` mit name, description, display, canvas (width/height per Display-Größe)
- Schreibt Starter `default.json` (aus `//go:embed`)
- Schlägt fehl wenn Verzeichnis bereits existiert (Race-Condition-Schutz)

**Generiertes `template.yaml` (Beispiel):**
```yaml
meta:
  name: "Mein Titel"
  description: "Meine Beschreibung"
  author: ""
  version: "1.0"
  display: "1.05\""
  canvas:
    width: 240
    height: 240

layers:
  # Starter-Layers (aus bestehendem starter/template.yaml)
  ...
```

---

## Go-Implementierungsstruktur

**Neues File:** `internal/server/create_handlers.go`

```go
type createHandler struct {
    db   *sql.DB
    cfg  EditorConfig   // wiederverwendet (TokenTTL, Mail)
    tmpl *template.Template
    tdir string
}
```

Registrierung in `RegisterEditorRoutes` (oder separater `RegisterCreateRoutes`-Methode auf `Server`).

---

## HTML-Templates

**`web/templates/create-new.html`**
- Layout: `form-card` (wie `edit-request.html`)
- Erklärungstext oben: "Hier kannst du ein neues Template anlegen. Gib deine E-Mail-Adresse an — du bekommst einen Link zum Editor. Die Template-ID ist dein eindeutiger Name in der URL (z.B. `/mein-template`)."
- Inline-Feedback für Template-ID: grüner Haken / rotes Kreuz + Text
- Submit-Button disabled-State per JS

**`web/templates/create-sent.html`**
- Erfolgsseite: "Wir haben dir einen Link an `{email}` geschickt. Klick ihn um deinen Template-Editor zu öffnen. Der Link ist 24 Stunden gültig."

---

## Gallery-Header (`gallery.html`)

Bestehenden Header um einen Link ergänzen:
```html
<a href="/create-new" class="btn btn-primary">+ Neues Template</a>
```

---

## Abgrenzung

- **Nicht geändert:** Bestehender `/{template}/edit`-Flow für bestehende Template-Besitzer bleibt unberührt.
- **`InitTemplate` bleibt** als Fallback für bereits bestehende Templates ohne Verzeichnis (wird in `handleEditor` aufgerufen).
- **Keine neuen Packages:** Logik in `internal/editor/files.go` und `internal/server/create_handlers.go`.
