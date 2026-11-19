# ZZA auf dem Gerät, Stufe 3 — Manueller Testplan

Spec: `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md`
Plan: `docs/superpowers/plans/2026-11-19-zza-on-device-stage3.md`
Firmware: Worktree `Zugzielanzeiger-tpl`, Branch `feature/zza-template-render`

Jeder Schritt: **Aktion** → **Erwartung**. Ergebnis mit ✅/❌ und Notiz eintragen.
Achtung: `uploadfs` überschreibt das komplette LittleFS (Züge, Konfiguration, Bilder). Vorher Komplett-Backup ziehen.

## 0. Automatische Prüfungen

| # | Aktion | Erwartung |
|---|---|---|
| 0.1 | Generator: `go vet ./... && go test ./...` (Mac: `CGO_CXXFLAGS` setzen) | grün |
| 0.2 | Generator: `tools/sync-zzarender.sh --check ../Zugzielanzeiger-tpl` | Exit 0, keine Abweichung |
| 0.3 | Firmware: `tools/test-native.sh` und `cd e2e && npm run unit` | grün |
| 0.4 | Firmware: `pio run` für QUAD/DUAL Debug/Release und `Zugzielanzeiger_ST7735_096_QUAD_Debug` | alle bauen |
| 0.5 | Firmware: `ZZA_CLI=<pfad-zu-zza> node run.ts --quad <ip> --yes` | grün, keine Reste auf dem Gerät |

## 1. Installieren (QUAD, Release + `uploadfs`)

| # | Aktion | Erwartung |
|---|---|---|
| 1.1 | `zza export -t faltblatt-105-v1` (der Export-Knopf im Editor kommt erst in Stufe 4) | ZIP entsteht |
| 1.2 | Web-UI → Menü → „Templates“ → ZIP wählen | Prüfung ok, Größe und freier Platz angezeigt |
| 1.3 | „Installieren“ | Fortschritt „Datei n von m“, danach in der Liste |
| 1.4 | Kaputtes ZIP (z. B. eine Datei im ZIP verändert) | Klartext-Fehler, nichts installiert |
| 1.5 | ZIP eines Templates mit Name > 32 Zeichen | Fehler „Name zu lang für den ZZA (max. 32 Zeichen)“ |

## 2. Anzeige

| # | Aktion | Erwartung |
|---|---|---|
| 2.1 | Gleis A: Zahnrad → Template `faltblatt-105-v1` → Speichern | beide Displays von A zeigen das Template; Gleis B unverändert |
| 2.2 | Zug wechseln (Pfeil) | Split-Flap läuft sichtbar flüssig und endet auf dem richtigen Bild |
| 2.3 | Vergleich: dieselben Zugdaten in der ZZA-Vorschau der Desktop-App | gleiches Bild |
| 2.4 | Gleis B: `umuc-105-v1` (Uhr) | Uhr läuft als Schleife mit zufälliger Uhrzeit; A animiert weiter bei Zugwechsel |
| 2.5 | Beide Gleise `faltblatt-105-v1`, gleichzeitig Zug wechseln | beide animieren (ggf. langsamer), kein Hänger, kein Neustart |
| 2.6 | Intervall-Modus 10 s auf A, 10 Minuten laufen lassen | stabil, `/templateStatus` `mem` bleibt gleich |
| 2.7 | `/showImage` (Bild als Zwischenbild) auf A | Bild steht; beim nächsten Zug wieder Template |
| 2.8 | Template auf A leeren | eingebautes Layout wie vorher |

## 3. Fehler und Rückfall

| # | Aktion | Erwartung |
|---|---|---|
| 3.1 | Benutztes Template löschen | Gleis zeigt eingebautes Layout, Web-UI meldet „nicht installiert“ |
| 3.2 | Dasselbe Template erneut installieren | Gleis rendert es wieder, ohne Neustart |
| 3.3 | Während „Installieren“ den Browser-Tab schließen | altes Template bleibt aktiv; erneutes Installieren klappt |
| 3.4 | Während „Installieren“ (Commit) den Strom ziehen (MAN-TPL-002) | nach dem Start altes oder neues Template vollständig, nie kaputt |
| 3.5 | Info-Display (BTN 0 kurz) bei einem Gleis mit Fehler | Zeile „Tpl A: … Fehler“ |

## 4. Backup

| # | Aktion | Erwartung |
|---|---|---|
| 4.1 | Komplett-Backup ziehen | ZIP enthält `tpl_<name>/…` |
| 4.2 | Restore „Alles ersetzen“ | Templates wie im Backup |
| 4.3 | Restore „Nur Züge …“ | installierte Templates bleiben unverändert |

## 5. DUAL (MAN-TPL-004)

| # | Aktion | Erwartung |
|---|---|---|
| 5.1 | DUAL-Release flashen, Abschnitte 1–2 kurz wiederholen (nur Gleis A) | wie QUAD; Split-Flap mit Vorrendern flüssiger als QUAD |
| 5.2 | Danach QUAD-Stand wieder flashen | Gerät wie vorher |
