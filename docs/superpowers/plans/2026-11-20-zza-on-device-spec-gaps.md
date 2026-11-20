# On-Device-Rendering: offene Punkte aus dem Spec-Abgleich (2026-11-20)

Abgleich von `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md` (inkl. §15) mit dem Code in diesem Repo (`feature/zza-on-device-rendering`) und im Firmware-Repo `Zugzielanzeiger-tpl` (`feature/zza-template-render`). Nur gelesen und gegrept, keine Tests ausgeführt. `tools/sync-zzarender.sh --check` lieferte Exit 0.

Erledigt am selben Tag:
- Editor-Knopf „Für ZZA exportieren“ (§6.3), Route `GET /edit/{template}/zza-export`.
- Template-Endpunkte aus der Firmware-`openapi.yaml` entfernt (gewollt: Swagger nur für `/api`; `docs/API.md` beschreibt sie weiter).

## Offen, nach Priorität

1. **Golden-Bilder (§9), Generator:** Keine eingecheckten RGB565-Goldens des C++-Kerns. `internal/devicerender/testdata/` hat nur `eval_corpus.json` und `regex_corpus.json`. Regressionen fallen nur über den Näherungsvergleich mit Go auf (`TestGeneratedNear`), nicht bitgenau. Entscheidung offen: welche Templates und Datenstände.
2. **Fehler je Gleis in der Firmware-Web-UI (§6.3), Firmware:** „beim letzten Start abgestürzt“, „Rechenaufwand zu hoch“ usw. nur auf dem Info-Display (`src/main.cpp:195`) und in `/templateStatus`. `data/app/js/app-templates.js:144-176` und `control2.js:767ff` werten `status.a/b.state` und `.error` nicht aus.
3. **Namenslänge (§15 P6), Generator:** Gerät erlaubt höchstens 32 Zeichen, Exporter lässt 64 zu (`internal/renderer/sanitize.go:18`) und warnt nicht. Abgelehnt wird erst in der Firmware-Web-UI. Grenze und `[a-z0-9-]` auch in `docs/yaml-template-spec.md:839-852` nennen.
4. **Kleinere Punkte:**
   - `zza export` ohne `-o` schreibt `<name>.zip` statt `<name>-zza.zip` (`internal/cli/export.go:26-31`).
   - Manifest `features` (§7.1) unvollständig: nur `splitflap`, `frames`, `regex`, `images` (`internal/devexport/export.go:58-70`); Layer-Typen, Filter, Variablen `now`/`anim` fehlen. Code oder Spec anpassen.
   - §7.3: kein Check, dass jeder Go-Filter im C++-Evaluator existiert. Heute identisch (`internal/renderer/evaluator.go` vs. `device/zzarender/eval.cpp:629-661`), neue Go-Filter würden aber nicht abgelehnt.
   - `CLAUDE.md`-Projektbaum: `internal/devexport` fehlt, `cli/` nur als „Render-CLI“ beschrieben.
   - Firmware-Sync-Test `e2e/lib/zzarenderSync.test.ts` prüft nur gegen `SOURCE.txt`: erkennt direkte Edits, aber keine veraltete Kopie gegenüber dem Generator.
5. **Stufe 4 / optional [P]:**
   - User-Guide-Kapitel „Template auf den ZZA übertragen“ (§10.4) fehlt; `docs/user-guide-templates.md:852-861` hat nur „ZZA-Vorschau“.
   - Galerie `GET /{template}-zza.zip` (§6.2) fehlt.
   - MCP `check_device_compat` bzw. Prüfung in `validate_template` fehlt (hängt an `feature/mcp-server`).

## Gleichwertig abweichend (kein Handlungsbedarf)

- Firmware-Web-UI nimmt Platz und Liste aus `/templateStatus` (`free`, `installed`) statt aus `/userFiles`.
- Alter Template-Ordner bleibt nach dem Commit als `.old` und wird in Häppchen aufgeräumt (`src/tpl/TplStore.cpp:109-122`, `src/WebServerTask.cpp:1683`), gedeckt durch §15 P4.
