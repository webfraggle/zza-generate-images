# Backlog

Ideen und Wünsche für spätere Features — noch nicht priorisiert, noch nicht geplant.

## Editor (Desktop)

- `YAML_FIELD_MAP` und `YAML_TO_DATA_KEY` aus `NODE_TYPES.fields` ableiten statt manuell pflegen (Wartungsfalle: Feld in NODE_TYPES ergänzen ≠ automatisch in YAML_FIELD_MAP). Erfordert `yamlKey`-Property in field-Defs oder Label-Konvention für `copy`-Labels (`src_w` vs. `src_width`).

## Renderer / Templates

- GIF-Render: Rate-Limit pro IP für `format=gif`; Cache-Größenlimit schon beim Schreiben prüfen (Aufräumen läuft nur alle 15 min); Request-Kontext bis in `RenderAnimation` durchreichen
- Split-Flap: `copy`-Layer vor einem späteren `splitflap` auf derselben Fläche erkennen und ablehnen (Blatt-Hintergrund wird sonst aus Frame 0 eingefroren)
- Split-Flap: fehlende Zellen bei `split: chars` auf die Leerklappe des Reels laufen lassen statt auf `""` zu springen
- GIF: bei 1-Frame-GIFs keinen Transparenz-Eintrag anhängen (Palette bleibt dann bei `canvas.colors`)
- Editor: Sequenzzähler für Vorschau-Antworten (späte GIF-Antwort überschreibt sonst „Standbild“); Animations-Umschalter ohne Reload nach dem ersten `splitflap`-Layer
- Templates direkt auf dem ZZA rendern (nur S3/1.05", Latin-1): Desktop-App exportiert ein vorkompiliertes Paket, die Firmware rendert selbst und animiert live — Spec `docs/superpowers/specs/2026-11-13-esp32-on-device-rendering-design.md` (zurückgestellt)
- Firmware: GIF/MJPEG-Wiedergabe auf dem Display (Abspielen der Faltblatt-GIFs bzw. MJPEG als Alternative)
- Stundenzeiger (24 h, ohne Minutenanteil) in `umuc-096-v1` und `nederland-096-v1` korrigieren (Formel wie in `umuc-105-v1`)
- Frame-Animation: statische Layer nur einmal rendern (Performance)
- Firmware: Loop-GIFs abspielen
- Weitere Faltblatt-Templates animieren (z.B. das alte 240×270-Faltblatt)
- `and()`/`or()` Funktionen im Evaluator — mehrere Bedingungen in einem `if:` verknüpfen, z.B. `if: "and(not(isEmpty(zug1.via)), isEmpty(zug1.hinweis))"`. Workaround heute: verschachtelte Block-Nodes.

## Galerie / UI

- Skalierung der Bilder sauber 1×/2× (Detail-Seite zeigt aktuell 1:1 ohne Zoom).
- Preview YAML in der Detail-Seite anzeigen.

## Sicherheit / Härtung

- YAML-Alias-Bombe-Schutz im Editor-Save (`gopkg.in/yaml.v3` ohne `SetMaxAliasCount`-Härtung). Body ist auf 1 MiB begrenzt; weitere Mitigation falls jemals Multi-Tenant.
- CORS im Desktop-Build einschränken statt Wildcard `*` auf `/render`.
- Host-Allowlist/Origin-Prüfung für den lokalen Editor (DNS-Rebinding/CSRF; betrifft auch `/edit/{t}/events`).
- `govulncheck` in den Build aufnehmen. Stand 2026-11-15 (`govulncheck ./...`): 29 erreichbare Befunde — Standardbibliothek go1.26.2 (Toolchain auf ≥ 1.26.11 heben), `golang.org/x/image` v0.18.0 → ≥ v0.39.0 (GO-2026-4962, Speicher bei bösartigen SFNT-Fonts; erreichbar über Renderer und `list_fonts`/`add_font`), `golang.org/x/net` v0.35.0 → ≥ v0.60.0.

## MCP-Server (Nachträge)

- Einheitliche Namensregeln über alle Tools (bereinigen per `SanitizeFilename` vs. ablehnen).
- Helfer für Fehlerkürzung, Endungsprüfung und Namens-Regex zusammenführen.
- Einheitliche Fehlertexte für Größen und Endungen.
- `ctx`-Abbruch auch für Render (`render_preview`, Probe-Render) und Foto-Werkzeuge.
- `create_template` räumt bei einem Fehler nach `InitTemplate` den halb angelegten Ordner nicht auf.
- `add_font` mit abweichender `id` für eine schon registrierte Datei gibt still die alte id zurück.
- Windows: stdio-Betrieb des GUI-Binaries und Chtimes→Write-Echo des Watchers praktisch prüfen.

## Animation / Uhr (Nachträge)

- Microcache (~1 s) oder Singleflight für BYPASS-Antworten zeitabhängiger Templates: wiederholte `GET /{template}/preview?format=gif` belegen sonst beide GIF-Slots (andere bekommen 503); PNG-Pfad hat kein Concurrency-Limit.
- Rotationsergebnis pro (Datei, Winkel) innerhalb eines Animations-Renders cachen (Stunden-/Minutenzeiger werden 60× identisch neu gedreht).

## Erledigt (Dual-Build, April 2026)

- ✅ ZIP-Download des kompletten Templates (`GET /{template}.zip`)
- ✅ UI in Exe mit WebView (Wails-v2-Bootstrap + Browser-Fallback)
- ✅ Server-Entschlackung: kein Auth, kein SMTP, kein SQLite, kein Admin
- ✅ Editor-Konsolidierung: `admin-editor.html` ist gelöscht, einziger Editor ist `edit-editor.html`

## Erledigt (Faltblatt-Animation, November 2026)

- ✅ Animierte GIFs: `type: splitflap`, `meta.animation`, `?format=gif` (Server) und `--format gif` (CLI), erstes Template `faltblatt-105-v1`
