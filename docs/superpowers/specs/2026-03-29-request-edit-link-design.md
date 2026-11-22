# Edit-Link via Modal anfordern — Design-Spec

**Datum:** 2026-03-29

## Ziel

Besitzer eines Templates können einen neuen Edit-Link anfordern, nachdem der ursprüngliche 24h-Link abgelaufen ist. Einstieg über einen Button auf der Detail-/Vorschau-Seite; E-Mail-Eingabe in einem Modal; Schutz vor Brute-Force über IP-basiertes Rate-Limiting.

## Architektur

- Neuer AJAX-Endpunkt `POST /{template}/request-token` gibt JSON zurück
- IP-Rate-Limiter als In-Memory-Struct in `editorState`
- Modal-Interaktion komplett client-seitig (kein Seitenreload)
- Bestehender `editor.RequestToken` wird wiederverwendet (E-Mail-Check, Token-Generierung, per-Template-Rate-Limit)

## Tech Stack

Go, `sync.Mutex`, Vanilla JS, keine neuen Abhängigkeiten

---

## 1. UI: Detail-Seite (`detail.html`)

### Button

In der `info-right`-Div, **oberhalb** des bestehenden „PNG herunterladen"-Buttons:

```html
<button id="owner-edit-btn" class="btn btn-level2">Edit durch Besitzer</button>
```

`btn-level2`: weißer Hintergrund, dunkelgraue Schrift (`#444`), gleiche Größe wie `btn-primary`.

### Modal

Inline im HTML (`display:none` → `display:flex` per JS). Struktur:

```
Overlay (halbtransparent, dunkel)
  └── Panel (weiß, zentriert)
        ├── Titel: „Edit-Link anfordern"
        ├── Beschreibung: „Gib deine E-Mail-Adresse ein. Wir schicken dir einen neuen Editier-Link."
        ├── E-Mail-Input
        ├── Fehlermeldung (verborgen bis nötig)
        ├── „Senden"-Button
        └── Schließen-Button (×, oben rechts)
```

**States:**
- Leer: Eingabe, Senden-Button aktiv
- Laden: Button disabled, Spinner-Text „Wird gesendet…"
- Fehler: Fehlermeldung eingeblendet, Formular bleibt offen, Button wieder aktiv
- Erfolg: Formular ausgeblendet, Bestätigungstext: „Wir haben dir einen Link an deine E-Mail geschickt."

**Schließen:** Klick auf Overlay oder × schließt Modal; Modal-Zustand wird zurückgesetzt.

---

## 2. Backend

### Neuer Endpunkt

`POST /{template}/request-token` — registriert in `RegisterEditorRoutes`.

**Request:** `application/x-www-form-urlencoded`, Feld `email`.

**Responses (immer HTTP 200):**

| Situation | JSON |
|---|---|
| Erfolg | `{"ok": true}` |
| Falsche E-Mail | `{"ok": false, "error": "Diese E-Mail-Adresse ist nicht als Besitzer registriert."}` |
| Kein Besitzer hinterlegt | `{"ok": false, "error": "Für dieses Template ist keine E-Mail hinterlegt."}` |
| IP gesperrt | `{"ok": false, "error": "Zu viele Fehlversuche. Bitte versuche es in 6 Stunden erneut."}` |
| Interner Fehler | `{"ok": false, "error": "Interner Fehler. Bitte versuche es später erneut."}` |

**Ablauf im Handler:**
1. IP ermitteln (erster Wert von `X-Forwarded-For`, Fallback `r.RemoteAddr`)
2. `IPLimiter.Allow(ip)` prüfen → bei false: IP-gesperrt-Response
3. E-Mail aus Form lesen, Format validieren
4. `editor.RequestToken(...)` aufrufen
5. Bei `ErrEmailMismatch` oder Template ohne DB-Eintrag (`ErrNoRows`): `IPLimiter.RecordFailure(ip)`
6. Bei Erfolg: `IPLimiter.RecordSuccess(ip)`, E-Mail senden (gleicher Dev-Fallback wie `handleEditSubmit`)

**Neue Datei:** `internal/server/request_token_handler.go`

### IP-Limiter

**Neue Datei:** `internal/server/ip_limiter.go`

```go
type IPLimiter struct {
    mu      sync.Mutex
    entries map[string]*ipEntry
}

type ipEntry struct {
    failures    int
    blockedUntil time.Time
}

const (
    maxIPFailures   = 6
    ipBlockDuration = 6 * time.Hour
)

func NewIPLimiter() *IPLimiter
func (l *IPLimiter) Allow(ip string) bool       // false wenn gesperrt und Sperre noch aktiv
func (l *IPLimiter) RecordFailure(ip string)    // Zähler +1; bei ≥ maxIPFailures → blockedUntil setzen
func (l *IPLimiter) RecordSuccess(ip string)    // Eintrag löschen (Zähler reset)
```

`Allow` gibt `true` zurück wenn Sperre abgelaufen ist (lazy expiry, kein Background-Goroutine nötig).

**Integration:** `editorState` bekommt Feld `ipLimiter *IPLimiter`; initialisiert in `RegisterEditorRoutes`.

### IP-Ermittlung

```go
func clientIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return strings.SplitN(xff, ",", 2)[0]  // erster Eintrag = echter Client
    }
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return host
}
```

---

## 3. DB-Schema

Keine Änderungen. IP-Daten leben ausschließlich im Arbeitsspeicher.

---

## 4. CSS

Neue Klasse in `app.css`:

```css
.btn-level2 {
  background: #ffffff;
  color: #444444;
  border: 1px solid var(--border);
}
.btn-level2:hover { border-color: var(--border-strong); }
```

Modal-CSS (Overlay + Panel) neu in `app.css`.

---

## 5. Tests

### `ip_limiter_test.go`
- 5 Fehlversuche → `Allow` gibt `true`
- 6. Fehlversuch → `Allow` gibt `false`
- Nach Ablauf von `blockedUntil` → `Allow` gibt wieder `true`
- `RecordSuccess` setzt Zähler zurück → `Allow` gibt `true`

### `request_token_handler_test.go`
- Korrekte E-Mail → `{"ok": true}`
- Falsche E-Mail → `{"ok": false, "error": "..."}` (kein interner Fehler)
- Template ohne DB-Eintrag → `{"ok": false, "error": "Für dieses Template..."}`
- 6× falsche E-Mail von selber IP → `{"ok": false, "error": "Zu viele Fehlversuche..."}` (7. Versuch, auch mit korrekter E-Mail)
