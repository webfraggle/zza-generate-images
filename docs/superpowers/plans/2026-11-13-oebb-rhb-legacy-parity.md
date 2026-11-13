# ÖBB/RhB Legacy-Parität + animierte Galerie — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ÖBB- und RhB-Template sehen aus wie die PHP-Themes; ÖBB-Uhr läuft mit Sekunden; Galerie zeigt animierte GIFs mit Neustart-Button.

**Architecture:** Kleine, allgemeine Renderer-Erweiterungen (Filter, Text-`id`/`advance`, `scale_y`), darauf zwei reine YAML-Templates. Galerie nutzt den bestehenden `?format=gif`-Preview-Endpoint.

**Tech Stack:** Go (stdlib, `golang.org/x/image`), html/template, Vanilla JS/CSS.

**Spec:** `docs/superpowers/specs/2026-11-13-oebb-rhb-legacy-parity-design.md`

## Global Constraints

- Server-Build bleibt CGO-frei; keine neuen Abhängigkeiten.
- Bestehende Templates (`faltblatt`, `nederland`, `sbb`, `umuc`) rendern unverändert.
- Der Arbeitsbaum enthält nicht committete Änderungen des Nutzers (gelöschte `*-096-v1`-Templates, `templates/nederland-105-v1/template.yaml`, `templates-backup/`). Diese nie committen, zurücksetzen oder verändern; immer nur die eigenen Dateien gezielt `git add`en.
- Implementierer committen nicht selbst; der Orchestrator committet nach jeder Task.
- Doku-Sprache Deutsch, Code-Kommentare Englisch (wie im Bestand).

## Review Focus

- Leere/fehlende JSON-Felder (`zeit`, `abw`, `nr`, `via`, `hinweis` leer oder nicht vorhanden): Template rendert ohne Fehler, nichts verrutscht.
- `match` mit bösartigem/überlangem Muster aus einem Template: kein Panic, `""`.
- Sehr langer Text mit `id`: `width`/`end` bleiben endlich, kein Überlauf in Folge-Koordinaten.
- `format('HH:mm:ss', x)` mit nicht-numerischem `x`: verhält sich wie ohne Versatz.
- Galerie: GIF-Request scheitert → Karte zeigt weiterhin ein Bild (PNG-Fallback), Klick auf ↻ öffnet nicht die Detailseite.

---

### Task 1: Filter `clock`, `replace`, `match`, `format`-Versatz

**Files:** Modify `internal/renderer/evaluator.go`, `internal/renderer/usesnow.go` (nur falls nötig); Test `internal/renderer/evaluator_test.go`; Doku `docs/yaml-template-spec.md` (Filter-Tabelle + Abschnitt Datum/Zeit), `docs/user-guide-templates.md` (falls Filter dort gelistet).

**Produces:** Filter exakt wie in der Spec-Tabelle „Filter".

- [ ] Vorab: `go test ./internal/renderer/` — `TestUsesNow_RealTemplates` schlägt auf dem Ausgangsstand fehl (vermutlich wegen der entfernten 0.96"-Templates). Test so anpassen, dass er nur vorhandene Templates voraussetzt.
- [ ] Failing Tests (table-driven, über `NewEvaluator`/`newEvaluatorAt` + Interpolation):
  - `{{m | clock}}`: `963`→`16:03`, `0`→`00:00`, `1445`→`00:05`, `-5`→`23:55`, `abc`→``, ``→``
  - `{{zug1.zeit | minutes | add(zug1.abw) | clock}}` mit `15:53`/`10` → `16:03`
  - `{{v | replace(' - ', '~')}}`: `A - B - C`→`A~B~C`; `replace('', 'x')` unverändert
  - `{{nr | match('([a-zA-Z]+)([0-9]+)', 1)}}`: `S1`→`S`, Gruppe 2→`1`, Gruppe 0→`S1`; `REX 1234`→``; Gruppe 5→``; Muster `(`→``; Muster mit 101 Zeichen→``
  - `match` als Bedingungs-Argument: `isEmpty(nr | match('([a-zA-Z]+)([0-9]+)', 2))`
  - `{{now | format('HH:mm:ss', 5)}}` bei now=12:00:58 → `12:01:03`; `format('HH:mm:ss', anim.time)` in Frame 3 mit delay 1 s → +3 s; `format('HH:mm', 'x')` → wie ohne Versatz
- [ ] Implementieren; Regex-Cache mit `sync` geschützt, max. 256 Einträge.
- [ ] `go test ./internal/renderer/` grün, `go vet ./...` sauber. Doku ergänzen.

### Task 2: Text-Layer `id`/`advance` und `scale_y`

**Files:** Modify `internal/renderer/template.go` (Felder `ID string yaml:"id"`, `Advance int yaml:"advance"`, `ScaleY float64 yaml:"scale_y"` + Validierung beim Laden), `internal/renderer/renderer.go` (`renderText`, `drawTextLine`); Test `internal/renderer/text_ext_test.go`; Doku `docs/yaml-template-spec.md` (Abschnitt `type: text`, Koordinaten-Ausdrücke).

**Produces:** Koordinaten-Variablen `<id>.width`, `<id>.end` und `scale_y` exakt wie in der Spec.

- [ ] Failing Tests:
  - Text `id: a, x: 10, advance: 3`, Folge-Rect mit `x: "{{a.end}}"`: Rect beginnt bei `10 + Textbreite + 3` (Textbreite über `measureText` mit derselben Face ermitteln); `{{a.width}}` = Textbreite.
  - Leerer Wert: `a.end` = 10, `a.width` = 0.
  - `scale_x: 0.5`: `a.width` = skalierte Breite.
  - Top-Level-`id` ist in einem Loop-Sublayer nutzbar.
  - Ladefehler: doppelte `id`; `id: "9x"`; `id` mit `align: right`; `id` mit `fit_items`; `id` auf `type: rect`; `advance: -1`, `advance: 241`.
  - `scale_y: 2`: Bounding-Box der gezeichneten Pixel ist ~doppelt so hoch wie mit 1 (±1 px), Oberkante bleibt (±1 px), Breite unverändert. `scale_y: 9`, `-1`, `.nan` → Ladefehler. Ohne `scale_y`: Ausgabe byte-identisch zum bisherigen Verhalten (Vergleich zweier Renders mit `scale_y` weggelassen vs. `1`).
- [ ] Implementieren. Variablen müssen pro Frame/Render neu entstehen (kein Zustand am `Template`).
- [ ] `go test ./...` grün. Doku ergänzen (inkl. Portierungshinweis: Legacy-`yFactor` = `scale_y`).

### Task 3: Galerie mit GIF + Neustart-Button

**Files:** Modify `internal/gallery/gallery.go` (`Animated bool` in `TemplateInfo`), `web/templates/gallery.html`, `web/static/app.css`; Test `internal/server/server_test.go` (+ ggf. Gallery-Test).

- [ ] Failing Tests: Galerie-HTML enthält für ein animiertes Template (`umuc-105-v1`) `src="/umuc-105-v1/preview?format=gif"` und einen Button mit `aria-label="Animation neu starten"`; für ein nicht animiertes (`sbb-105-v1`) `src="/sbb-105-v1/preview"` und keinen Button. Server-Tests, die noch `sbb-096-v1` voraussetzen, auf `sbb-105-v1` umstellen.
- [ ] Implementieren laut Spec-Abschnitt „Galerie": Karte = Wrapper (`position: relative`) mit Link und Button als Geschwistern; Inline-Script: Klick → `img.src = base + "&r=" + (++n)`; `img.onerror` → einmalig PNG-Fallback. Bestehendes Karten-Layout/Hover bleibt optisch gleich.
- [ ] `go test ./...` grün; im Browser prüfen (`go run ./cmd/zza-server serve`, Port per `PORT`).

### Task 4: Template ÖBB

**Files:** Modify `templates/oebb-105-v1/template.yaml`; Test `internal/renderer/` (neuer `oebb_test.go` nach Muster `umuc_test.go`).

**Consumes:** Task 1 + 2.

- [ ] Referenzbilder erzeugen: `cd legacy/oebb-105-v1 && php index.php >/dev/null 2>&1` (neueste Datei in `cache/`); für weitere Fälle `default.json` in eine Temp-Kopie des Legacy-Ordners im Scratchpad legen und dort variieren (Legacy-Ordner selbst nicht ändern). Fälle: Standard (S1, abw 10, `*`-Hinweis), `abw` 0, Hinweis ohne `*`, kein Hinweis, `nr` = `REX1234`, `nr` = `RJ`, leere Zeit.
- [ ] Template nach Spec-Abschnitt „Template ÖBB" schreiben; je Fall Go-Render (`go run ./cmd/zza render …`) neben PHP-Render legen (ImageMagick `compare`/`+append`), Positionen justieren bis ≤ 1–2 px Abweichung je Element. Die Uhr ist zeitabhängig und wird nur auf Lage/Größe verglichen.
- [ ] Tests: Template lädt, ist animiert (60 Frames), rendert alle obigen Fälle fehlerfrei; bei `abw: 0` gibt es keine Pixel in `#fbef52` oberhalb y 89; mit `*`-Hinweis ist Pixel (120, 100) gelb-/schwarzanteilig Balken (Pixel (1, 90) = `#fbef52`), ohne `*` nicht.
- [ ] `go test ./...` grün.

### Task 5: Template RhB

**Files:** Modify `templates/rhb-105-v1/template.yaml`; Test `internal/renderer/rhb_test.go`.

**Consumes:** Task 2.

- [ ] Referenzbilder wie in Task 4 (`legacy/rhb-105-v1`). Fälle: Standard (RE, zwei Vias), `nr` = `IR` und `R`, keine Vias, fünf Vias, leere `nr`.
- [ ] Template nach Spec-Abschnitt „Template RhB" schreiben und gegen PHP justieren.
- [ ] Tests: lädt, rendert alle Fälle fehlerfrei; Via-Text beginnt weiter rechts, wenn `nr` breiter ist (erste gelbe Pixelspalte in y 6–16 rechts des Rechteck-Anfangs für `RE` < für `REX`).
- [ ] `go test ./...` grün.

### Task 6: Abschluss

- [ ] `security-reviewer` und `code-reviewer` auf `git diff develop...HEAD`; Findings beheben.
- [ ] Manueller Testplan `2026-11-13-oebb-rhb-legacy-parity-manual-tests.md` durchgehen.
