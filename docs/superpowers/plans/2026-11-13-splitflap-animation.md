# Split-Flap-Animation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Templates können Klappenmodule (`type: splitflap`) beschreiben; der Renderer liefert daraus ein Standbild (PNG, wie bisher) oder eine deterministische Faltblatt-Animation als GIF. `faltblatt-105-v1` wird damit layout-korrigiert und animiert.

**Architecture:** `splitflap` rendert pro Modul deckende „Blätter" (Leinwand-Schnappschuss + Kind-Layer) in Offscreen-Bilder. Standbild = Zielblatt. Für die Animation wird pro Frame die ganze Layer-Liste neu durchlaufen (so wirken `copy` und Overlay automatisch); ein reiner, getesteter Zeitplan (`flipCount`/`flipSequence`) bestimmt je Modul und Frame, welche zwei Blätter in welcher Phase zusammengesetzt werden. Ein eigener GIF-Encoder (globale Palette, Delta-Rechtecke, kein Loop) sitzt im renderer-Paket und wird von Server und CLI genutzt.

**Tech Stack:** Go 1.26, stdlib `image`, `image/gif`, `golang.org/x/image` (vorhanden), `gopkg.in/yaml.v3`. Keine neuen Abhängigkeiten.

**Spec:** `docs/superpowers/specs/2026-11-13-splitflap-animation-design.md` — Umsetzer lesen Spec **und** ihren Task.

## Global Constraints

- Branch `feature/splitflap-animation`. Kein Push, kein Merge, kein `./build.sh` (würde `VERSION` erhöhen).
- Server-Build bleibt CGO-frei; keine neue Dependency in `go.mod`.
- `POST /{template}/render` ohne `format` liefert PNG, bitgleich wie vorher für alle Templates außer `faltblatt-105-v1`. Text-Layer ohne `scale_x`/`fit_items`/`valign: baseline` laufen durch den unveränderten Zeichenpfad.
- Gleiche Eingabe ⇒ bytegleiches GIF (kein `math/rand` ohne Seed, keine Map-Iteration in ausgaberelevanter Reihenfolge, keine Zeit).
- Sicherheitsregeln bleiben: Dateinamen ohne Pfad, `file` nie interpoliert, bestehende Limits. Neue Limits: max. 125 Frames, max. 64 Module pro Template, max. 64 `reel`-Einträge pro Layer, fps 5–25, duration 0.5–5 s.
- Fehlertexte im Stil des Pakets (`renderer: …`, englisch); UI-Texte deutsch.
- Jeder Task: TDD, `go test ./...` grün, `go vet ./...` sauber, ein Commit mit Trailer `Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>`.

## Review Focus

1. **Zielwert mit Mehrbyte-Zeichen / länger als `cells`** („MÜNCHEN HBF TIEF", „Zürich") — Runen statt Bytes, sauber abgeschnitten. → Task 2.
2. **Leere oder fehlende Felder** (`zug1.via` leer, `zeit` ohne „:", `nr` leer, `vonnach` leer) — Standbild und GIF rendern ohne Fehler, Module landen auf leerem Blatt. → Task 2, 3, 6.
3. **Modul ragt über den Leinwandrand oder hat Größe 0** — Template-Fehler mit klarer Meldung, kein Panic. → Task 2.
4. **GIF-Anforderung für Template ohne `splitflap`** und für Templates mit `canvas.colors` — gültiges 1-Frame-GIF, Palette ≤ `colors`. → Task 4, 5.
5. **Zwei aufeinanderfolgende identische Frames / gar keine Änderung** — Encoder erzeugt gültiges GIF (kein leeres Rechteck). → Task 4.

---

### Task 1: Text-Erweiterungen und Filter

**Files:**
- Modify: `internal/renderer/template.go` (Layer-Felder), `internal/renderer/renderer.go` (`renderText`), `internal/renderer/evaluator.go` (`applyFilter`)
- Test: `internal/renderer/text_ext_test.go` (neu), `internal/renderer/evaluator_test.go`

**Interfaces:**
- Produces (YAML/Layer): `Layer.ScaleX float64 \`yaml:"scale_x"\``; `Layer.FitItems *FitItems \`yaml:"fit_items"\`` mit `type FitItems struct { SplitBy string \`yaml:"split_by"\`; Gap int \`yaml:"gap"\` }`; `valign: baseline`; Filter `before('x')`, `after('x')`.

- [ ] **Step 1: Failing tests** (Font für Tests: `../../templates/faltblatt-105-v1/NimbusSanL-Bol.otf` über ein In-Memory-`Template` mit `Dir` auf dieses Verzeichnis; Helfer `inkBounds(img) image.Rectangle` = Bounding-Box aller Pixel mit Alpha > 0 auf transparenter Leinwand):
  - `TestText_ValignBaseline`: Text „H", size 20, `y: 30`, `valign: baseline` → `inkBounds.Max.Y` ∈ {30, 31} und `Min.Y` < 20. Derselbe Layer ohne `valign` → `Min.Y` ≥ 30.
  - `TestText_ScaleX`: „HHHH" size 20 einmal ohne, einmal mit `scale_x: 0.5` → Breite der Ink-Box mit Skalierung = Hälfte (±1 px), Höhe gleich (±1 px).
  - `TestText_ScaleX_AlignRight`: `align: right`, `x: 100`, `scale_x: 0.5` → `inkBounds.Max.X` ≤ 100 und ≥ 97.
  - `TestText_NoNewFields_Unchanged`: Layer ohne neue Felder rendert pixelidentisch zu einer vor der Änderung erzeugten Erwartung — umgesetzt als: Rendern über `renderText` und über eine im Test eingefrorene Kopie des alten Algorithmus (font.Drawer direkt) ergibt identische `Pix`.
  - `TestText_FitItems`: Wert „Neuhof - Gelnhausen - Hanau", `fit_items: {split_by: "-", gap: 2}`, `scale_x: 0.5`, size 12.67, `x: 0`, `max_width: 87` → mit Hilfsfunktion `fitItems(...)` (siehe Step 3) gezeichnete Elemente = `["Neuhof", "Gelnhausen"]`. Zweiter Fall: erstes Element zu breit (`max_width: 5`) → nichts gezeichnet, kein Fehler. Dritter Fall: mittleres Element zu breit, späteres passt nicht mehr, weil `xpos` trotzdem weiterrückt.
  - `TestFilter_BeforeAfter`: `{{t | before(':')}}` mit „13:30" → „13"; `after(':')` → „30"; ohne Trenner: `before` → ganzer Wert, `after` → „"; leerer Wert → „".
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'Text_|Filter_BeforeAfter'` → FAIL.
- [ ] **Step 3: Implementieren.**
  - `valign: baseline`: Grundlinie der ersten Zeile = `y` (unabhängig von `height`).
  - `scale_x` (nur wenn > 0 und ≠ 1): Zeile in ein transparentes Offscreen-NRGBA natürlicher Größe zeichnen, horizontal mit `xdraw.CatmullRom` auf `ceil(Breite·scale_x)` skalieren, mit `draw.Over` setzen. Messbreite für `align` und `fit_items` = `ceil(natürliche Breite · scale_x)`.
  - `fit_items`: einzeilig, linksbündig; reine Funktion `fitItems(widths []int, gap, maxWidth int) []bool` (true = zeichnen) mit Legacy-Regel `xpos+width < maxWidth` (strikt), `xpos += width+gap` immer; Elemente werden vor dem Messen getrimmt, leere übersprungen.
  - Filter `before`/`after` in `applyFilter`, Argument über vorhandenes `parseFilterCall`.
- [ ] **Step 4:** `go test ./internal/renderer/` → PASS.
- [ ] **Step 5:** Commit `feat(renderer): text baseline, scale_x, fit_items; before/after filters`.

---

### Task 2: `splitflap` als Standbild

**Files:**
- Create: `internal/renderer/splitflap.go`, `internal/renderer/splitflap_test.go`
- Modify: `internal/renderer/template.go` (Felder), `internal/renderer/renderer.go` (Dispatch, Render-Kontext)

**Interfaces:**
- Produces:
  - Layer-Felder: `Cells int \`yaml:"cells"\``, `Pitch int \`yaml:"pitch"\``, `Split string \`yaml:"split"\`` (`""`|`"chars"`), `Reel Reel \`yaml:"reel"\`` (`type Reel []string`, YAML-String ⇒ eine Rune pro Eintrag, YAML-Liste ⇒ Einträge), `SplitY int \`yaml:"split_y"\``, `FlipFrames int \`yaml:"flip_frames"\``, `MinFlips int \`yaml:"min_flips"\``, `MaxFlips int \`yaml:"max_flips"\``. `Var`, `Value`, `Layers`, `X/Y/Width/Height` werden wiederverwendet.
  - `func (t *Template) HasSplitflap() bool` (rekursiv durch Block-Nodes).
  - `type flapModule struct { layerIdx, cellIdx int; rect image.Rectangle; splitY int; target string; layer *Layer }`
  - `func (r *Renderer) renderLeaf(rc *renderCtx, m flapModule, bg *image.NRGBA, value string) (*image.NRGBA, error)` — deckendes Blatt in Modulgröße: Kopie von `bg`, darauf Kind-Layer mit `var = value`, Koordinaten relativ zum Modul, auf das Modul geclippt.
  - `renderCtx`: neuer interner Träger für Render-Zustand, der `renderLayers` durchgereicht wird (statt weiterer Einzelparameter): mindestens `tmpl *Template`, `layerCount *int`, `flapLayerSeq int` (laufende Nummer der `splitflap`-Layer in Durchlaufreihenfolge), `imgCache map[string]image.Image` (dekodierte Bilder, pro Render), `anim *animState` (nil = Standbild; Typ kommt in Task 3, hier als `type animState struct{}` anlegen). Bestehende Signatur von `Render` bleibt.

- [ ] **Step 1: Failing tests** (In-Memory-Templates mit `rect`-Kindern, Pixelprüfung wie in `renderer_test.go`):
  - `TestSplitflap_Static_Chars`: Leinwand 60×10, Hintergrund-`rect` blau; `splitflap` x 0, y 0, width 8, height 10, cells 4, pitch 15, `split: chars`, value „AB", var `ch`, Kind: Block `if: "eq(ch, 'A')"` → rotes `rect` 0,0,8,10; `elif: "eq(ch, 'B')"` → grünes. Erwartung: Pixel (4,5) rot, (19,5) grün, (34,5) und (49,5) blau, Zwischenraum (10,5) blau.
  - `TestSplitflap_Static_RelativeAndClipped`: Kind-`rect` x −5, y 2, width 100, height 3 in Modul x 20, width 8 → nur x 20..27 gefärbt, (19,3) und (28,3) nicht.
  - `TestSplitflap_Chars_Runes`: value „ÄÖÜß€" mit cells 3 → Module bekommen „Ä", „Ö", „Ü" (über Test-Hook `planModules`, siehe Step 3).
  - `TestSplitflap_Whole`: ohne `split`, value „ic23" → ein Modul, Kind-Bedingung `startsWith(nr,'ic')` greift.
  - `TestSplitflap_EmptyValue`: value „" → alle Module rendern (Kind sieht `""`), kein Fehler.
  - `TestSplitflap_Validation`: je ein Template → Fehler enthält erwarteten Text: width 0; Modul über Leinwandrand; `cells` 65; Reel mit 65 Einträgen; zwei überlappende `splitflap`-Layer; `splitflap` in `loop`; `loop`/`copy`/`splitflap` als Kind; `split_y` ≥ height; 9 Layer à 8 Module (> 64 gesamt).
  - `TestReel_Unmarshal`: YAML `reel: " AB"` → `[" ", "A", "B"]`; `reel: ["rb", "", "ice"]` → drei Einträge.
  - `TestHasSplitflap`: true für Layer in Block-Node, false für `sbb-096-v1` (echtes Template laden).
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'Splitflap|Reel|HasSplitflap'` → FAIL.
- [ ] **Step 3: Implementieren.**
  - `planModules(layer *Layer, layerIdx int, eval *Evaluator, canvas image.Rectangle) ([]flapModule, error)`: löst Geometrie und Zielwerte auf (bei `split: chars` per `[]rune`, Rest leer `""`; Leerzeichen-Zielwert bleibt `" "`), validiert. Defaults: `cells` 1, `pitch` = width, `split_y` = height/2.
  - Standbild-Pfad: pro Modul Schnappschuss der Leinwand unter `rect` → `renderLeaf(…, target)` → mit `draw.Src` zurücksetzen.
  - Kind-Layer zeichnen über `renderLayers` auf das Offscreen-Blatt; Kinder zählen gegen `maxLayers` nur im Standbild-Durchlauf einmal pro Modul.
  - Überlappungsprüfung über alle Module eines Renders (in `renderCtx` sammeln).
  - `imgCache` in `renderImage` nutzen (Schlüssel: Dateiname; nur dekodiertes Original, Skalierung/Rotation wie bisher).
- [ ] **Step 4:** `go test ./...` → PASS (bestehende Server-Tests gegen echte Templates inklusive).
- [ ] **Step 5:** Commit `feat(renderer): splitflap layer type (static rendering)`.

---

### Task 3: Animations-Zeitplan und Frames

**Files:**
- Create: `internal/renderer/animation.go`, `internal/renderer/animation_test.go`
- Modify: `internal/renderer/template.go` (`Meta.Animation`), `internal/renderer/splitflap.go`, `internal/renderer/renderer.go`

**Interfaces:**
- Consumes: `flapModule`, `renderLeaf`, `renderCtx`, `planModules` aus Task 2.
- Produces:
  - `type AnimationMeta struct { FPS int \`yaml:"fps"\`; Duration float64 \`yaml:"duration"\` }`, Feld `Meta.Animation AnimationMeta \`yaml:"animation"\``. Defaults 20 / 2.5; außerhalb 5–25 bzw. 0.5–5 ⇒ Fehler in `LoadTemplate` (0 = Default).
  - `type Animation struct { Frames []*image.NRGBA; FPS int }`
  - `func (r *Renderer) RenderAnimation(tmpl *Template, data map[string]interface{}, seed []byte) (*Animation, error)` — Template ohne `splitflap` ⇒ genau 1 Frame (= `Render`).
  - `func AnimationSeed(templateName string, body []byte) []byte` — `sha256(templateName ‖ 0x00 ‖ body)`.
  - `func flipCount(seed []byte, layerIdx, cellIdx, minFlips, maxFlips int) int` — deterministisch in `[minFlips, maxFlips]` (sha256 über seed‖layerIdx‖cellIdx, modulo).
  - `func flipSequence(reel []string, target string, k int) []string` — Länge `k+1`, letztes Element = `target`.
  - `func composeFlip(dst *image.NRGBA, at image.Point, from, to *image.NRGBA, splitY int, phase, phases int)` — zeichnet den Zwischenzustand eines Umklappens; `phase` ∈ `[0, phases)`.

- [ ] **Step 1: Failing tests:**
  - `TestFlipSequence_TargetInReel`: reel `[" ","A","B","C","D"]`, target „D", k 3 → `["A","B","C","D"]`. k 7 (Umlauf) → `["B","C","D"," ","A","B","C","D"]`.
  - `TestFlipSequence_TargetNotInReel`: reel `["x","y"]`, target „Neuhof", k 3 → Länge 4, letztes „Neuhof", die ersten drei aus dem Reel in zyklischer Reihenfolge, aufeinanderfolgende Einträge verschieden.
  - `TestFlipSequence_NoReel`: reel leer, target „A", k 2 → `["", "", "A"]`.
  - `TestFlipSequence_SingleEntryReelEqualsTarget`: reel `["A"]`, target „A", k 2 → Länge 3, endet auf „A" (kein Panic, keine Endlosschleife).
  - `TestFlipCount_DeterministicAndBounded`: 1000 Kombinationen aus (layerIdx, cellIdx) → alle in `[4, 20]`; gleiche Eingabe gleiches Ergebnis; anderer Seed ⇒ in mindestens 50 % der Fälle anderer Wert; mindestens 5 verschiedene Werte über 13 Zellen (Stopps verteilen sich).
  - `TestComposeFlip_Phases`: `from` = komplett rot 10×10, `to` = komplett grün, `splitY` 5, 2 Phasen. Phase 0: oberste Zeile (y 0) grün, Zeile direkt über dem Scharnier (y 4) rötlich (R > G), gesamte untere Hälfte rot. Phase 1: gesamte obere Hälfte grün, Zeile direkt unter dem Scharnier (y 5) grünlich, unterste Zeile (y 9) rot.
  - `TestRenderAnimation_Basics`: Template aus Task-2-Test (`TestSplitflap_Static_Chars`, mit `reel: " AB"`), seed fest → `len(Frames)` ≥ 2 und ≤ 125; letztes Frame `Pix`-identisch zu `Render`; Frame 0 ≠ letztes Frame; zweimaliger Aufruf ⇒ alle Frames identisch; anderer Seed ⇒ mindestens ein Frame verschieden.
  - `TestRenderAnimation_StaggeredStops`: 13 Zellen; pro Zelle erstes Frame bestimmen, ab dem der Modulbereich bis zum Ende unverändert bleibt → mindestens 4 verschiedene Stopp-Frames.
  - `TestRenderAnimation_FrameZeroInMotion`: jede Zelle unterscheidet sich in Frame 0 vom Endzustand (auch Zellen mit leerem Ziel, sofern `reel` nicht leer).
  - `TestRenderAnimation_CopyFollows`: Template mit `splitflap` in der oberen Hälfte und `copy` nach unten → in jedem Frame ist die Kopie pixelgleich zur Quelle.
  - `TestRenderAnimation_NoSplitflap`: echtes Template `sbb-096-v1` mit dessen `default.json` → 1 Frame.
  - `TestRenderAnimation_Limits`: `duration: 5`, `fps: 25`, `flip_frames: 1`, `max_flips: 500` → Fehler oder ≤ 125 Frames (Erwartung: `max_flips` wird auf das in `duration` Passende gekappt, kein Fehler); `fps: 60` → `LoadTemplate`-Fehler.
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'Flip|RenderAnimation'` → FAIL.
- [ ] **Step 3: Implementieren.**
  - Zeitplan je Modul: `f = flip_frames` (Default 2, 1–6), `maxK = floor(duration·fps / f)`, `min_flips` Default 4, `max_flips` Default `maxK`; beide auf `[1, maxK]` begrenzt; `k = flipCount(...)`. Modul animiert in Frames `0 … k·f−1`, steht ab Frame `k·f`. Frameanzahl = `max(k·f) + 1`.
  - `flipSequence`: Ziel im Reel ⇒ rückwärts ab Zielposition `k` Schritte zyklisch; nicht im Reel ⇒ `k` Reel-Einträge zyklisch ab einem aus dem Seed abgeleiteten Start, dann Ziel; Reel leer ⇒ `""`.
  - `composeFlip` (Spec §6): Phase der oberen Hälfte — `to`-Oberteil voll, darüber `from`-Oberteil vertikal gestaucht (Höhe = Oberteil·(1−p)), an der Scharnierlinie verankert, abgedunkelt; Phase der unteren Hälfte — `to`-Oberteil voll, `from`-Unterteil voll, darüber `to`-Unterteil gestaucht (Höhe = Unterteil·p), ab Scharnier nach unten, leicht abgedunkelt. Bei 2 Phasen p = 0.5 je Hälfte; bei mehr Phasen gleichmäßig. Abdunklung als benannte Konstanten (`flapShadeUpper = 0.25`, `flapShadeLower = 0.15`), Stauchung mit `xdraw.ApproxBiLinear`.
  - Frame-Rendern: pro Frame frische Leinwand, `renderLayers` mit `rc.anim` (Frame-Index, Zeitplan, Blatt-Cache `map[leafKey]*image.NRGBA` mit `leafKey{layerIdx, cellIdx, value}`); Blätter werden nur für Werte der jeweiligen Sequenz erzeugt. Ein einziger `Evaluator` (ein `now`) für alle Frames.
- [ ] **Step 4:** `go test ./internal/renderer/` → PASS.
- [ ] **Step 5:** Commit `feat(renderer): deterministic split-flap animation frames`.

---

### Task 4: GIF-Encoder

**Files:**
- Create: `internal/renderer/gifenc.go`, `internal/renderer/gifenc_test.go`

**Interfaces:**
- Consumes: `Animation` (Task 3), `Quantize` (`quantize.go`).
- Produces: `func EncodeGIF(w io.Writer, anim *Animation, colors int) error` — `colors` 0 ⇒ 256, sonst 2–256.

- [ ] **Step 1: Failing tests** (Dekodieren mit `gif.DecodeAll`):
  - `TestEncodeGIF_Structure`: 3 Frames 20×20 (einfarbig, im mittleren ein 4×4-Quadrat bei (5,5), im letzten bei (10,10)), FPS 20 → 3 Bilder; `LoopCount == -1`; `Delay == [5, 5, 100]`; `Config.Width/Height == 20`; Frame 1 `Bounds()` = `(5,5)-(9,9)`; Frame 2 `Bounds()` = `(5,5)-(14,14)`; alle Frames nutzen dieselbe Palette wie `Config.ColorModel`.
  - `TestEncodeGIF_Reconstructs`: Frames mit ≤ 16 Farben; dekodieren, Frames der Reihe nach übereinanderlegen → jedes rekonstruierte Vollbild pixelgleich zum Eingabe-Frame.
  - `TestEncodeGIF_IdenticalFrames`: 3 identische Frames → gültiges GIF; rekonstruiertes Endbild korrekt; Summe der Delays = 5+5+100 (aufeinanderfolgende identische Frames werden zusammengelegt: 1 Bild mit Delay 110 ist zulässig — der Test prüft nur Summe und Endbild).
  - `TestEncodeGIF_SingleFrame`: 1 Frame → 1 Bild, dekodierbar auch mit `gif.Decode`.
  - `TestEncodeGIF_Colors`: `colors: 4` → `len(palette)` ≤ 4.
  - `TestEncodeGIF_Deterministic`: zweimal kodieren → identische Bytes.
  - `TestEncodeGIF_Empty`: `anim` ohne Frames oder FPS 0 → Fehler.
- [ ] **Step 2:** `go test ./internal/renderer/ -run EncodeGIF` → FAIL.
- [ ] **Step 3: Implementieren.** Globale Palette: Stichproben-Mosaik aus erstem, letztem und bis zu 6 gleichmäßig verteilten Zwischen-Frames nebeneinander → `Quantize(mosaik, colors)` → dessen Palette. Alle Frames ohne Dithering auf diese Palette abbilden (pro Farbe gecachter Nearest-Lookup). Ab Frame 1 nur das Begrenzungsrechteck der gegenüber dem vorigen Palettenbild geänderten Pixel als `SubImage`, `Disposal: gif.DisposalNone`. Delay = `round(100/FPS)`, letztes Frame 100. `LoopCount: -1`.
- [ ] **Step 4:** `go test ./internal/renderer/` → PASS.
- [ ] **Step 5:** Commit `feat(renderer): GIF encoder with global palette and delta frames`.

---

### Task 5: Server-Format, Cache, CLI

**Files:**
- Modify: `internal/server/server.go`, `internal/server/cache.go`, `internal/cli/render.go`
- Test: `internal/server/server_test.go`, `internal/server/cache_test.go`, `internal/cli/render_test.go` (anlegen, falls nicht vorhanden)

**Interfaces:**
- Consumes: `RenderAnimation`, `AnimationSeed`, `EncodeGIF`, `Template.HasSplitflap`.
- Produces:
  - HTTP: Query `format` = `png` (Default) | `gif` auf `POST /{template}/render` und `GET /{template}/preview`.
  - `func (c *Cache) GetExt(key, ext string) ([]byte, bool)`, `func (c *Cache) SetExt(key, ext string, data []byte) error`; `Get`/`Set` delegieren mit `"png"`. Cleanup, `Stats`, `Flush` erfassen `.gif` mit.
  - `detailData.Animated bool` (für Task 7), gesetzt aus `tmpl.HasSplitflap()`; dasselbe Flag im View-Model der Editor-Seite, falls diese ein eigenes hat.
  - CLI: Flag `--format png|gif` (Default: aus Endung von `-o`, sonst png).

- [ ] **Step 1: Failing tests.** Für GIF-Tests ein Test-Template in einem `t.TempDir()`-Templates-Verzeichnis anlegen (kleines `splitflap`-Template mit `rect`-Kindern + `default.json`), Server darauf starten:
  - `TestRender_FormatGIF`: `POST /t/render?format=gif` → 200, `Content-Type: image/gif`, Body beginnt mit `GIF89a`, `gif.DecodeAll` liefert > 1 Frame, `X-Cache: MISS`; zweiter Aufruf `HIT` mit identischen Bytes.
  - `TestRender_DefaultStillPNG`: derselbe Body ohne `format` und mit `format=png` → `image/png`, PNG-Magic; Bytes beider Antworten identisch; ein vorheriger GIF-Aufruf verändert sie nicht.
  - `TestRender_FormatInvalid`: `format=webp` → 400.
  - `TestPreview_FormatGIF`: `GET /t/preview?format=gif` → `image/gif`.
  - `TestRender_GIF_NoSplitflap`: echtes Template `sbb-096-v1` mit `format=gif` → 200, genau 1 Frame.
  - `TestRender_GIF_HTTPNoRedirect`: `POST …/render?format=gif` mit `X-Forwarded-Proto: http` → kein 301.
  - `TestRender_GIF_Deterministic`: zwei Server mit getrennten Cache-Verzeichnissen → identische GIF-Bytes.
  - `TestCache_Ext`: `SetExt(k,"gif",…)` und `Set(k,…)` koexistieren mit unterschiedlichem Inhalt; `Flush` entfernt beide; Cleanup nach Alter entfernt `.gif`.
  - `TestCache_KeyUnchanged`: `Key("a", []byte("b"))` = fest eingetragener Hex-Wert, der vor der Änderung ermittelt wurde.
  - CLI: `render -t t -i in.json -o out.gif` schreibt Datei mit `GIF89a`; `-o out.png` PNG; `-o out.png --format gif` GIF; `--format bmp` Fehler.
- [ ] **Step 2:** `go test ./internal/server/ ./internal/cli/` → FAIL.
- [ ] **Step 3: Implementieren.** `renderAndServe(w, templateName, body, format string)`; Format-Validierung in den Handlern vor dem Rendern. PNG-Zweig unverändert lassen (gleicher Cache-Schlüssel, gleiche Encode-Schritte). GIF-Zweig: gleicher Schlüssel, Endung `gif`; `seed = AnimationSeed(templateName, body)`; `colors = tmpl.Meta.Canvas.Colors`. Gleichzeitige GIF-Renders über einen Semaphor (Kapazität 2) begrenzen; Warten bricht ab, wenn der Request-Kontext endet (dann 503). Kommentar über `renderAndServe` und CLI-Hilfetexte anpassen.
- [ ] **Step 4:** `go test ./...` → PASS; `CGO_ENABLED=0 go build ./cmd/zza-server` → ok.
- [ ] **Step 5:** Commit `feat(server,cli): format=gif output with cache support`.

---

### Task 6: `faltblatt-105-v1` — Layout nach Referenz und Animation

**Files:**
- Modify: `templates/faltblatt-105-v1/template.yaml`
- Create: `templates/faltblatt-105-v1/ice.png` (Kopie von `legacy/faltblatt-105-v1/img/ice.png`)
- Create: `internal/renderer/faltblatt_test.go`
- Create: `internal/renderer/contactsheet_test.go` (Hilfstest, schreibt nur bei gesetzter Env-Variable)

**Interfaces:**
- Consumes: alles aus Task 1–4.
- Produces: fertiges Template; Kontaktbogen-Werkzeug `ZZA_CONTACT_SHEET=<pfad.png> go test ./internal/renderer/ -run TestContactSheet` (alle Frames von `faltblatt-105-v1` mit Default-Daten, 4× vergrößert ohne Glättung, Raster 6 Spalten, Frame-Nummer als Dateiname-unabhängige Reihenfolge links→rechts, oben→unten) sowie `ZZA_STILL=<pfad.png>` für das Standbild und `ZZA_GIF=<pfad.gif>` für das GIF.

Vorgaben (aus Spec §5/§6 und `legacy/faltblatt-105-v1/index.php`). **Schriftgrößen = Legacy × 16/9** (das PHP multipliziert mit 96/72, und GD rechnet Punktgrößen selbst noch einmal mit 96 DPI um; gegen die Referenz geprüft: Via-Breite, Versalhöhe der Zielzeile, Höhe der Gleisziffer). Maßgeblich ist bei Abweichungen immer das Referenzbild:

| Element | Legacy-Aufruf | Neu |
|---|---|---|
| „Gleis" | size 12, x 27, y 34, center, xFactor 0.5 | text, size 21.33, `valign: baseline`, `scale_x: 0.5`, statisch |
| Gleisnummer | size 33, x 27, y 90, center, 0.5 | text, size 58.67, baseline, `scale_x: 0.5`, statisch |
| „Abfahrt" | size 10.5, x 57, y 35, left, 0.5 | konstanter Kind-Layer der Stunden-Klappe, size 18.67, `scale_x: 0.5` |
| Stunde | size 12.5, x 119, y 37, right, xFactor 1 | Kind-Layer, size 22.22, ohne `scale_x`; Wert `{{zug1.zeit \| before(':')}}`; Reel „00"…„23" |
| Minute | size 8.5, x 133, y 32, center, 1 | eigene Klappe, size 15.11; Wert `after(':')`; Reel „00","05",…,"55" |
| Zugart | Bild bei (55,53), ice vor ic, case-insensitiv | Klappe, Wert `{{zug1.nr \| lower}}`, Reel `["rb","re","ic","ice",""]`, Kinder if/elif `startsWith` in Reihenfolge ice, ic, rb, re |
| Via | size 9.5, x 147, y 68, maxX +87, Split „-", gap 2, 0.5 | Klappe mit einem text-Kind: size 16.89, `scale_x: 0.5`, `fit_items: {split_by: "-", gap: 2}`, `max_width: 87`; Reel: 3 Füll-Viatexte + `""` |
| Ziel | 13 Zeichen, Raster 14, Mitte xpos+5, y 105, size 12, 0.5, upper | Klappe `cells: 13`, `pitch: 14`, `split: chars`, Reel `" ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ-."`, size 21.33, `scale_x: 0.5` |
| `fg.png` | nach allen Texten | direkt vor den drei `copy`-Layern |

Modulrechtecke = ganze Klappen aus `bg.png`, `split_y` auf den Scharnierstrichen aus `fg.png` — **ausmessen** (Pixel lesen), nicht schätzen. Ungefähre Lage: Stunde x 54–121 / y 15–43, Minute x 124–142, Zugart x 54–141 / y 48–76, Via x 145–233 / y 48–76, Zielzellen ab x 54, y 82–111, Scharniere bei y ≈ 29 / 63 / 96. Kind-Koordinaten = Legacy-Koordinate − Modulursprung. `meta.animation` nicht setzen (Defaults); Zielzeile `flip_frames: 2`, große Tafeln `flip_frames: 3` und `max_flips: 8`.

- [ ] **Step 1: Failing tests** (`faltblatt_test.go`, echtes Template + `default.json`, Referenz `../../docs/superpowers/specs/assets/2026-11-13-faltblatt-referenz.png`):
  - `TestFaltblatt_MatchesReference`: Hilfsfunktion `darkBounds(img, rect)` = Bounding-Box der Pixel mit Luminanz < 110 innerhalb `rect`. Für die Regionen der oberen Hälfte — Gleis-Block (x 3–52), Stunden-Klappe, Minuten-Klappe, Via-Klappe, jede der 13 Zielzellen — weichen die vier Kanten von Render und Referenz um höchstens 1 px ab (Zielzellen 10–13: beide leer). Regionen als Tabelle im Test.
  - `TestFaltblatt_ZugartIcon`: Region der Zugart-Klappe: mittlere absolute Differenz der RGB-Werte zwischen Render und Referenz < 12 (IC-Logo an gleicher Stelle); mit `nr: "ICE 71"` unterscheidet sich die Region vom IC-Render; mit `nr: "ic 5"` (klein) gleich dem IC-Render.
  - `TestFaltblatt_BottomHalf`: untere Hälfte entspricht den drei Legacy-Kopien (Pixelvergleich Quelle/Ziel im gerenderten Bild).
  - `TestFaltblatt_OverlayOnTop`: an einem Scharnierstrich-Pixel innerhalb eines Buchstabens der Zielzeile (Koordinate aus `fg.png` ermitteln, Alpha > 200) hat das Render die Farbe des Overlays, nicht die Schriftfarbe.
  - `TestFaltblatt_EdgeData`: Tabelle — `via: ""`; `zeit: "7"`; `zeit: ""`; `nr: ""`; `vonnach: ""`; `vonnach: "München Hbf tief Gleis"`; `gleis: ""`; nur `{}` → `Render` und `RenderAnimation` ohne Fehler, letztes Frame = Standbild.
  - `TestFaltblatt_Animation`: Default-Daten → Frames ≥ 30 und ≤ 61; letztes Frame = `Render`; Gleis-Block (x 3–52, y 10–118) in allen Frames identisch; kodiertes GIF ≤ 400 KB (Größe per `t.Logf` ausgeben); `RenderAnimation`+`EncodeGIF` < 1 s (bei `testing.Short()` überspringen, Dauer loggen).
- [ ] **Step 2:** `go test ./internal/renderer/ -run Faltblatt` → FAIL.
- [ ] **Step 3:** Template umbauen, `ice.png` kopieren, Kontaktbogen-Hilfstest schreiben.
- [ ] **Step 4:** `go test ./...` → PASS. Standbild, Kontaktbogen und GIF in das im Auftrag genannte Scratch-Verzeichnis schreiben und die Pfade plus gemessene GIF-Größe/Renderzeit im Abschlussbericht nennen.
- [ ] **Step 5:** Commit `feat(templates): faltblatt-105-v1 layout per legacy reference, split-flap animation`.

*Nach diesem Task prüft der Planer Standbild und Kontaktbogen visuell und gibt ggf. Nachbesserungen (Tuning von `composeFlip`, Timing, Reels) in Auftrag, bevor Task 7 startet.*

---

### Task 7: Web-Oberfläche

**Files:**
- Modify: `web/templates/detail.html`, `web/templates/edit-editor.html`, ggf. `web/static/*.css`, Handler für die Editor-Seite (View-Model-Flag)
- Test: `internal/server/server_test.go`, ggf. `internal/editor/*_test.go`

**Interfaces:**
- Consumes: `detailData.Animated`, `?format=gif` (Task 5).

- [ ] **Step 1: Failing tests:** `TestDetail_AnimatedControls`: Detailseite eines `splitflap`-Templates enthält `id="play-anim-btn"` und `id="download-gif-btn"`; Detailseite von `sbb-096-v1` enthält beide nicht. Analog Editor-Seite: `id="preview-mode"` nur bei animiertem Template — falls das Flag dort serverseitig nicht verfügbar ist, Umschalter immer rendern und Test entsprechend auf Vorhandensein beschränken.
- [ ] **Step 2:** Test läuft rot.
- [ ] **Step 3: Implementieren.**
  - Detailseite: Standard bleibt Standbild in den zwei Canvas-Hälften. „Animation abspielen" (`play-anim-btn`) rendert die aktuellen JSON-Daten mit `?format=gif` und zeigt das GIF in derselben Zwei-Hälften-Geometrie (zwei Elemente mit demselben Blob-URL, obere/untere Hälfte per CSS beschnitten); erneuter Klick lädt neu (neuer Blob-URL ⇒ Neustart). JSON-Änderung schaltet zurück aufs Standbild. `download-gif-btn` speichert `<name>.gif`; der bestehende Button bleibt PNG. Blob-URLs freigeben.
  - Editor: Auswahl `preview-mode` (Standbild | Animation) neben der Vorschau; bei „Animation" ruft `renderPreview()` `?format=gif` ab; Knopf „Erneut abspielen"; Größenanzeige bleibt.
  - Galerie unverändert.
- [ ] **Step 4:** `go test ./...` → PASS; vorhandene JS-Tests (`node --test web/static/test/*.mjs`) → PASS. Server lokal starten (`go run ./cmd/zza-server serve` mit `PORT=18080 CACHE_DIR=<scratch>/cache`), mit `curl` prüfen, dass Detailseite und `preview?format=gif` 200 liefern; Server beenden.
- [ ] **Step 5:** Commit `feat(web): animation preview and GIF download`.

---

### Task 8: Dokumentation und manueller Testplan

**Files:**
- Modify: `docs/yaml-template-spec.md`, `docs/user-guide-templates.md`, `README.md`, `docs/backlog.md`, `CLAUDE.md` (Routen-Zeile um `?format=gif` ergänzen)
- Create: `docs/superpowers/plans/2026-11-13-splitflap-animation-manual-tests.md`

- [ ] **Step 1:** `yaml-template-spec.md`: Abschnitte `splitflap` (alle Felder, Regeln, Limits, Beispiel Zielzeile + Zugart aus dem echten Template), `meta.animation`, `valign: baseline`, `scale_x`, `fit_items`, Filter `before`/`after`; die falsche Aussage korrigieren, `size` akzeptiere `{{…}}`. `user-guide-templates.md`: kurze Anleitung „Faltblatt-Animation bauen" (Modul = ganze Klappe, `split_y` aufs Scharnier, Overlay nach den Klappen, `now` und Determinismus). `README.md`: API `?format=gif`, CLI `--format`. `backlog.md`: „Animierte GIFs" als erledigt; neu aufnehmen: „Firmware: GIF/MJPEG-Wiedergabe", „weitere Faltblatt-Templates animieren".
- [ ] **Step 2:** Manueller Testplan: Galerie/Detail/Editor im Browser (Abspielen, Neustart, Download), GIF in Vorschau-App einmalig ohne Loop, Vergleich Standbild mit `https://zza.yuv.de/i/`, Display-Regression (PNG-Render eines anderen Templates unverändert), Desktop-App.
- [ ] **Step 3:** Commit `docs: split-flap animation (template spec, guide, README, manual tests)`.

---

## Abschluss (nach Task 8)

Security-Review (`security-reviewer`) und Code-Review (`code-reviewer`) auf `git diff develop...feature/splitflap-animation`; Befunde beheben; `go test ./...`, `go vet ./...`, `CGO_ENABLED=0 go build ./cmd/zza-server`. Kein Merge, kein Push — Übergabe an Christoph auf dem Branch.
