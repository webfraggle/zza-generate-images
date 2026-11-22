# Manuelle Tests: Detailseite mit Formular

Server lokal: `go run ./cmd/zza-server serve` (oder `zza serve`), dann im Browser.

| # | Schritt | Erwartung |
|---|---|---|
| 1 | `/faltblatt-105-v1` öffnen | Formular zeigt Bahnsteig (Gleis) und Zug 1 (Abfahrt, Zug, Über, Ziel). Kein JSON sichtbar. Vorschau lädt. |
| 2 | Ziel ändern | Vorschau aktualisiert sich nach kurzer Pause ohne Klick. |
| 3 | „Abspielen“ | Split-Flap-Animation läuft in der Vorschau. Danach Feld ändern → Standbild. |
| 4 | „PNG herunterladen“ | Datei `faltblatt-105-v1.png` mit aktuellem Inhalt. |
| 5 | „GIF herunterladen“ | Datei `faltblatt-105-v1.gif`, animiert. |
| 6 | `/umuc-105-v1` | Gruppen Zug 1–3 offen (Defaults gefüllt). |
| 7 | `/sbb-105-v1` | Zug 1 mit Verspätung und Hinweis; Verspätung „abc“ → kein Fehler-Crash. |
| 8 | `/rhb-105-v1` | Nicht animiert: nur „PNG herunterladen“, kein Abspielen/GIF. |
| 9 | „Für Entwickler“ aufklappen | Render-URL kopierbar, aktuelles JSON entspricht Formular, ZIP-Link. |
| 10 | Fenster < 860 px | Eine Spalte, Vorschau oben, keine horizontale Scrollbar. |
| 11 | Tastatur | Alle Felder/Buttons per Tab erreichbar, Fokus sichtbar. |
| 12 | Desktop-Build (`zza serve`) | „Bearbeiten“-Link im Entwickler-Bereich vorhanden. |
