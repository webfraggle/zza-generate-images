# Requirements Collection

Gesammelte Ideen und Anforderungen — noch nicht priorisiert oder umgesetzt.

## Änderungen / Refactoring

- **Komplettes Rewrite** — kein PHP mehr
- **Neue Sprache: Go**
  - Begründung: geringe Ressourcen, performant, single binary
  - Deployment: kleine VM via Docker Compose
  - Lokale Nutzung: native Binaries für Windows (.exe) und macOS
- **Template-System statt hardcodierter Layouts**
  - Layouts sollen nicht mehr in Go-Code stehen, sondern als externe Templates
  - Templates sollen von Usern genutzt, angepasst und erstellt werden können
  - Ziel: Community kann eigene Layouts beisteuern ohne Go-Kenntnisse

## Neue Features

### Online Template Editor
- Web-basiertes Editing-Tool für Templates
- **Kein Login-System** — Authentifizierung nur via E-Mail-Link

#### Flow: Neues Template anlegen
1. User gibt gewünschten Template-Namen ein
2. System prüft ob Name bereits existiert
3. Falls frei: User gibt E-Mail-Adresse ein
4. System speichert E-Mail zur Template-ID (nicht öffentlich auslesbar)
5. Zeitlich begrenzter Editier-Link wird an die Mail geschickt

#### Flow: Bestehendes Template editieren
1. User gibt Template-Namen ein
2. System schickt neuen zeitlich begrenzten Editier-Link an die hinterlegte Mail
3. User klickt Link → kann editieren

#### Sicherheit / Datenschutz
- E-Mail-Adresse ist intern gespeichert, aber für andere User nicht sichtbar
- Links sind zeitlich begrenzt (Dauer: noch offen)
- Kein Passwort, kein Account

#### Editor UI — Bestandteile
- **Dateiliste** — Assets des Templates (Bilder, Fonts) anzeigen / hochladen
- **YAML-Editor** — Editierfeld für die Template-Datei
- **Zug-JSON-Feld** — Testdaten eingeben für die Vorschau
- **Preview** — gerendertes PNG live anzeigen
- Technologie: **Vanilla JS** + **CodeMirror** (nur für YAML-Editor)
  - Kein Framework, kein Build-Prozess

## Template-System — Details

### Grundobjekte
- **Bild** — platzierbar, skalierbar (Hintergründe, Logos)
- **Formen** — z.B. Rechteck (Position, Größe, Farbe)
- **Font** — Definition von Schriftart und -größe
- **Text** — platzierbar, Farbe, Font-Referenz, Ausrichtung

### Logik / Steuerzeichen
- Templates brauchen ein rudimentäres **IF / ELIF / ELSE**-System
- Beispiel: Sternchen `*` am Anfang eines Hinweistexts → schwarzer Text auf gelbem Grund
- Steuerzeichen im Text sollen entweder:
  - **ausgewertet** werden (triggern Logik)
  - oder **ignoriert/entfernt** werden (vor der Darstellung strippen)
- Logik muss im Template definierbar sein, nicht im Code

### Format
- **YAML** — entschieden
  - Leserlicher als JSON (kein Anführungszeichen-Overhead, Kommentare möglich)
  - XML explizit ausgeschlossen (zu verbose)
- Variablen-Referenzierung: offen — vermutlich `{{zug1.hinweis}}` o.ä.

### Template-Verzeichnis
- Jedes Template lebt in einem eigenen Verzeichnis (wie bisher die PHP-Themes)
- Dort liegen: YAML-Templatedatei + zugehörige Assets (Bilder, Fonts)
- Erreichbar über:
  - **HTTP-Route** im Server-Modus (z.B. `/templates/sbb-096/`)
  - **CLI-Parameter** im lokalen Binary-Modus (z.B. `--template sbb-096`)

## Sicherheitsanforderungen

- **Kritisch** — der Editor erlaubt das Schreiben von Dateien auf dem Server → große Angriffsfläche
- Bei jeder Implementierung: Senior/Lead-Level Code Review mit Fokus auf Security
- Security-Prüfung durch dedizierten Agenten vor jeder Merge-Entscheidung
- Zu prüfende Bereiche (nicht abschließend):
  - Path Traversal (Dateipfade aus User-Input)
  - Arbitrary File Upload (Dateitype-Validierung)
  - YAML Injection / Deserialisierung
  - Token-Sicherheit (Editier-Links)
  - Rate Limiting (E-Mail-Versand, Token-Anfragen)
  - CORS-Konfiguration
  - Template-Rendering (Code-Injection über Variablen)

### Rollen & Rechte

#### Normaler User (E-Mail-authentifiziert)
- Darf **ausschließlich** auf das eine Template zugreifen, für das der Editier-Link ausgestellt wurde
- Token ist an Template-ID gebunden — serverseitig geprüft, nicht nur im Link
- Kein Zugriff auf andere Templates, keine Template-Liste, keine Admin-Funktionen

#### Superuser
- Zugriff auf alle Templates
- **Authentifizierung: 2 Schichten**
  1. **Admin-Token** — langer zufälliger String (64+ Zeichen), einmalig generiert, als Umgebungsvariable in `docker-compose.yml` / `.env` gesetzt
  2. **TOTP** (Time-based One-Time Password) — wie Google Authenticator/Authy. QR-Code wird beim ersten Start generiert. Jeder Login braucht Token + aktuellen TOTP-Code
  - Läuft komplett offline, kein externer Dienst nötig
  - Selbst bei kompromittiertem Token kein Zugriff ohne das physische Gerät

### Dateinamen-Regeln

Gilt für: Template-Namen, hochgeladene Asset-Dateien

| Regel | Detail |
|---|---|
| Erlaubte Zeichen | `a-z`, `0-9`, `-` (Bindestrich) |
| Max. Länge | 64 Zeichen |
| Automatische Bereinigung | Leerzeichen → `-`, Großbuchstaben → Kleinbuchstaben, Sonderzeichen → entfernt, zu lang → abschneiden |
| Mehrfache Bindestriche | werden zu einem zusammengefasst (`--` → `-`) |
| Dateiendungen | bleiben erhalten, werden aber ebenfalls bereinigt |

- Bereinigung passiert **automatisch und transparent** — der User sieht den bereinigten Namen
- Keine Fehlermeldung, kein Abbruch — einfach sanitizen und weitermachen

## Hinweise / Kontext

- Zielgruppe: Modellbahn-Enthusiasten mit physischen Zugzielanzeigern (kleine Displays)
- Die Zugzielanzeiger-Hardware schickt eine JSON mit Zuginformationen → PHP generiert daraus ein PNG
- Aktuell: jedes Layout in eigenem Verzeichnis mit eigener PHP-Datei (z.B. `sbb-096-v1/index.php`)
- Layouts bilden reale Anzeigesysteme nach: SBB, ÖBB, RhB, NS, U-Bahn München etc.
- Displaygrößen: 0.96", 1.05", 1.14"
