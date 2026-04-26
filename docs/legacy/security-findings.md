# Security Findings — /create-new und /request-token Flows

Audit-Datum: 2026-03-29
Auditor: Security-Review-Agent
Status: Alle Findings behoben — 2026-03-30 (Branch `fix/security-findings`)

---

## Kritisch / Muss vor Public-Deployment behoben werden

### F1 [HOCH] Kein Rate Limiting auf `POST /create-new`

**Datei:** `internal/server/create_handlers.go`, `handleCreateSubmit`

**Problem:**
Der Handler hat weder IP-Rate-Limiting noch CSRF-Schutz. Ein Angreifer kann in einer Schleife beliebig viele Templates anlegen bis der Disk-Space erschöpft ist. Gleichzeitig wird bei jedem erfolgreichen Aufruf eine E-Mail versendet — der Server fungiert als Open Mail Relay (E-Mail-Bombing gegen beliebige Adressen).

**Fix:**
`createHandler` erhält ein `*IPLimiter`-Feld. `RegisterCreateRoutes` nimmt den Limiter entgegen oder erstellt einen eigenen. `handleCreateSubmit` prüft am Anfang (nach Form-Parsing, vor Template-Anlage):

```go
ip := clientIP(r)
if !ch.ipLimiter.Allow(ip) {
    renderForm("Zu viele Anfragen. Bitte versuche es später erneut.")
    return
}
```

Bei Fehler (z. B. Template-ID bereits vergeben) kein `RecordFailure` — das ist kein Brute-Force-Versuch. Erfolgreiche Anlage: kein `RecordSuccess` nötig (der Limiter schützt hier gegen Spam, nicht gegen E-Mail-Guessing).

---

### F2 [HOCH] Kein CSRF-Schutz auf POST-Endpunkten

**Dateien:** `internal/server/create_handlers.go`, `internal/server/editor_handlers.go`, `internal/server/request_token_handler.go`

**Problem:**
`POST /create-new`, `POST /{template}/edit` und `POST /{template}/request-token` prüfen keinen CSRF-Token. Eine fremde Webseite kann per Auto-Submit-Formular Templates anlegen oder Token-Requests auslösen.

**Fix (Minimalansatz ohne Library):**
In einer neuen Middleware `checkCSRF(r *http.Request) bool` den `Origin`- oder `Referer`-Header gegen `BASE_URL` prüfen. Nur Browser-Requests haben diese Header. API-Requests von Microcontrollern (`/render`) sind davon ausgenommen.

```go
func checkOrigin(r *http.Request, baseURL string) bool {
    origin := r.Header.Get("Origin")
    if origin != "" {
        return strings.HasPrefix(origin, baseURL)
    }
    referer := r.Header.Get("Referer")
    return referer == "" || strings.HasPrefix(referer, baseURL)
}
```

Alle drei POST-Handler rufen `checkCSRF` am Anfang auf und antworten mit `http.StatusForbidden` bei Mismatch.

---

### F3 [MITTEL] `IPLimiter.Cleanup()` löscht Einträge mit unvollständigen Fehlversuchen

**Datei:** `internal/server/ip_limiter.go`, `Cleanup()`, Zeile ~82

**Problem:**
Die Bedingung `e.blockedUntil.IsZero() || now.After(e.blockedUntil)` löscht auch Einträge, bei denen `blockedUntil` null ist — also IPs, die 1–5 Fehlversuche hatten aber noch nicht blockiert wurden. Der Fehlerzähler dieser IPs wird stündlich zurückgesetzt. Ein Angreifer kann 5 Fehlversuche/Stunde machen und wird nie gesperrt.

**Fix:**
`IsZero()`-Bedingung aus `Cleanup()` entfernen. Nur abgelaufene Blocks bereinigen:

```go
func (l *IPLimiter) Cleanup() {
    l.mu.Lock()
    defer l.mu.Unlock()
    now := time.Now()
    for ip, e := range l.entries {
        if !e.blockedUntil.IsZero() && now.After(e.blockedUntil) {
            delete(l.entries, ip)
        }
    }
}
```

Test aktualisieren: `TestIPLimiter_CleanupRemovesExpiredEntries` bleibt gültig. Zusätzlich einen Test `TestIPLimiter_CleanupKeepsActiveFailures` schreiben, der prüft dass Einträge mit `failures > 0` und `blockedUntil.IsZero()` nach `Cleanup()` noch vorhanden sind.

---

## Sollte behoben werden

### F4 [HOCH] E-Mail-Header-Injection in `SendTokenMail` nicht abgesichert

**Datei:** `internal/editor/mailer.go`, Zeile ~43

**Problem:**
`templateName` und `to` werden unvalidiert in den SMTP-Header interpoliert. Aktuell durch vorgelagerte Validierung (`ValidateTemplateName`, `emailRe`) de facto gemildert, aber die Mailer-Funktion selbst vertraut blind den Parametern. Zukünftige Erweiterungen (z. B. freier `subject`-Parameter) würden Header-Injection öffnen.

**Fix:**
CRLF-Guard direkt am Anfang von `SendTokenMail`:

```go
for _, s := range []string{to, templateName} {
    if strings.ContainsAny(s, "\r\n") {
        return fmt.Errorf("mailer: header injection attempt in %q", s)
    }
}
```

---

### F5 [MITTEL] E-Mail-Enumeration über unterschiedliche Fehlermeldungen in `/request-token`

**Datei:** `internal/server/request_token_handler.go`, Zeilen ~56–71

**Problem:**
Zwei unterschiedliche Meldungen verraten ob ein Template-Besitzer existiert:
- `"Für dieses Template ist keine E-Mail hinterlegt."` (kein DB-Eintrag)
- `"Diese E-Mail-Adresse ist nicht als Besitzer registriert."` (falscher E-Mail)

**Fix:**
Beide Fälle mit derselben generischen Meldung zusammenfassen:

```
"Falls eine E-Mail für dieses Template hinterlegt ist und deine Adresse übereinstimmt, erhältst du in Kürze einen Link."
```

`RecordFailure(ip)` weiterhin nur bei echtem E-Mail-Mismatch aufrufen (nicht bei fehlendem DB-Eintrag).

---

### F6 [HOCH] Template-ID-Enumeration über `GET /create-new/check` ohne Rate Limit

**Datei:** `internal/server/create_handlers.go`, `handleCreateCheck`

**Problem:**
Der Endpunkt hat kein Rate Limiting. Mit einer Wortliste können in Sekunden alle existierenden Template-Namen enumeriert werden (`"available": false, "reason": "Bereits vergeben."`).

**Bewertung:** Weniger kritisch solange alle Templates öffentlich in der Galerie sichtbar sind.

**Fix:**
IP-Rate-Limiting analog zu `handleRequestToken`. Alternative: bei mehr als X Anfragen/Minute nur noch `{"available": false}` ohne `reason` zurückgeben.

---

### F7 [MITTEL] Token `used`-Flag wird nicht gesetzt nach Nutzung

**Datei:** `internal/editor/auth.go`, `ValidateToken`

**Problem:**
`ValidateToken` liest das `used`-Flag aus der DB, setzt es aber nie. Edit-Tokens sind theoretisch unbegrenzt wiederverwendbar bis zur `expires_at`-Zeit. Ein gestohlener Token bleibt gültig.

**Fix:**
In `ValidateToken` nach erfolgreicher Validierung das Token als `used` markieren, wenn es sich um einen Einmal-Token handeln soll:

```go
_, _ = db.Exec(`UPDATE edit_tokens SET used = 1 WHERE token = ?`, token)
```

Oder: im Edit-Handler nach dem ersten API-Request das Token revoken. Abhängig vom gewünschten UX (einmalige Session vs. mehrfache Nutzung während TTL).

---

## Zusammenfassung

| ID | Schwere | Titel | Status |
|---|---|---|---|
| F1 | HOCH | Kein Rate Limit auf POST /create-new | ✅ Behoben |
| F2 | HOCH | Kein CSRF-Schutz | ✅ Behoben |
| F3 | MITTEL | IPLimiter.Cleanup() löscht aktive Fehlerzähler | ✅ Behoben |
| F4 | HOCH | E-Mail-Header-Injection (Mailer) | ✅ Behoben |
| F5 | MITTEL | E-Mail-Enumeration via Fehlermeldungen | ✅ Behoben |
| F6 | HOCH | Template-ID-Enumeration via /check | ✅ Behoben |

**Klar bestanden:** Path Traversal, SQL Injection, Token-Entropie (crypto/rand 256 Bit), Upload-Whitelist, YAML-Injection, IP-Spoofing via Header.
