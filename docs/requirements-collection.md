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

## Cache-Management

- Dateibasierter Cache (SHA1-Hash des JSON-Inputs als Dateiname)
- **Automatischer Cleanup** — verhindert volllaufenden Speicher (bekanntes Problem der PHP-Version)
- Cleanup-Strategien (zu entscheiden):
  - Nach Alter: Dateien älter als X Stunden/Tage löschen
  - Nach Größe: Wenn Cache-Verzeichnis > X MB, älteste Dateien löschen
  - Beides kombiniert
- Cleanup läuft als interner Go-Goroutine (kein externer Cronjob nötig)
- Konfigurierbar via Umgebungsvariable (max. Alter, max. Größe)
- Strategie: **beides kombiniert** — nach Alter UND nach Gesamtgröße

## Mitgelieferte Templates

- Alle 14 bestehenden Themes werden als YAML-Templates übernommen
- Basis: PHP-Logik und Assets aus `legacy/` als Referenz
- Werden von Claude aus der PHP-Implementierung in das neue YAML-Format portiert

## Neue Features

### Template-Galerie
- Öffentliche Übersicht aller verfügbaren Templates
- Zeigt: Template-Name, Vorschaubild, Beschreibung (aus `meta` im YAML)
- Kein Login nötig zum Durchsuchen
- Von dort direkt zur Render-URL oder zum Editor-Anfrageformular

### Ausprobiermodus (pro Template)
- Formular mit Zugwerten direkt in der Galerie/Detailseite
- Vorschau des gerenderten PNGs in Echtzeit
- Formular wird mit Werten aus `default.json` des Templates vorbelegt
- `default.json` liegt flach im Template-Verzeichnis (wie die anderen Dateien)
- `default.json` ist vom Template-Besitzer über den Editor editierbar
- Kein Login nötig zum Ausprobieren

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
- **Block-Level if/elif/else** ✅ — mehrere Layer unter einer gemeinsamen Bedingung gruppieren (ein Block-Eintrag ohne `type:`, mit `layers:`); `else:` ohne Wert ist gleichwertig mit `else: true`; beliebig tief verschachtelbar

### Format
- **YAML** — entschieden
  - Leserlicher als JSON (kein Anführungszeichen-Overhead, Kommentare möglich)
  - XML explizit ausgeschlossen (zu verbose)
- Variablen-Referenzierung: `{{zug1.hinweis}}` — entschieden, siehe `docs/yaml-template-spec.md`

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

## CLI-Modus

- Lokale Nutzung als Binary (Windows + macOS)
- Syntax: `zza render --template sbb-096-v1 --input zug.json --output bild.png`
- Details werden später verfeinert

## Technische Entscheidungen

| Thema | Entscheidung |
|---|---|
| Sprache | Go |
| Deployment | Docker Compose, kleine VM |
| Lokale Binaries | Windows (.exe) + macOS |
| Template-Format | YAML |
| Datenbank | SQLite |
| E-Mail | Eigener SMTP-Server — Konfiguration via Umgebungsvariablen in docker-compose.yml |
| API-Struktur | Template-Name als erstes URL-Segment: `POST /{template}/render`, `GET /{template}/edit` |
| Caching | Dateibasiert (SHA1-Hash wie bisher), mit automatischem Cleanup-Mechanismus |

## Hinweise / Kontext

- Zielgruppe: Modellbahn-Enthusiasten mit physischen Zugzielanzeigern (kleine Displays)
- Die Zugzielanzeiger-Hardware schickt eine JSON mit Zuginformationen → PHP generiert daraus ein PNG
- Aktuell: jedes Layout in eigenem Verzeichnis mit eigener PHP-Datei (z.B. `sbb-096-v1/index.php`)
- Layouts bilden reale Anzeigesysteme nach: SBB, ÖBB, RhB, NS, U-Bahn München etc.
- Displaygrößen: 0.96", 1.05", 1.14"
