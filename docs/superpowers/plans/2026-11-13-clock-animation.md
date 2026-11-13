# Generische Frame-Animation + umuc-Uhr Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Templates bekommen generische Animations- und Ausdrucksbausteine:
- Frame-Modus (`meta.animation.frames/frame_delay/loop`),
- Variablen `anim.*`,
- Variablen/Unter-Pipelines als Argumente numerischer Filter, neue Filter `mod` und `minutes`,
- `meta.timezone`,
- zeitabhängige Templates werden nicht gecacht.

Damit wird `umuc-105-v1` aufgeräumt und bekommt eine korrekte Uhr mit laufendem Sekundenzeiger (60 × 1 s, Endlosschleife) sowie die Legacy-Minutenspalte.

**Architecture:**
- **Evaluator:** bekommt einen klammer- und anführungszeichenbewussten Pipeline-Parser. Numerische Filter werten nicht-literale Argumente rekursiv als Pipeline aus.
- **Renderer:** bezieht `now` aus einer injizierbaren Uhr in der Template-Zeitzone und hängt `anim.*` per `withAnim` an den Evaluator.
- **`RenderAnimation`:** hat einen zweiten Modus. N Frames, jeder durchläuft die ganze Layer-Liste mit `anim.frame = f`, `now` bleibt eingefroren.
- **`EncodeGIF`:** kennt festes Delay und Loop.
- **Server:** lädt das Template vor dem Cache-Lookup und umgeht den Cache für `UsesNow()`-Templates.

**Tech Stack:** Go 1.26, stdlib (`image`, `image/gif`, `time`, `time/tzdata`), `golang.org/x/image` (vorhanden), `gopkg.in/yaml.v3`. Node ≥ 20 nur für die vorhandenen JS-Tests. Keine neuen Abhängigkeiten.

**Spec:** `docs/superpowers/specs/2026-11-13-clock-animation-design.md`. Umsetzer lesen die Spec **und** ihren Task. Paragraphen-Verweise (§) beziehen sich auf die Spec.

## Global Constraints

- **Git:** Branch `feature/clock-animation`. Kein Push, kein Merge, kein `./build.sh` (würde `VERSION` erhöhen).
- **Build:** Server-Build bleibt CGO-frei. Keine neue Dependency in `go.mod`.
- **`umuc-096-v1`:** wird nicht angefasst.
- **Kompatibilität:** Bestehende Templates rendern bei fester Zeit identisch. Einzige erlaubte Abweichungen sind Zeitzone (Berlin) und Drehachse (§11). Splitflap-GIFs bleiben bytegleich.
- **Sicherheit:** `file` wird nie interpoliert. Bestehende Limits bleiben, neue Limits stehen in Spec §13.
- **Texte:** Fehlertexte im Stil des Pakets (`renderer: …`, englisch), UI-Texte deutsch.
- **Pro Task:**
  - TDD: Tests zuerst, rot sehen, dann implementieren.
  - Danach `go test ./...` grün und `go vet ./...` sauber.
  - Ab Task 6 zusätzlich `node --test web/static/test/*.mjs` grün.
  - Genau ein Commit pro Task mit der angegebenen Message plus Trailer-Zeile `Co-Authored-By: <Modell der ausführenden Session> <noreply@anthropic.com>` (Attribution der Session).
- **Zeit in Tests:** In Tests nie die echte Uhr verwenden, wenn ein Wert geprüft wird. Feste Zeit: `fixedNow = time.Date(2026, 11, 13, 9, 15, 30, 0, time.UTC)`, in Berlin (MEZ) 10:15:30.

## Review Focus

1. **Kompatibilität des Parsers:** Jeder bestehende Ausdruck ergibt dasselbe (Task 1, Kompatibilitätstest). Unbalancierte Klammern und Quotes führen zu keinem Panic.
2. **Leere/ungültige Zeiten und Mitternacht** in der Minutenspalte (Task 1, Task 7).
3. **Widersprüchliche `meta.animation`** (`frames` + `fps`, `frames` + `splitflap`, `frame_delay` allein) ergibt einen klaren Ladefehler (Task 4).
4. **Cache:** zeitabhängige Templates nie aus dem Cache, auch nicht nach der GIF-Wartezeit. Nicht zeitabhängige weiter `HIT` (Task 5).
5. **Splitflap unverändert:** bestehende Animation- und GIF-Tests grün ohne Anpassung der Erwartungswerte (Task 3, 4).

---

### Task 1: Pipeline-Parser, Variablen-Argumente, Filter `mod`/`minutes`

**Files:**
- Modify: `internal/renderer/evaluator.go`
- Test: `internal/renderer/evaluator_test.go` (erweitern), `internal/renderer/evaluator_compat_test.go` (neu), `internal/renderer/templates_all_test.go` (neu)

**Interfaces (Produces):**
- `const maxPipelineDepth = 4`
- `func splitTopLevel(s string, sep byte) []string`: trennt an `sep` außerhalb von `'…'`, `"…"` und `( … )`. Die Klammertiefe fällt nie unter 0, eine überzählige `)` zählt als normales Zeichen. Teile werden **nicht** getrimmt.
- `func (e *Evaluator) evalPipelineDepth(expr string, depth int) string`. `evalPipeline(expr)` ruft `evalPipelineDepth(expr, 0)` auf. Bei `depth > maxPipelineDepth` ist das Ergebnis `""`.
- `applyFilter(value, filter string, depth int) string` (neuer Parameter `depth`)
- `func (e *Evaluator) numericArg(arg string, depth int) (val string, fromExpr bool)`
- `func doMath(value, arg string, strictArg bool, op func(a, b float64) float64) string`
- Filter `mod(n)` und `minutes` (Spec §6.3)
- `parseArgs` nutzt `splitTopLevel(s, ',')`: Teile trimmen, leere verwerfen (wie bisher).
- `evalCond`: `fieldVal := e.evalPipelineDepth(strings.TrimSpace(args[0]), 1)`

- [ ] **Step 1: Sicherheitsnetz zuerst** (vor jeder Änderung, muss sofort grün sein). `templates_all_test.go` → `TestAllTemplates_Render`:
  - für jedes Verzeichnis unter `../../templates` mit `template.yaml`: `LoadTemplate`, `default.json` laden (fehlt sie, `{}`),
  - `New("../../templates").Render(...)` und `RenderAnimation(..., AnimationSeed(name, body))` ohne Fehler, `len(Frames) ≥ 1`,
  - Subtests per `t.Run(name, …)`.
- [ ] **Step 2: Kompatibilitätstest** `evaluator_compat_test.go`:
  - Kopiere die **aktuelle** (alte) Implementierung von `evalPipeline`, `applyFilter`, `parseFilterCall`, `parseArgs` und `doMath` unverändert als `legacyEvalPipeline` usw. in die Testdatei, mit Methoden auf einem eigenen Typ `legacyEvaluator{data map[string]interface{}; now time.Time}`. `resolveVar` darf über einen echten `*Evaluator` mit denselben Daten delegieren.
  - `TestPipeline_LegacyCompat`:
    - Sammle **jetzt einmalig** alle `interpolateRe`-Treffer aus allen `templates/*/template.yaml` (innerer Ausdruck, getrimmt, dedupliziert, z.B. per `grep -rhoE '\{\{[^}]*\}\}' templates/*/template.yaml | sort -u`) und lege sie als feste Liste `legacyCorpus` im Test ab. Kein dynamisches Einlesen: Task 7 führt neue Ausdrücke ein, die absichtlich anders auswerten.
    - Ergänze den festen Korpus: `x | mul(6)`, `x | mul('6')`, `x | add( 5 )`, `x|upper`, `x | round`, `x | div(0)`, `x | strip('*') | upper`, `x | stripBetween('(', ')')`, `x | before(':')`, `x | after(':')`, `now | format('HH:mm')`, `now.hour12 | mul(30)`.
    - Für jeden Ausdruck und jede `default.json` (plus Datensatz `{"x":"12.5"}`, `{"x":"abc"}`, `{"x":""}`): `newEvaluatorAtForTest(data, fixedNow)` (Helfer im Test: `e := NewEvaluator(data); e.now = fixedNow`) gegen `legacyEvaluator` mit gleichem `now`.
    - Erwartung: identische Strings.
  - Läuft vor der Implementierung grün, weil der Code noch gleich ist. Er muss nach Step 4 grün **bleiben**.
- [ ] **Step 3: Failing tests** in `evaluator_test.go` (Daten: `{"a":"5","b":"2.5","n":"-3","z1":"10:15","z2":"10:18","z3":"10:26","t1":"23:58","t2":"00:04","empty":"","txt":"abc","v":"a|b","seven":"7","sevenhalf":"7.5","mseven":"-7"}`):
  - `TestPipeline_VariableArgs`:
    - `{{a | add(b)}}` → `7.5`
    - `{{a|add(b)}}` → `7.5`
    - `{{a | mul(b) | round}}` → `13`
    - `{{a | add('3')}}` → `8`
    - `{{a | sub(missing)}}` → `""`
    - `{{txt | add(a)}}` → `abc`
    - `{{a | add(txt)}}` → `""`
  - `TestFilter_Minutes`:
    - `10:15` → `615`, `0:04` → `4`, `23:58` → `1438`, ` 10:15 Uhr` → `615`, `10:5` → `605`
    - `""` → `""`, `ca. 10` → `""`, `1015` → `""`, `:15` → `""`
  - `TestFilter_Mod`: `n | mod(1440)` → `1437`; `seven | mod(3)` → `1`; `sevenhalf | mod(2)` → `1.5`; `mseven | mod(-3)` → `-1`; `a | mod(0)` → `0`; `txt | mod(3)` → `abc`.
  - `TestPipeline_MinutesColumn` (Ausdruck `{{Z | minutes | sub(R | minutes) | mod(1440) | add(5)}}`):
    - Z = z1, R = z1 → `5`; z2/z1 → `8`; z3/z1 → `16`; t2/t1 → `11`
    - Z = empty, R = z1 → `""`; Z = z2, R = empty → `""`; Z = txt, R = z1 → `""`
  - `TestPipeline_ClockAngles` (`e.now` direkt setzen, `time.UTC`-Zeiten), Ausdruck `{{now.hour12 | mul(30) | add(now.minute | div(2))}}`: 10:15 → `307.5`, 12:00 → `360`, 00:30 → `375`, 03:00 → `90`.
  - `TestPipeline_QuotedPipeAndParens`: `{{v | stripAll('|')}}` → `ab`; `{{v | before('|')}}` → `a`.
  - `TestPipeline_DepthLimit`: `{{a | add(a | add(a | add(a | add(a))))}}` (4 Ebenen) → `25`; mit 5 Ebenen `{{a | add(a | add(a | add(a | add(a | add(a)))))}}` → `""`.
  - `TestPipeline_Malformed`: kein Panic, Ergebnis wie angegeben: `{{a | add(b}}` → `5` (kein schließendes `)` ⇒ unbekannter Filter `add(b`); `{{a | add()}}` → `5` (kein Argument); `{{a | add(b))}}` → `""` (Argument `b)` ist ein unbekannter Pfad); `{{a | add('x)}}` → `""` (offenes Quote ⇒ Argument `'x` ist kein Literal, sondern ein unbekannter Pfad); `{{ | }}` → `""`. Zusätzlich eine Schleife über 2000 zufällige Strings (fester `math/rand`-Seed, Alphabet `a|(),'"0 .`), jeweils `Interpolate("{{"+s+"}}")` ohne Panic.
  - `TestCondition_PipelineArg`: `greaterThan(z2 | minutes, 600)` → true; `greaterThan(z1 | minutes, 615)` → false; `equals(z1 | minutes, '615')` → true; `isEmpty(empty | minutes)` → true; `startsWith(txt, 'a')` → true (unverändert).
  - `TestSplitTopLevel`: `"a | b(c | d) | e"` mit `'|'` → `["a ", " b(c | d) ", " e"]`; `"'x|y' | z"` → `["'x|y' ", " z"]`; `"a)|b"` → `["a)", "b"]`; `"x, f(a, b), 'c,d'"` mit `','` → `["x", " f(a, b)", " 'c,d'"]`.
- [ ] **Step 4:** `go test ./internal/renderer/ -run 'Pipeline|Filter_M|Condition_PipelineArg|SplitTopLevel'` → FAIL.
- [ ] **Step 5: Implementieren** in `evaluator.go`:
  - `pipelineSplitRe` entfernen. Stattdessen `splitTopLevel(expr, '|')` und jede Stufe trimmen.
  - `numericArg`:
    1. `t := strings.TrimSpace(arg)`.
    2. In Anführungszeichen → `unquote(t), false`.
    3. `strconv.ParseFloat(t, 64)` ok → `t, false`.
    4. Sonst → `e.evalPipelineDepth(t, depth+1), true`.
  - `doMath(value, arg, strictArg, op)`:
    - Ist `value` keine Zahl, gib `value` zurück.
    - Ist `arg` keine Zahl, gib bei `strictArg` `""` zurück, sonst `value`.
    - Ausgabeformat wie bisher.
  - `mod`: `r := math.Mod(a, b); if r != 0 && (r < 0) != (b < 0) { r += b }`; `b == 0` → `0`.
  - `minutes`: `var minutesRe = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})`)` auf `strings.TrimSpace(value)`.
  - Kommentare an `Interpolate`/`evalPipeline` auf die neue Grammatik (Spec §6.1) aktualisieren.
- [ ] **Step 6:** `go test ./...` → PASS (inkl. Kompatibilitätstest und `TestAllTemplates_Render`). `go vet ./...` sauber.
- [ ] **Step 7:** Commit `feat(renderer): variable filter arguments, nested pipelines, mod and minutes filters`.

---

### Task 2: Zeitzone, injizierbare Uhr, exakte Drehachse

**Files:**
- Create: `internal/renderer/timezone.go`, `internal/renderer/timezone_test.go`
- Modify: `internal/renderer/template.go` (`Meta.Timezone`, `Template.loc`), `internal/renderer/renderer.go` (`Renderer.nowFunc`, `LoadTemplate`, `Render`, `rotateImageCW`), `internal/renderer/animation.go` (Evaluator-Erzeugung), `internal/renderer/evaluator.go`
- Test: `internal/renderer/renderer_test.go` (Drehachse)

**Interfaces (Produces):**
- `Meta.Timezone string \`yaml:"timezone"\``
- `const defaultTimezone = "Europe/Berlin"`, `const maxTimezoneLen = 64`
- `func loadTimezone(name string) (*time.Location, error)`: `""` → Berlin. Validierung nach Spec §8. `timezone.go` enthält `import _ "time/tzdata"`.
- `Template.loc *time.Location` (unexportiert, gesetzt in `LoadTemplate`) und `func (t *Template) location() *time.Location`: `t.loc`, wenn gesetzt, sonst `loadTimezone(t.Meta.Timezone)`, bei Fehler Berlin.
- `Renderer.nowFunc func() time.Time` (nil = `time.Now`) und `func (r *Renderer) clock() time.Time`
- `func newEvaluatorAt(data map[string]interface{}, now time.Time) *Evaluator`
- `func (r *Renderer) newEvaluator(tmpl *Template, data map[string]interface{}) *Evaluator` = `newEvaluatorAt(data, r.clock().In(tmpl.location()))`. In Task 4 kommt `withAnim` dazu.
- `LoadTemplate`: nach den bestehenden Prüfungen `loc, err := loadTimezone(tmpl.Meta.Timezone)`, bei Fehler `renderer: LoadTemplate: invalid meta.timezone %q: %w`. Dann `tmpl.loc = loc`.

- [ ] **Step 1: Failing tests:**
  - `TestLoadTimezone`: ok für `""` (→ `Europe/Berlin`, `loc.String()`), `Europe/Berlin`, `UTC`, `America/New_York`, `Europe/Zurich`. Fehler für `Mars/Olympus`, `../../etc/passwd`, `/etc/localtime`, `Local`, `Europe/Berlin\n`, einen 65 Zeichen langen Namen und `Europe//Berlin`.
  - `TestLoadTemplate_Timezone`: Temp-Template mit `meta.timezone: "Mars/Olympus"` → Fehler enthält `invalid meta.timezone`. Mit `UTC` → ok, `tmpl.location().String() == "UTC"`.
  - `TestRender_TemplateTimezone`: In-Memory-Template 1×1. Layer `rect` 0,0,1,1 mit `Color` als Bedingung: `StringOrCond{cond: &condMap{branches: []condBranch{{ifExpr: "equals(now.hour, '10')", then: "#ff0000"}}, els: "#00ff00"}}`. `r := New("/tmp"); r.nowFunc = func() time.Time { return fixedNow }`.
    - Default-Zeitzone → Pixel rot.
    - `tmpl.Meta.Timezone = "UTC"` → grün.
    - Sommerzeit: `nowFunc` → `2026-07-01T08:00:00Z`, Default → rot (10 Uhr MESZ).
  - `TestRotateImageCW_ParityAndAxis`:
    - (a) Für src 70×70 und alle Winkel 0…359 (Schritt 1) sind `Dx` und `Dy` des Ergebnisses gerade. Für src 9×9 bei 0, 30, 45, 90 sind sie ungerade.
    - (b) src 70×70 transparent mit deckendem 2×2-Block bei Pixeln (34..35, 34..35), also um die Bildmitte 35,0. Für Winkel {0, 17, 45, 90, 133, 180, 271}: alpha-gewichteter Schwerpunkt (Pixelmitte = x+0,5) des Ergebnisses minus `(Dx/2, Dy/2)` (Ganzzahl-Division wie in `renderImage`) liegt in jeder Achse innerhalb ±0,15 von 0.
    - Vor dem Fix schlägt das bei 45° fehl.
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'Timezone|TemplateTimezone|RotateImageCW'` → FAIL.
- [ ] **Step 3: Implementieren:**
  - `timezone.go`: Regex `^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`. Länge prüfen, `Local` ablehnen, dann `time.LoadLocation`. Berlin einmalig laden: `var berlin = sync.OnceValue(func() *time.Location { l, err := time.LoadLocation(defaultTimezone); if err != nil { return time.UTC }; return l })`.
  - `Render` und `RenderAnimation` nutzen `r.newEvaluator(tmpl, data)` statt `NewEvaluator(data)`. `withLoopVars` übernimmt `now` wie bisher.
  - `rotateImageCW`: nach der `ceil`-Berechnung `if (newW-bounds.Dx())%2 != 0 { newW++ }`, analog `newH`.
  - Kommentar an `NewEvaluator`: „uses time.Now in the local zone; the renderer uses newEvaluator (template time zone, injectable clock)".
- [ ] **Step 4:** `go test ./...` → PASS. Ein Binary mit `GOOS=windows go build ./cmd/zza-server` baut (nur Kompilierprüfung, Binary verwerfen).
- [ ] **Step 5:** Commit `feat(renderer): meta.timezone (default Europe/Berlin), injectable clock, exact rotate axis`.

---

### Task 3: GIF-Encoder — festes Delay und Endlosschleife

**Files:**
- Modify: `internal/renderer/animation.go` (nur `type Animation`), `internal/renderer/gifenc.go`
- Test: `internal/renderer/gifenc_test.go`

**Interfaces (Produces):**
```go
// Animation is a rendered frame sequence.
type Animation struct {
	Frames []*image.NRGBA
	FPS    int  // splitflap mode: every frame lasts round(100/FPS) cs, the last frame gifFinalDelay
	Delay  int  // frame mode: every frame lasts Delay cs (1–1000); when > 0, FPS is ignored and the last frame is not held
	Loop   bool // GIF repeats forever (NETSCAPE loop count 0); false = plays once
}
```
- `const maxGIFDelay = 1000`
- `EncodeGIF`:
  - `anim.Delay < 0 || anim.Delay > maxGIFDelay` → Fehler `renderer: EncodeGIF: delay must be between 0 and 1000 (got %d)`.
  - `anim.Delay == 0 && anim.FPS <= 0` → bestehender FPS-Fehler.
  - `LoopCount`: `0` bei `Loop`, sonst `-1`.

- [ ] **Step 1: Failing tests:**
  - `TestEncodeGIF_FrameDelay`: 3 verschiedene Frames (`gifSquare` an x = 0, 4, 8; 16×16), `Delay: 100`, `FPS: 0` → `g.Delay == [100 100 100]`, `g.LoopCount == -1`.
  - `TestEncodeGIF_Loop`: 60 Frames 16×16, ein 1×1-Pixel wandert (`x = i % 16`, `y = i / 16`), `Delay: 100`, `Loop: true` → `g.LoopCount == 0`, `len(g.Image) == 60`, alle Delays 100. Rekonstruktion (`reconstruct`) pixelgleich zu jedem Frame.
  - `TestEncodeGIF_LoopSplitflapTiming`: 3 verschiedene Frames, `FPS: 20`, `Loop: true` → Delays `[5 5 100]`, `LoopCount == 0`.
  - `TestEncodeGIF_FrameDelayMergesIdentical`: Frames A, A, B mit `Delay: 100` → `len(g.Image) == 2`, Delays `[200 100]`.
  - `TestEncodeGIF_DelayErrors`: `Delay: 1001` → Fehler mit `delay must be between`; `Delay: -1` → Fehler; `Delay: 0, FPS: 0` → Fehler mit `fps must be positive`.
  - Bestehende Tests bleiben unverändert grün (sie setzen nur `FPS`, also `Delay = 0` und `Loop = false`).
- [ ] **Step 2:** `go test ./internal/renderer/ -run EncodeGIF` → FAIL.
- [ ] **Step 3: Implementieren:**
  - `d := delay; if anim.Delay > 0 { d = anim.Delay } else if i == len(anim.Frames)-1 { d = gifFinalDelay }`.
  - Doc-Kommentar von `EncodeGIF` anpassen: Loop/Delay beschreiben, „plays exactly once" nur noch für `Loop == false`.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5:** Commit `feat(renderer): GIF encoder supports fixed frame delay and infinite loop`.

---

### Task 4: Frame-Modus und `anim.*`-Variablen

**Files:**
- Modify: `internal/renderer/template.go` (`AnimationMeta`, `IsAnimated`), `internal/renderer/animation.go` (Validierung, `RenderAnimation`), `internal/renderer/evaluator.go` (`anim`, `withAnim`, `resolveVar`), `internal/renderer/renderer.go` (`LoadTemplate`, `newEvaluator`)
- Create: `internal/renderer/frames_test.go`

**Interfaces (Produces):**
```go
type AnimationMeta struct {
	FPS        int     `yaml:"fps"`         // splitflap: frames per second; 0 = 20, allowed 5–25
	Duration   float64 `yaml:"duration"`    // splitflap: max running time in s; 0 = 2.5, allowed 0.5–5
	Frames     int     `yaml:"frames"`      // frame mode: number of frames; 0 = off, allowed 2–125
	FrameDelay float64 `yaml:"frame_delay"` // frame mode: seconds per frame; 0 = 1, allowed 0.02–10 in 0.01 steps
	Loop       bool    `yaml:"loop"`        // GIF repeats forever (both modes)
}
```
- Konstanten: `minAnimFrames = 2`, `defaultFrameDelay = 1.0`, `minFrameDelay = 0.02`, `maxFrameDelay = 10.0`, `maxAnimSeconds = 120.0`. `maxAnimFrames = 125` und `maxAnimPixels` werden wiederverwendet.
- `func (a AnimationMeta) frameDelayCS() int`: `round(FrameDelay·100)`, bei 0 → 100.
- `func (t *Template) validateAnimation() error`:
  - ruft das bestehende `resolve()` auf (fps/duration-Bereiche) und prüft die Regeln aus Spec §7.2 mit **genau** den dort genannten Fehlertexten,
  - `frame_delay` gilt als Vielfaches von 0.01, wenn `math.Abs(d*100-math.Round(d*100)) < 1e-9`,
  - `LoadTemplate` ersetzt den bisherigen `resolve()`-Aufruf durch `tmpl.validateAnimation()`.
- `func (t *Template) IsAnimated() bool { return t.Meta.Animation.Frames > 0 || t.HasSplitflap() }`
- `func (t *Template) animFrames() int`: `Frames`, wenn > 0, sonst 1.
- Evaluator: Feld `anim animVars` mit `type animVars struct{ frame, frames, delayCS int }`. `NewEvaluator` und `newEvaluatorAt` setzen `frames: 1`.
- `func (e *Evaluator) withAnim(frame, frames, delayCS int) *Evaluator`: Kopie von `data` und `now`. `intVars` werden kopiert plus `"anim.frame": frame, "anim.frames": frames`. `withLoopVars` übernimmt `anim` ebenfalls.
- `resolveVar`: `name == "anim" || strings.HasPrefix(name, "anim.")` → `resolveAnim`:
  - `anim.frame` → `strconv.Itoa(frame)`
  - `anim.frames` → `strconv.Itoa(frames)`
  - `anim.time` → `strconv.FormatFloat(float64(frame*delayCS)/100, 'f', -1, 64)`
  - sonst `""`
- `Renderer.newEvaluator` hängt an: `.withAnim(0, tmpl.animFrames(), delayCS)` mit `delayCS = tmpl.Meta.Animation.frameDelayCS()` im Frame-Modus, sonst 0.
- `RenderAnimation`: Frame-Modus-Zweig nach Spec §7.4 (vor der Splitflap-Logik). Der Splitflap-Zweig und der nicht animierte Zweig setzen zusätzlich `Loop: tmpl.Meta.Animation.Loop` (Splitflap) bzw. `false`. Doc-Kommentar aktualisieren.

- [ ] **Step 1: Failing tests** (`frames_test.go`; Helfer `writeTmpl(t, yaml string) (dir, name string)` legt ein Temp-Template an, analog `writeAnimTemplate` in `animation_test.go`):
  - `TestAnimationMeta_Validate` (Tabelle, `LoadTemplate` auf Temp-YAML mit Canvas 10×10 und einem `rect`-Layer; Erwartung ok bzw. Fehler-Teilstring):
    - `frames: 60, frame_delay: 1, loop: true` → ok
    - `frames: 60` (ohne Delay) → ok, `frameDelayCS() == 100`
    - `frames: 120, frame_delay: 1` → ok
    - `frames: 1` → `between 2 and 125`
    - `frames: 126` → `between 2 and 125`
    - `frames: -1` → `between 2 and 125`
    - `frames: 60, fps: 20` → `cannot be combined`
    - `frames: 60, duration: 2` → `cannot be combined`
    - `frame_delay: 1` (ohne frames) → `requires animation.frames`
    - `frames: 10, frame_delay: 0.015` → `between 0.02 and 10`
    - `frames: 10, frame_delay: 0.025` → `steps of 0.01`
    - `frames: 10, frame_delay: 10.5` → `between 0.02 and 10`
    - `frames: 125, frame_delay: 1` → `must not exceed 120 seconds`
    - `frames: 2` + `splitflap`-Layer (YAML wie `flapTemplateYAML` in `internal/server/format_test.go`) → `splitflap layers`
    - `fps: 60` → bestehender fps-Fehler (unverändert)
    - `loop: true` ohne frames und ohne splitflap → ok
  - `TestIsAnimated`: `flapAnimTemplate(2, "AB")` → true; Template mit `Meta.Animation.Frames = 60` → true; `makeTemplate(1,1,nil)` → false.
  - `TestEvaluator_AnimVars`:
    - `e := NewEvaluator(map[string]interface{}{"anim": map[string]interface{}{"frame": "x"}}).withAnim(3, 60, 100)`: `{{anim.frame}}` → `3` (JSON-`anim` verdeckt); `{{anim.frames}}` → `60`; `{{anim.time}}` → `3`; `{{anim.foo}}` → `""`.
    - `withAnim(3, 10, 10)` → `{{anim.time}}` → `0.3`.
    - `evalIntExpr("anim.frame * 2 + anim.frames", e.intVars)` → `66`.
    - Nach `e.withLoopVars(map[string]string{"v":"a"}, map[string]int{"i": 1})` weiterhin `{{anim.frame}}` → `3` und `evalIntExpr("anim.frame + i", …)` → `4`.
    - `NewEvaluator(nil)`: `{{anim.frame}}` → `0`, `{{anim.frames}}` → `1`.
  - `TestRenderAnimation_FrameMode`: Template 10×1, `Meta.Animation{Frames: 5, FrameDelay: 0.5, Loop: true}`. Layer: `rectLayer(0,0,10,1,"#000000")`, dann `rect` mit `X: IntOrExpr{expr: "anim.frame", isExpr: true}`, y 0, 1×1, rot.
    - Ergebnis: `len(Frames) == 5`, `Delay == 50`, `Loop == true`.
    - Frame f hat bei (f,0) Rot, alle anderen Pixel schwarz.
  - `TestRenderAnimation_FrameMode_StringVars`: Template 1×1, Frames 3. `rect` mit Farbe als Bedingung `equals(anim.frame, '1')` → `#ff0000`, sonst `#0000ff` → Frames blau, rot, blau. Zweite Bedingung `equals(anim.frames, '3')` in einem Block → greift in jedem Frame.
  - `TestRenderAnimation_FrameMode_NowFrozen`: `r.nowFunc` liefert bei jedem Aufruf 1 s später (Start `fixedNow`, Zähler in Closure). Template 1×1, Frames 4, Farbe `equals(now.second, '30')` → rot, sonst grün. Alle 4 Frames rot.
  - `TestRenderAnimation_FrameMode_FirstFrameEqualsRender`: Template aus `TestRenderAnimation_FrameMode`, fester `nowFunc` → `Render(...).Pix` == `Frames[0].Pix`. Zweiter `RenderAnimation`-Aufruf: alle Frames identisch (Determinismus).
  - `TestRenderAnimation_FrameMode_PixelLimit`: Canvas 1000×1000, Frames 40 → Fehler mit `animation too large`.
  - `TestRender_StaticAnimVars`: Template aus `TestRenderAnimation_FrameMode` → `Render`: Pixel (0,0) rot, (1,0) schwarz.
  - `TestRenderAnimation_SplitflapLoop`: `flapAnimTemplate(4, "AB")` mit `Meta.Animation.Loop = true` → `anim.Loop == true`, `anim.Delay == 0`. Ohne `Loop` → `false`.
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'AnimationMeta_Validate|IsAnimated|AnimVars|FrameMode|StaticAnimVars|SplitflapLoop'` → FAIL.
- [ ] **Step 3:** Implementieren (siehe Interfaces). Die Pixelbudget-Prüfung des Frame-Modus steht vor dem ersten gerenderten Frame. Der Bild-Cache (`imgCache`) wird über alle Frames geteilt wie im Splitflap-Zweig.
- [ ] **Step 4:** `go test ./...` → PASS. Insbesondere laufen alle bestehenden `TestRenderAnimation_*`- und `TestEncodeGIF_*`-Tests unverändert grün.
- [ ] **Step 5:** Commit `feat(renderer): generic frame animation mode with anim.* variables`.

---

### Task 5: Server — Cache-Bypass für zeitabhängige Templates, `IsAnimated` überall

**Files:**
- Create: `internal/renderer/usesnow.go`, `internal/renderer/usesnow_test.go`
- Modify: `internal/server/server.go`, `internal/cli/render.go` (Hilfetext), `web/templates/detail.html` und `web/templates/edit-editor.html` (nur Kommentare)
- Test: `internal/server/format_test.go`

**Interfaces (Produces):**
- `func (t *Template) UsesNow() bool` (Spec §9). Hilfsfunktionen: `layersUseNow([]Layer) bool`, `exprUsesNow(s string) bool` (Token-Regex `(^|[^A-Za-z0-9_.])now([^A-Za-z0-9_]|$)` oder `\bformat\s*\(`) und `interpUsesNow(s string) bool` (prüft jeden `interpolateRe`-Treffer mit `exprUsesNow`). Für `StringOrCond`: `raw`, jedes `then`, `els` über `interpUsesNow`, jedes `ifExpr` über `exprUsesNow`. Für `Layer`: `Value`, `Color`, `Rotate`, `If`, `Elif`, rekursiv `Layers`.
- `server.go`:
  - `renderAndServe` lädt das Template direkt nach dem Bau von `key` und **vor** `GetExt` (404 wie bisher), `cacheable := !tmpl.UsesNow()`.
  - Lookup und Speichern nur bei `cacheable`. Der spätere zweite `LoadTemplate`-Aufruf entfällt.
  - `renderGIF(ctx, w, templateName, body, key, tmpl, data, cacheable bool)`: Recheck nach der Wartezeit und `SetExt` nur bei `cacheable`.
  - `writeImage(w, format, cacheState, data)` setzt bei `cacheState == "BYPASS"` zusätzlich `Cache-Control: no-store`. Der PNG-Pfad nutzt jetzt auch `writeImage` (`"MISS"` bzw. `"BYPASS"`).
- `isAnimated` und `detailData.Animated` nutzen `tmpl.IsAnimated()`. Kommentare anpassen („template is animated: splitflap or frame mode").
- `internal/cli/render.go`, Long-Text: „A GIF of a template without animation (splitflap or meta.animation.frames) has a single frame."

- [ ] **Step 1: Failing tests:**
  - `TestUsesNow` (Tabelle mit In-Memory-Layern):
    - true: `value: "{{now.hour}}"`; `value: "{{zug1.zeit | format('HH:mm')}}"`; `value: "{{x | add(now.second)}}"`; `If: "greaterThan(now.hour, 5)"`; Farbe mit Zweig `then: "{{now.second}}"`; Bedingung `ifExpr: "equals(now.minute, '0')"`; `Rotate` mit now; Kind-Layer in Block, in `loop` und in `splitflap`.
    - false: `value: "now"` (ohne Klammern); `value: "Unknown {{zug1.nr}}"`; `value: "{{nowhere}}"`; `value: "{{snow.x}}"`; `value: "{{zug1.now}}"`; `value: "{{anim.frame}}"`.
  - `TestUsesNow_RealTemplates`: `LoadTemplate("../../templates", name)`:
    - true für `nederland-096-v1`, `nederland-105-v1`, `oebb-096-v1`, `oebb-105-v1`, `umuc-096-v1`, `umuc-105-v1`,
    - false für `faltblatt-105-v1`, `rhb-096-v1`, `rhb-105-v1`, `sbb-096-v1`, `sbb-105-v1`.
  - `format_test.go`, neuer Helfer `newTemplateServer(t, files map[string]string) (*Server, cacheDir string)`. `newFlapServer` darf ihn nutzen.
    - Template `n` (zeitabhängig): `meta: {canvas: {width: 4, height: 4}}`, ein `rect` 0,0,4,4 mit `color: {if: "greaterThan(now.second, -1)", then: "#ff0000", else: "#00ff00"}`.
    - Template `f` (Frame-Modus, nicht zeitabhängig): `meta: {canvas: {width: 4, height: 1}, animation: {frames: 4, frame_delay: 1, loop: true}}`, schwarzes `rect` 0,0,4,1, rotes `rect` mit `x: "{{anim.frame}}"`, 1×1.
  - `TestRender_UsesNow_BypassesCache`:
    - Zweimal `POST /n/render` → beide 200, `X-Cache: BYPASS`, `Cache-Control: no-store`.
    - Danach enthält `cacheDir` (rekursiv) keine Datei mit Endung `.png` oder `.gif`.
    - Dasselbe für `?format=gif` und für `GET /n/preview`.
  - `TestRender_UsesNow_GIFStillLimited`: `srv.gifWaitTimeout = 50 * time.Millisecond`; zwei Slots belegen (`srv.gifSem <- struct{}{}` zweimal); `POST /n/render?format=gif` → 503 mit `Retry-After`. Slots wieder freigeben.
  - `TestRender_FrameMode_GIF`: `POST /f/render?format=gif` → 200; `gif.DecodeAll`: `len(Image) == 4`, alle Delays 100, `LoopCount == 0`. Zweiter Aufruf → `X-Cache: HIT` (Frame-Modus ohne now bleibt cachebar).
  - `TestDetail_AnimatedFlag_FrameMode`: `GET /f` enthält `id="play-anim-btn"` und `id="download-gif-btn"`; `getEditorPage(srv, "f")` (Helfer aus `format_test.go`, wie in `TestEditor_AnimatedControls`) enthält `id="preview-mode"`.
  - Bestehende Tests (u.a. `TestHandleRender…` mit `sbb-096-v1` → `HIT`) bleiben unverändert grün.
- [ ] **Step 2:** `go test ./internal/renderer/ -run UsesNow` und `go test ./internal/server/` → FAIL.
- [ ] **Step 3:** Implementieren. Den Doc-Kommentar von `renderAndServe` um den Bypass ergänzen. Kommentare in `detail.html` („Animation controls exist only for templates with splitflap layers") und `edit-editor.html` („only for templates with splitflap layers") auf „animierte Templates (splitflap oder Frame-Modus)" ändern. Kein Verhalten im JS ändern.
- [ ] **Step 4:** `go test ./...` → PASS. Danach manuell:
  - `PORT=18080 TEMPLATES_DIR=./templates CACHE_DIR=<scratch>/cache go run ./cmd/zza-server serve` starten,
  - `curl -s -D - -o /dev/null -X POST localhost:18080/nederland-105-v1/render -H 'Content-Type: application/json' --data-binary @templates/nederland-105-v1/default.json` → `X-Cache: BYPASS`,
  - `sbb-096-v1` zweimal → `MISS`, dann `HIT`,
  - Server beenden.
- [ ] **Step 5:** Commit `feat(server): bypass cache for time-dependent templates; animated flag for frame mode`.

---

### Task 6: Node-Editor — verschachtelte Filter-Argumente, `mod`, `minutes`

**Files:**
- Modify: `web/static/node-filters.js`
- Test: `web/static/test/node-filters.test.mjs`

**Interfaces (Produces):**
- `export function splitTopLevel(str, sep)`: gleiche Semantik wie Go `splitTopLevel` (Quotes `'`/`"`, Klammertiefe, keine Trimmung).
- `parseValueAndFilters`: `inner` per `splitTopLevel(inner, '|')`, Teile trimmen. Ergibt sich nur ein Teil, bleibt es bei `{ base: str, filters: [] }` (wie bisher). Filter-Argument = Text zwischen erster `(` und **letzter** `)` des Teils.
- `FILTER_DEFS`: `{ fn: 'mod', label: 'mod', hasArg: true, category: 'math' }` (nach `sub`) und `{ fn: 'minutes', label: 'minutes', hasArg: false, category: 'time' }` (vor `format`).
- `_applyFilter(value, fn, arg, testJson)`:
  - Für `mul`/`div`/`add`/`sub`/`mod` gilt: Ist `arg` weder quotiert noch eine Zahl, wird es als Pipeline gegen `testJson` ausgewertet (rekursiv über eine interne Funktion `_evalPipeline(expr, testJson, depth)`, Tiefe ≤ 4).
  - `mod` floored wie in Go.
  - `minutes` per `/^(\d{1,2}):(\d{1,2})/` auf `value.trim()`, sonst `''`.
  - Ist ein Variablen-Argument keine Zahl → `''`.
  - `evaluatePreview` reicht `testJson` durch.

- [ ] **Step 1: Failing tests** (bestehende Tests nicht ändern):
  - Parse `{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}` → `base: '{{zug2.zeit}}'`, `filters: [{fn:'minutes',arg:null},{fn:'sub',arg:'zug1.zeit | minutes'},{fn:'mod',arg:'1440'},{fn:'add',arg:'5'}]`.
  - Roundtrip `formatValueWithFilters(parse(s).base, parse(s).filters) === s` für den Ausdruck oben und für `{{now.second | add(anim.frame) | mul(6)}}` sowie `{{now.hour12 | mul(30) | add(now.minute | div(2))}}`.
  - Parse `{{v | stripAll('|')}}` → genau ein Filter `stripAll` mit Arg `"'|'"`.
  - `splitTopLevel("a | b(c | d) | e", '|')` → `['a ', ' b(c | d) ', ' e']`.
  - `evaluatePreview` mit `testJson = {zug1:{zeit:'10:15'}, zug2:{zeit:'10:18'}, t1:'23:58', t2:'00:04', n:'-3'}`:
    - Minutenformel zug2/zug1 → `'8'`
    - t2/t1 → `'11'`
    - `{{n}}` + `mod(1440)` → `'1437'`
    - `{{zug1.zeit}}` + `minutes` → `'615'`
    - `{{zug1.zeit}}` + `sub(missing)` → `''`
- [ ] **Step 2:** `node --test web/static/test/*.mjs` → FAIL (neue Tests).
- [ ] **Step 3:** Implementieren.
- [ ] **Step 4:** `node --test web/static/test/*.mjs` → PASS (alle, inkl. bestehende 67). `go test ./...` → PASS (das `web`-Embed kompiliert).
- [ ] **Step 5:** Commit `feat(web): node editor parses nested filter arguments; mod and minutes filters`.

---

### Task 7: Template `umuc-105-v1` — aufräumen, Uhr, Sekundenzeiger, Minutenspalte

**Files:**
- Create: `templates/umuc-105-v1/clock-seconds.png`, `internal/renderer/umuc_test.go`
- Modify: `templates/umuc-105-v1/template.yaml` (komplett ersetzen), `templates/umuc-105-v1/clock-hour.png`, `templates/umuc-105-v1/clock-minutes.png`
- Nicht ändern: `templates/umuc-096-v1/*`, `templates/umuc-105-v1/default.json`

- [ ] **Step 1: Failing tests** `umuc_test.go`. Helfer: `loadUmuc(t)` lädt Template und `default.json`, `r := New("../../templates"); r.nowFunc = func() time.Time { return fixedNow }` (09:15:30 UTC = 10:15:30 Berlin). `reddish(img, x, y)` = irgendein Pixel im 3×3-Fenster um (x,y) mit `R > 150 && G < 120 && int(R)-int(G) > 80`.
  - `TestUmuc105_Meta`: `Meta.Animation == {Frames: 60, FrameDelay: 1, Loop: true}`, `IsAnimated()`, `UsesNow()`, `location().String() == "Europe/Berlin"`.
  - `TestUmuc105_Expressions`:
    - Alle `text`-Layer mit `X.Resolve(nil) == 238` in Reihenfolge, `Value` mit `r.newEvaluator(tmpl, data)` interpoliert → `["5", "8", "16"]`.
    - Mit `zug1.zeit = "23:58"`, `zug2.zeit = "00:04"`, `zug3.zeit = ""` → `["5", "11", ""]`.
    - `rotate` der Layer mit `File` `clock-hour.png` / `clock-minutes.png` / `clock-seconds.png` → `307.5` / `90` / `180`.
    - Mit `withAnim(59, 60, 100)` ist der Sekundenwinkel `534`.
  - `TestUmuc105_Badges` (Pixelvergleich, Bereich `image.Rect(120, 62, 139, 75)` = Größe von `U2.png`):
    - Render mit `zug2.nr = "U2"`: Der Bereich ist pixelgleich zu `U2.png` (dekodiert, RGB-Vergleich).
    - Render mit `zug2.nr = "U9"`: Der Bereich unterscheidet sich von dem bei `"U2"`, und es gibt dort mindestens ein Pixel mit R, G, B > 200 (weißer Text-Fallback „U9").
    - Render mit `zug2.nr = ""`: kein Fehler.
  - `TestUmuc105_SecondHand`: `RenderAnimation` → 60 Frames, `Delay == 100`, `Loop == true`.
    - Frame 0 (Sekunde 30, Zeiger nach unten): `reddish(39, 76)`, gespiegelte Uhr `reddish(201, 196)`.
    - Frame 15 (Sekunde 45, nach links): `reddish(19, 56)` und `reddish(181, 176)`, **nicht** `reddish(39, 76)`.
    - Frame 0 == `Render`, Frame 0 ≠ Frame 1.
    - Bereich der Zugliste (x 120–239, y 18–113) ist in allen Frames identisch.
  - `TestUmuc105_HandAxis`: `clock-hour.png`, `clock-minutes.png` und `clock-seconds.png` sind 70×70. Der alpha-gewichtete x-Schwerpunkt (Pixelmitte +0,5) jedes PNGs liegt bei 35,0 ± 0,3. Ist der Ist-Zustand 36,0 → rot vor dem Verschieben.
- [ ] **Step 2:** `go test ./internal/renderer/ -run Umuc105` → FAIL.
- [ ] **Step 3: PNGs.**
  - Wegwerf-Go-Programm im Scratchpad (nicht im Repo), mit `go run <scratch>/genhands/main.go <repo>/templates/umuc-105-v1`.
  - `clock-hour.png` und `clock-minutes.png` dekodieren, jedes Pixel (x,y) nach (x−1,y) kopieren. Die Spalte 0 ist leer, prüfen und sonst abbrechen. Die letzte Spalte wird transparent, als NRGBA-PNG neu speichern.
  - `clock-seconds.png` erzeugen: 70×70 NRGBA, Farbe `#e2001a`. Abdeckung per 8×8-Supersampling für die Vereinigung aus
    - Rechteck x ∈ [34,25; 35,75], y ∈ [2; 46] und
    - Kreis um (35,0; 35,0) mit r = 2,5.
  - Alpha = Abdeckung × 255.
  - Sichtprüfung: Die drei PNGs mit dem Read-Tool ansehen, und ein PNG-Render von `umuc-105-v1` ansehen (nach Step 4).
- [ ] **Step 4: `template.yaml` vollständig ersetzen** durch:

```yaml
meta:
  name: "U-Muc 1.05\""
  description: "U-Bahn München, 240×240px, 1.05 Zoll, Bahnhofsuhr mit laufendem Sekundenzeiger"
  author: "christoph"
  version: "2.0"
  canvas:
    width: 240
    height: 240
  timezone: "Europe/Berlin"
  animation:
    frames: 60        # eine Minute …
    frame_delay: 1    # … in Sekundenschritten, danach von vorn
    loop: true

fonts:
  - id: bold
    file: Roboto-Bold.ttf
  - id: regular
    file: RobotoCondensed-Regular.ttf

# Aufbau wie legacy/umuc-105-v1/index.php: Gezeichnet wird die obere
# Bildhälfte (links Uhr + Gleis, rechts Abfahrtsliste); zwei copy-Layer am
# Ende setzen sie vertauscht in die untere Hälfte. Alles vor den copy-Layern
# — auch die Uhrzeiger — erscheint deshalb zweimal.
# Schriftgrößen = Legacy-Größe × 4/3; Positionen = Oberkante der Schrift.
# Linien-Badges: file ist aus Sicherheitsgründen nicht interpolierbar, daher
# je Zeile eine if/elif-Kette über U1–U8; jede andere Linie (auch leer)
# landet im else-Zweig als Text.
# Spalte „Min.": wie im Original relativ zu Zug 1, der fest mit 5 Minuten
# angezeigt wird; mod(1440) hält den Wert über Mitternacht positiv.
# Leere oder ungültige Zeiten ergeben eine leere Zelle.

layers:
  # Hintergrund mit Zifferblatt, Logo und Notruf-Schild
  - type: image
    file: bg.png
    x: 0
    y: 0

  # Uhr: Zifferblatt-Mitte (39,56). Zeiger-PNGs 70×70, Drehpunkt = Bildmitte,
  # bei 0° nach oben; rotate dreht im Uhrzeigersinn, x/y = Drehpunkt.
  # now ist pro Render eingefroren: Stunde und Minute stehen im GIF still,
  # nur der Sekundenzeiger läuft über anim.frame (60 Frames à 1 s).
  - type: image
    file: clock-hour.png
    x: 39
    y: 56
    rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"

  - type: image
    file: clock-minutes.png
    x: 39
    y: 56
    rotate: "{{now.minute | mul(6)}}"

  - type: image
    file: clock-seconds.png
    x: 39
    y: 56
    rotate: "{{now.second | add(anim.frame) | mul(6)}}"

  # Kopfzeile der Abfahrtsliste
  - type: text
    value: "{{zug1.hinweis}}"
    x: 142
    y: 14
    font: regular
    size: 10
    color: "#ffffff"

  - type: text
    value: "Min."
    x: 222
    y: 14
    font: regular
    size: 10
    color: "#ffffff"

  # Gleis unter der Uhr
  - type: text
    value: "Gleis {{gleis}}"
    x: 25
    y: 105
    font: bold
    size: 17
    color: "#ffffff"
    align: center

  # --- Zug 1 (Zeile y 39) ---
  - type: image
    if: "equals(zug1.nr, 'U1')"
    file: U1.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U2')"
    file: U2.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U3')"
    file: U3.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U4')"
    file: U4.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U5')"
    file: U5.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U6')"
    file: U6.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U7')"
    file: U7.png
    x: 120
    y: 36
  - type: image
    elif: "equals(zug1.nr, 'U8')"
    file: U8.png
    x: 120
    y: 36
  - type: text
    else: true
    value: "{{zug1.nr}}"
    x: 121
    y: 39
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug1.vonnach}}"
    x: 142
    y: 39
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug1.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"
    x: 238
    y: 47
    font: regular
    size: 12
    color: "#ffffff"
    align: right

  # --- Zug 2 (Zeile y 65) ---
  - type: image
    if: "equals(zug2.nr, 'U1')"
    file: U1.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U2')"
    file: U2.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U3')"
    file: U3.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U4')"
    file: U4.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U5')"
    file: U5.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U6')"
    file: U6.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U7')"
    file: U7.png
    x: 120
    y: 62
  - type: image
    elif: "equals(zug2.nr, 'U8')"
    file: U8.png
    x: 120
    y: 62
  - type: text
    else: true
    value: "{{zug2.nr}}"
    x: 121
    y: 65
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug2.vonnach}}"
    x: 142
    y: 65
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"
    x: 238
    y: 73
    font: regular
    size: 12
    color: "#ffffff"
    align: right

  # --- Zug 3 (Zeile y 91) ---
  - type: image
    if: "equals(zug3.nr, 'U1')"
    file: U1.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U2')"
    file: U2.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U3')"
    file: U3.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U4')"
    file: U4.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U5')"
    file: U5.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U6')"
    file: U6.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U7')"
    file: U7.png
    x: 120
    y: 88
  - type: image
    elif: "equals(zug3.nr, 'U8')"
    file: U8.png
    x: 120
    y: 88
  - type: text
    else: true
    value: "{{zug3.nr}}"
    x: 121
    y: 91
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug3.vonnach}}"
    x: 142
    y: 91
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug3.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"
    x: 238
    y: 99
    font: regular
    size: 12
    color: "#ffffff"
    align: right

  # Untere Hälfte: Abfahrtsliste nach links unten …
  - type: copy
    src_x: 120
    src_y: 18
    src_width: 120
    src_height: 96
    x: 0
    y: 138

  # … Uhr und Gleis nach rechts unten
  - type: copy
    src_x: 0
    src_y: 18
    src_width: 77
    src_height: 102
    x: 162
    y: 138
```

  Hinweis zur Layer-Kette (im Code verifiziert, `renderLayers`, `default`-Zweig setzt `inChain = false`): Eine Kette endet am nächsten Layer ohne `if`/`elif`/`else`. Der `vonnach`-Text beendet sie, und Zug 2 beginnt mit neuem `if`.
- [ ] **Step 5:** `go test ./...` → PASS. Danach rendern und ansehen:
  - `go run ./cmd/zza render -t umuc-105-v1 -i templates/umuc-105-v1/default.json -o <scratch>/umuc.png --templates-dir templates` mit dem Read-Tool ansehen (zwei Uhren mit drei Zeigern, „Min." 5 / 8 / 16, Badges U1/U2/U7),
  - `… -o <scratch>/umuc.gif`: Dateigröße notieren (Richtwert ≤ 300 KB), `time` notieren (Richtwert < 2 s).
- [ ] **Step 6:** Commit `feat(templates): umuc-105-v1 clock with running second hand, legacy minutes column, cleanup`.

---

### Task 8: Dokumentation

**Files:**
- Modify: `docs/yaml-template-spec.md`, `docs/user-guide-templates.md`, `README.md`, `docs/backlog.md`
- `CLAUDE.md`: Keine Änderung nötig (Routen/Architektur unverändert). Nur prüfen.

- [ ] **Step 1: `yaml-template-spec.md`:**
  - **§1 meta:** `timezone` (Default, Validierung, Wirkung auf `now`) und `animation` als Tabelle beider Modi (Spec §7.1/§7.2 inkl. Fehlerregeln, Limits, `loop`). Den bestehenden Absatz zu `animation` so umformulieren, dass er beide Modi abdeckt.
  - **Neuer Abschnitt „Frame-Animation"** nach `type: splitflap`: Ablauf, `anim.*`-Tabelle (Spec §7.3), Beispiel Sekundenzeiger, „now ist eingefroren", PNG = Frame 0.
  - **Variablen:** `anim.*` bei den Systemvariablen. Hinweis „`now` in `meta.timezone`, Default Europe/Berlin".
  - **Filter:** Grammatik (Spec §6.1) und Argument-Regeln (Spec §6.2). `mod` und `minutes` in die Filtertabelle. Mathe-Beispiele ersetzen: Stundenzeiger mit Minutenanteil statt „grob", Sekundenzeiger mit `anim.frame`, Minutenspalte.
  - **Bedingungen:** erstes Argument als Pipeline.
  - **GIF-Ausgabe:** Loop/Delay je Modus. Cache-Hinweis: Templates mit `now`/`format` werden nicht gecacht (`X-Cache: BYPASS`).
  - **Rotation:** Drehachse exakt auf x/y.
- [ ] **Step 2: `user-guide-templates.md`:**
  - Abschnitt „Beispiel: Analoge Uhr" auf die korrekten Formeln umstellen und um den Sekundenzeiger mit Frame-Animation erweitern (vollständiges Mini-Beispiel mit `meta.animation`).
  - Abschnitt „Mathe-Filter" um Variablen-Argumente, `mod` und `minutes` (Minuten-bis-Abfahrt-Beispiel) ergänzen.
  - Kurzer Abschnitt „Zeitzone".
- [ ] **Step 3: `README.md`:**
  - API-Abschnitt: GIF-Verhalten je Modus (Splitflap einmal mit Halte-Frame, Frame-Modus mit `loop`).
  - `X-Cache: BYPASS` + `Cache-Control: no-store` für zeitabhängige Templates.
  - Beispiel-`curl` für `umuc-105-v1?format=gif`.
  - CLI-Satz „Templates ohne `splitflap`-Layer …" auf „ohne Animation" ändern.
- [ ] **Step 4: `docs/backlog.md`:**
  - neu „Stundenzeiger (24 h, ohne Minutenanteil) in `umuc-096-v1` und `nederland-096-v1` korrigieren",
  - „Frame-Animation: statische Layer nur einmal rendern (Performance)",
  - „Firmware: Loop-GIFs abspielen".
- [ ] **Step 5:** Gegenlesen: Jede in der Doku genannte Grenze und jedes Beispiel stimmt mit dem Code überein. Die drei Beispiel-Ausdrücke aus Spec §6.5 einmal per `go test` (bestehende Tests) bzw. Render verifizieren.
- [ ] **Step 6:** Commit `docs: frame animation, timezone, filter arguments, umuc clock`.

---

## Abschluss (nach Task 8)

1. Security-Review (`security-reviewer`) und Code-Review (`code-reviewer`) auf `git diff develop...feature/clock-animation` laufen lassen und Befunde beheben.
2. Prüfen: `go test ./...`, `go vet ./...`, `node --test web/static/test/*.mjs`, `CGO_ENABLED=0 go build ./cmd/zza-server`, `GOOS=windows go build ./cmd/zza-server`.
3. Manuellen Testplan `docs/superpowers/plans/2026-11-13-clock-animation-manual-tests.md` an Christoph übergeben.
4. Kein Merge, kein Push.
