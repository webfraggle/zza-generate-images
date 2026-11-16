# ZZA-Rendering auf dem Gerät — Stufe 2: C++-Kern, Exporter, ZZA-Vorschau — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ein allgemeingültiger C++-Renderer (`device/zzarender`) für das komplette Template-Format, ein Exporter, der Templates in ZZA-Pakete (ZIP) übersetzt, und eine ZZA-Vorschau im Desktop-Editor, die per cgo genau diesen C++-Code nutzt.

**Architecture:** Der C++-Kern liest ein Paket über eine Datei-Schnittstelle, rendert in eine RGB888-Leinwand und liefert für Animationen jeden Frame samt geänderten Rechtecken; aller Speicher läuft über ein Budget, Speichermangel ist ein Fehlercode, kein Absturz. Der Exporter (reines Go, CGO-frei) prüft Kompatibilität, rastert Glyphen, skaliert/komprimiert Bilder, kompiliert Regex-Muster zu Programmen und schreibt eine flache ZIP. Die Desktop-App bindet den Kern per cgo ein (`internal/devicerender`) und bietet im Editor „Server / ZZA“; der Server-Build bleibt CGO-frei.

**Tech Stack:** C++17 (Exceptions erlaubt, kein RTTI-Zwang), ArduinoJson 6.21.6 (header-only, vendored), Go 1.26 + cgo, `golang.org/x/image` (opentype, draw), `regexp/syntax`, `archive/zip`, Vanilla-JS im Editor.

**Spec:** `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md` (insb. §2 Ziel 8, §5–§9, §12). Spike-Erkenntnisse: Branch `spike/zza-render`, `spike/RESULTS.md` und `spike/zzarender/` (Wegwerf-Code, nur als Anschauung).

## Global Constraints

- **Allgemeingültig (Spec Ziel 8):** Unterstützt wird das Template-Format (`docs/yaml-template-spec.md`), nicht die heutigen Templates. Keine Regel, Einschränkung oder Optimierung anhand von Template-Namen oder „was heutige Templates nutzen“. Abgelehnt wird nur: Canvas ≠ 240 × 240, Zeichen außerhalb Latin-1 in **Konstanten**, ungültiges Regex-Muster (wie Go), Überschreitung der Limits aus §8.3/`internal/renderer`.
- Canvas fest 240 × 240 (ein Gleis; Zeilen 0–119 / 120–239 verteilt erst die Firmware).
- Zeichensatz Latin-1; dynamische Zeichen > U+00FF werden beim Zeichnen zu `?`.
- Kern ohne Arduino, TFT_eSPI, FreeRTOS; nur C++17-Standardbibliothek + ArduinoJson 6.21.6. Muss mit Apple clang (Host), mingw-w64 g++ (Windows-Cross) und xtensa-esp32s3-elf-g++ (GCC 8, `-std=gnu++17`) bauen — **keine Default-Member-Initializer in Aggregaten, die per `{…}` initialisiert werden** (Spike: GCC-8-Fehler).
- **Kein `double` im Pixelpfad** (S3-FPU nur `float`), Gewichte als Festkomma. **Keine vielen Kleinallokationen** für Tabellen (zusammenhängende Arrays). **Keine großen Objekte auf dem Stack** (> 512 B).
- Aller nennenswerte Speicher des Kerns über den Budget-Allocator (`ZzaAlloc`); Budget überschritten oder `malloc` == NULL → `std::bad_alloc` intern → API-Rückgabe `ZZA_ERR_NOMEM`. Nie `abort()`.
- Server-Build: `CGO_ENABLED=0 go build ./cmd/zza-server` muss bauen; `go list -deps ./cmd/zza-server | grep -E 'fsnotify|modelcontextprotocol|oksvg|rasterx|internal/mcp|internal/watch|internal/devicerender|internal/devpreview'` muss leer sein. `internal/devexport` (reines Go) darf der Server später importieren.
- Referenz für Semantik ist immer `internal/renderer` (Go). Bitgleichheit mit Go ist **nicht** gefordert; Vergleichsmaß („nah“): nach 4 × 4-Blockmittelung max. |Δ| ≤ 48 je Kanal **und** Anteil Pixel mit |Δ| > 8 ≤ 12 % (beide Bilder über Schwarz komponiert).
- Lokale Mac-Eigenheit (Spike): Apple-CLT-clang findet libc++ nicht von selbst. Tests mit `CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"` oder vorher `xcode-select --install` reparieren. Task 1 legt das als Doku + Make-Ziel fest.
- Commits: Conventional Commits, Ende mit `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Branch `feature/zza-on-device-rendering`.

## Review Focus

1. **Zugdaten außerhalb der Erwartung** — UTF-8 mit Zeichen > U+00FF, Emojis, ungültiges UTF-8, sehr lange Texte (≥ 100 KB), Zahlen als Strings und umgekehrt: Bild muss entstehen, `?` statt unbekannter Zeichen, Laufzeit begrenzt (`capText`). → Test in Task 2 (Evaluator) und Task 6 (Text).
2. **Speicher erschöpft mitten in der Arbeit** — Budget reicht für Laden, aber nicht für Leaf-Cache/Animation: `ZZA_ERR_NOMEM`, keine Lecks, Renderer danach weiter benutzbar (Standbild geht). → Test in Task 7 und Task 8.
3. **Pathologische Regex-Muster und lange Eingaben** — `(a*)*b`, `(a|aa)+$` auf 10 000 Zeichen: lineare Laufzeit (< 50 ms Host), gleiche Gruppen wie Go. → Test in Task 3.
4. **Kaputte Pakete** — Datei fehlt, abgeschnitten, falscher Hash, unbekannte `format`-Version, `template.json` kein JSON, Glyphentabelle mit Offsets außerhalb: Fehlercode, kein Lesen außerhalb von Puffern. → Test + Fuzz in Task 5.
5. **Funktionen in nicht genommenen Zweigen / Grenzwerte dynamischer Größen** — `splitflap` nur in einem `if`-Zweig, erster `splitflap` setzt `elif`-Kette fort (Vorspann-Optimierung muss entfallen), dynamisches `width` → 0, negativ, 10 000: Ergebnis wie Go (Fehler oder kein Bild), kein Absturz. → Test in Task 6 und Task 7.

---

## Dateistruktur

```
device/zzarender/                 C++-Kern (Master-Kopie, später nach Firmware lib/zzarender/ kopiert)
  zzarender.h                     C++-API (Renderer, Status, Time, Rect, Fs)
  zzarender_c.h / capi.cpp        C-API für cgo
  mem.h                           Budget, ZzaAlloc, ZzaBytes, ZzaVec<T>
  sha256.{h,cpp}
  utf8.{h,cpp}                    UTF-8 ↔ Latin-1, Runen, Case-Mapping
  numfmt.{h,cpp}                  Go-kompatible Zahlformatierung (%v, 'f' -1)
  eval.{h,cpp}                    Interpolation, Pipelines, Filter, Bedingungen (Port evaluator.go)
  intexpr.{h,cpp}                 Koordinaten-Ausdrücke (Port expr.go)
  regex.{h,cpp}                   Pike-VM über exportierte RE2-Programme
  package.{h,cpp}                 Manifest, Hashes, RLE, Bilder, Glyphen, Kerning laden
  canvas.{h,cpp}                  RGB888-Leinwand, Blend, Bild-Blit, Catmull-Rom, Bilinear-Rotation, copy, rect
  text.{h,cpp}                    Messen, Umbruch, fit_items, Ausrichtung, Glyphen zeichnen
  layers.{h,cpp}                  Layer-Liste, if/elif/else, loop, Limits, Vorspann-Regel
  splitflap.{h,cpp}               Module, Leaves, Zeitplan, composeFlip, shade
  anim.{h,cpp}                    Animations-API (beide Modi), Dirty-Rechtecke
  third_party/ArduinoJson.h       v6.21.6 single header (MIT, Lizenz beilegen)
internal/devexport/               Exporter (reines Go): check.go, glyphs.go, images.go, regex.go, pack.go, export.go
internal/devicerender/            cgo-Wrapper: devicerender.go, zzarender_unity.cpp, *_test.go, testdata/
internal/devpreview/              Desktop-HTTP-Handler für die ZZA-Vorschau
testdata/devtemplates/            Funktionsabdeckungs- und Worst-Case-Templates (je Ordner wie templates/)
```

---

### Task 1: Gerüst — Budget-Allocator, C-API-Hülle, cgo-Brücke, Build-Prüfungen

**Files:**
- Create: `device/zzarender/{zzarender.h,zzarender_c.h,capi.cpp,mem.h,sha256.h,sha256.cpp}`, `device/zzarender/third_party/ArduinoJson.h` + `LICENSE-ArduinoJson.txt`, `device/README.md`
- Create: `internal/devicerender/devicerender.go`, `internal/devicerender/zzarender_unity.cpp`, `internal/devicerender/devicerender_test.go`
- Modify: `CLAUDE.md` (Prüfbefehl um `internal/devicerender|internal/devpreview` erweitern; Projektstruktur `device/`, neue Pakete)

**Interfaces:**
- Produces (C, `zzarender_c.h`):
  ```c
  typedef struct zza_renderer zza_renderer;
  typedef struct { int year, month, day, hour, minute, second, weekday; } zza_time; /* weekday 0=So */
  typedef struct { int x, y, w, h; } zza_rect;
  typedef struct { const char* name; const unsigned char* data; size_t len; } zza_file;
  enum { ZZA_OK = 0, ZZA_ERR_FORMAT = 1, ZZA_ERR_TEMPLATE = 2, ZZA_ERR_NOMEM = 3, ZZA_ERR_DATA = 4, ZZA_ERR_STATE = 5 };
  zza_renderer* zza_new(size_t mem_budget);
  void          zza_delete(zza_renderer*);
  const char*   zza_version(void);              /* "zzarender/1" */
  int           zza_load(zza_renderer*, const zza_file* files, int n);          /* Task 5 */
  int           zza_render(zza_renderer*, const char* data, size_t len, const zza_time* now, unsigned char* rgb888); /* Task 6 */
  int           zza_anim_begin(zza_renderer*, const char* data, size_t len, const zza_time* now,
                               int* frames, int* loop);                         /* Task 7 */
  int           zza_anim_frame(zza_renderer*, int i, unsigned char* rgb888, int* delay_cs,
                               zza_rect* dirty, int max_rects, int* n_rects);   /* Task 7 */
  const char*   zza_error(const zza_renderer*);
  size_t        zza_mem_used(const zza_renderer*);
  size_t        zza_mem_peak(const zza_renderer*);
  ```
- Produces (C++, `mem.h`): `struct ZzaBudget { size_t limit, used, peak; void* alloc(size_t); void free(void*, size_t); };` (alloc wirft `std::bad_alloc` bei Überschreitung oder NULL), `template<class T> struct ZzaAlloc` (hält `ZzaBudget*`), `using ZzaBytes = std::vector<uint8_t, ZzaAlloc<uint8_t>>`, `template<class T> using ZzaVec = std::vector<T, ZzaAlloc<T>>`. Plattform-Hook für die Rohallokation: `extern void* (*zza_raw_malloc)(size_t); extern void (*zza_raw_free)(void*);` (Default `malloc`/`free`; Firmware setzt PSRAM).
- Produces (Go, `devicerender`):
  ```go
  type Renderer struct{ /* *C.zza_renderer */ }
  func New(memBudget int) *Renderer
  func (r *Renderer) Close()
  func Version() string
  func (r *Renderer) MemUsed() int
  func (r *Renderer) MemPeak() int
  var ErrNoMem, ErrFormat, ErrTemplate, ErrData = errors.New(...) // Fehlertext aus zza_error angehängt
  ```
  cgo: `#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/../../device/zzarender`, `zzarender_unity.cpp` `#include`t alle `device/zzarender/*.cpp` (cgo kompiliert nur Dateien im Paketordner).

- [ ] **Step 1: Failing tests** in `internal/devicerender/devicerender_test.go`:
  - `TestVersion`: `Version() == "zzarender/1"`.
  - `TestBudgetNoMem`: `r := New(1024)`; über eine Test-only-C-Funktion `zza_test_alloc(r, 4096)` (in `capi.cpp` hinter `#ifdef ZZA_TESTING`, per `#cgo CXXFLAGS: -DZZA_TESTING` nur im Test-Build… ) → Rückgabe `ZZA_ERR_NOMEM`, `MemUsed() == 0` danach, `zza_test_alloc(r, 512) == ZZA_OK`, `MemPeak() >= 512`.
  - `TestSha256`: C-Funktion `zza_test_sha256("abc")` == Go `sha256.Sum256([]byte("abc"))`, außerdem 1 MB Zufallsdaten.
  (Testhilfen: `#cgo` darf in `_test.go` nicht stehen → Testhelfer als `export_test.go`-ähnliche Datei `testhooks.go` mit Build-Tag `zzatesting` ist **nicht** nötig: die Test-C-Funktionen sind immer einkompiliert, aber nur über den nicht exportierten Go-Wrapper `testAlloc`/`testSHA256` in `testhooks.go` erreichbar.)
- [ ] **Step 2:** `go test ./internal/devicerender/` → FAIL (Paket fehlt).
- [ ] **Step 3:** Implementieren. `capi.cpp` fängt an jeder API-Grenze `std::bad_alloc` → `ZZA_ERR_NOMEM`, `std::exception` → `ZZA_ERR_TEMPLATE` mit `what()` als Fehlertext; nie eine Exception über die C-Grenze. ArduinoJson aus `.pio/libdeps/…/ArduinoJson/src` der Firmware als Single-Header vendoren (`ArduinoJson-v6.21.6.h`), Version im Kopfkommentar.
- [ ] **Step 4:** Tests grün; `CGO_ENABLED=0 go build ./cmd/zza-server` ok; Prüfbefehl aus den Global Constraints leer; `device/README.md` beschreibt Host-Build, Mac-libc++-Eigenheit und „Master-Kopie, Firmware-Kopie nie direkt bearbeiten“.
- [ ] **Step 5: Commit** `feat(device): renderer core skeleton, memory budget and cgo bridge`

### Task 2: Evaluator, Zahlformat, UTF-8 — gemeinsamer Korpus mit Go

**Files:**
- Create: `device/zzarender/{eval.h,eval.cpp,numfmt.h,numfmt.cpp,utf8.h,utf8.cpp,intexpr.h,intexpr.cpp}`
- Create: `internal/devicerender/eval_test.go`, `internal/devicerender/testdata/eval_corpus.json`
- Modify: `internal/renderer/evaluator.go` — exportierte Testhilfe `func EvalForTest(data map[string]any, now time.Time, frame, frames, delayCS int, expr string, isCond bool) string` in neuer Datei `internal/renderer/export_eval.go` (Go ist die Referenz; Funktion ruft `newEvaluatorAt(...).withAnim(...)` und `Interpolate`/`EvalCondition`).
- Modify: `internal/renderer/expr.go` — exportierte Hilfe `EvalIntExprForTest(expr string, vars map[string]int) (int, error)` (neue Datei `export_expr.go`).

**Interfaces:**
- Produces (C++): `class Eval` mit `std::string interpolate(const std::string&) const; bool cond(const std::string&) const;` Datenquelle `JsonVariantConst`, Variablen-Scopes (Loop-/Flap-Variablen, Int-Variablen für Koordinaten, `anim.*`, `now.*` aus `zza_time`); `int evalIntExpr(const std::string&, const IntVars&, bool* ok)`.
- Test-C-Funktionen: `zza_test_eval(const char* dataJson, const zza_time*, int frame, int frames, int delayCs, const char* expr, int isCond, char* out, size_t outLen)` und `zza_test_intexpr(const char* expr, const char* varsJson, int* out)`.

- [ ] **Step 1: Korpus** `testdata/eval_corpus.json`: Liste `{data, now, anim, expr, cond?}` — **nur Eingaben**, Erwartung liefert Go zur Testzeit. Mindestens: jeder Filter aus `evaluator.go` (`strip stripAll stripBetween before after upper lower trim prefix suffix format clock replace match mul div mod add sub minutes round`) mit Grenzfällen (leere Argumente, nicht-numerische Werte, Division durch 0, negative `mod`, Ergebnis mit Nachkommastellen, große Zahlen ≥ 1e21, `-0`); verschachtelte Pipelines bis Tiefe 5 (Tiefe > 4 ergibt ""); Quotes mit `|`, `,`, `}` darin; alle `now.*`/`format`-Token; `anim.frame/frames/time`; Pfade auf fehlende Keys, `null`, Bool, Zahl (JSON-Zahl 8000115 → Go `%v`), Arrays/Objekte; Bedingungen `startsWith endsWith contains isEmpty eq equals greaterThan not and or` inkl. Tiefe > 10; `upper`/`lower` auf `äöüß ÿ É` und Nicht-Latin-1 (`ẞ`, `ő`, Emoji); Koordinaten-Ausdrücke (`+ - * /`, Klammern, Variablen, Division durch 0, Überlauf) für `intexpr`.
- [ ] **Step 2: Failing test** `TestEvalCorpus`: für jeden Eintrag `want := renderer.EvalForTest(...)`, `got := testEval(...)`; `got == want`, Fehlermeldung mit Index und Ausdruck. `TestIntExprCorpus` analog gegen `EvalIntExprForTest` (Fehlerfälle: beide melden Fehler). **Review Focus 1:** `TestEvalLongInput`: Wert mit 200 KB und 10 000 Emojis durch `upper | replace('a','bb') | before('x')` → gleich wie Go, Host-Laufzeit < 100 ms.
- [ ] **Step 3:** → FAIL.
- [ ] **Step 4:** Implementieren als Port von `evaluator.go`/`expr.go`. Zahlformat: `formatF(double)` = `strconv.FormatFloat(v,'f',-1,64)`, `formatV(double)` = `fmt.Sprint(float64)` (kürzeste Ziffern, Exponent bei exp < −4 oder ≥ 21 — gegen Korpus verifizieren). `match` ruft vorerst `regexMatchGroup` aus Task 3 auf; bis dahin liefert es "" und die `match`-Einträge des Korpus stehen in einer Liste `pendingRegex`, die Task 3 leert.
- [ ] **Step 5:** Tests grün (außer `pendingRegex`, übersprungen mit `t.Skip` nur für diese Einträge).
- [ ] **Step 6: Commit** `feat(device): evaluator, number formatting and coordinate expressions with shared Go corpus`

### Task 3: Regex — RE2-Programme aus Go, Pike-VM in C++

**Files:**
- Create: `internal/devexport/regex.go`, `internal/devexport/regex_test.go`
- Create: `device/zzarender/{regex.h,regex.cpp}`, `internal/devicerender/regex_test.go`, `internal/devicerender/testdata/regex_corpus.json`

**Interfaces:**
- Produces (Go): `func CompileRegex(pattern string) (Prog, error)` — `syntax.Parse(pattern, syntax.Perl)` → `Simplify` → `syntax.Compile`; Fehler wie `regexp.Compile`. `FoldCase`-Rune-Instruktionen werden beim Export in explizite Rune-Bereiche expandiert (über `unicode.SimpleFold`), damit der Kern keine Unicode-Tabellen braucht. `type Prog struct { Start, NumCap int; Inst []Inst }`, `type Inst struct { Op uint8; Out, Arg uint32; Runes []rune }` mit JSON-Form `[op,out,arg,[r0,r1,…]]`.
- Produces (C++): `class RegexProg { bool load(JsonArrayConst); /* aus Paket */ }; std::string matchGroup(const RegexProg&, const std::string& utf8Input, int group);` — Pike-VM (Thread-Liste mit Priorität = Leftmost-First wie RE2), Eingabe als Runen (UTF-8 dekodiert, ungültige Bytes = U+FFFD wie Go), Empty-Width-Flags (`^ $ \A \z \b \B`, Zeilenanfang/-ende) wie `syntax.EmptyOpContext`.
- Eingabegrenze wie Go: `maxFilterValueLen = 64 KiB` (Eingabe länger → ""), Muster ≤ 100 Zeichen (`maxMatchPatternLen`).

- [ ] **Step 1: Failing tests**
  - `regex_corpus.json`: ≥ 150 Fälle `(pattern, input, group)`: alle RE2-Konstrukte (Klassen, negiert, POSIX `[[:alpha:]]`, Perl `\d\D\w\W\s\S`, `.` mit/ohne `(?s)`, Anker, `\b`, `{n}`, `{n,}`, `{n,m}`, nicht-gierig `*? +? ?? {n,m}?`, Alternation mit Priorität, Gruppen fangend/nicht fangend/benannt, Flags `(?i)(?m)(?s)(?U)`, Unicode-Eingaben, ungültiges UTF-8, leere Matches), Gruppe 0..n und Gruppe > Anzahl.
  - `TestRegexCorpus` (devicerender): `want := regexp.MustCompile(p).FindStringSubmatch(in)[g]` (bzw. "" ohne Match / Gruppe fehlt — exakt Go-`matchGroup`-Semantik aus `evaluator.go`), `got := testRegex(prog, in, g)`; gleich.
  - `TestRegexLinear` (**Review Focus 3**): `(a*)*b`, `(a|aa)+$`, `(x+x+)+y` auf 10 000-Zeichen-Eingaben: Ergebnis wie Go, Host < 50 ms.
  - `FuzzRegex`: zufällige Muster (Seed aus Korpus) + Eingaben; wenn Go kompiliert, gleiche Gruppe 0 und 1.
  - `TestCompileRegexRejects` (devexport): ungültige Muster (`(`, `a**`, `\8`) → Fehler genau wie `regexp.Compile`.
- [ ] **Step 2:** → FAIL.
- [ ] **Step 3:** Implementieren; `match` im Evaluator nutzt die Programme (Task 5 lädt sie aus dem Paket; für Tests übergibt `zza_test_regex` das JSON-Programm direkt). `pendingRegex` aus Task 2 leeren.
- [ ] **Step 4:** Alle Tests aus Task 2 und 3 grün.
- [ ] **Step 5: Commit** `feat(device): RE2-compatible regex via exported programs and a Pike VM`

### Task 4: Exporter — Kompatibilität, Glyphen, Bilder, ZIP

**Files:**
- Create: `internal/devexport/{export.go,check.go,glyphs.go,images.go,pack.go,rle.go}` + Tests
- Modify: `cmd/zza/main.go` — Subcommand `export`

**Interfaces:**
- Produces (Go):
  ```go
  type Package struct{ Name string; Files map[string][]byte } // flache Namen
  type Incompatible struct{ Reasons []string }                 // implements error, Text "nicht ZZA-kompatibel: …"
  func Export(templatesDir, name string) (*Package, error)
  func (p *Package) ZIP() ([]byte, error)                      // Methode 0 (stored), feste Reihenfolge, feste Zeitstempel → reproduzierbar
  func ReadZIP(b []byte) (*Package, error)                     // toleriert einen Oberordner (wie Firmware-Backup)
  const FormatVersion = 1
  ```
- Paketinhalt (Namen ≤ 12 Zeichen, flach): `manifest.json` (`{"format":1,"name":…,"canvas":[240,240],"files":[{"n":…,"s":size,"h":sha256hex}],"features":[…]}`), `t.json` (Template als JSON), `iN.img` (Bild), `fN.fnt` (Glyphen + Kerning). `t.json` ist das YAML als JSON plus Annotationen: Text-Layer `"_f":"fN"`, Image-Layer `"_i":"iN"` (vorskaliert bei konstantem `width`/`height`) und `"_io":"iM"` (Original, nur bei dynamischer Größe), Top-Level `"_re":{"<pattern>":<Prog-JSON>}` für alle `match`-Muster, die als Konstante im Template stehen.
- Bildformat `iN.img`: `u16 w, u16 h, u8 enc` (0 = roh RGBA8888, 1 = RLE), Daten. **RLE** über 4-Byte-Pixel: Steuerbyte `c < 128` → `c+1` Literalpixel folgen; `c ≥ 128` → nächstes Pixel `c−126` mal (2…129). Encoder nimmt RLE nur, wenn kleiner als roh.
- Glyphenformat `fN.fnt`: Header `"ZF1", u8 flags, i16 ascent, i16 descent, i16 height` (Pixel, `scale_y` eingerechnet), `u16 nGlyphs`, `u32 nKern`; Glyph-Einträge sortiert nach Codepunkt (`u8 cp, i32 adv26_6 natürlich, i16 bx, i16 by, u16 w, u16 h, u32 off`), Kerning sortiert (`u16 key = l<<8|r, i32 kern26_6`), Masken (A8, RLE wie oben über 1-Byte-Werte, Flag-Bit 0). Gerastert bei Subpixel 0 mit `opentype.NewFace(size, DPI 72)`; bei `scale_x`/`scale_y` ≠ 1 Maske per `xdraw.CatmullRom` skaliert, `bx/by` skaliert und gerundet (Spike-Verfahren).
- **Glyphensatz (allgemeine Regel):** Für eine Font-Kombination werden alle Zeichen aufgenommen, die in **konstanten** Texten dieser Kombination vorkommen; sobald ein Text dieser Kombination `{{` enthält (auch über Splitflap-Variablen), der volle Satz 0x20–0xFF. Fehlende Glyphe zur Laufzeit → Glyphe `?`, und `?` ist immer enthalten.
- **Kompatibilitätsprüfung** (`check.go`, sammelt alle Gründe): Template lädt mit `renderer.LoadTemplate` (gleiche Validierung wie Server); Canvas 240 × 240; konstante Texte und Reel-Einträge nur Latin-1; jedes konstante `match`-Muster kompiliert (`CompileRegex`); dynamische `match`-Muster (Muster aus `{{…}}`) → Grund „Regex-Muster muss konstant sein“ (Go erlaubt das; das Gerät kann nicht zur Laufzeit kompilieren — **einzige Format-Einschränkung, in Spec §7.3 nachtragen**); Paketgröße > 3 MB → Grund (LittleFS).

- [ ] **Step 1: Failing tests**
  - `TestExportAllShippedTemplates`: jedes Template in `templates/` exportiert ohne Fehler, ZIP enthält `manifest.json`, alle Hashes stimmen, Paket ist reproduzierbar (zweimal exportieren → bytegleich).
  - `TestIncompatibleReasons`: Test-Templates in `t.TempDir()`: Canvas 160 × 80 → Grund enthält „240 × 240“; konstanter Text `"€"` → Grund enthält „Latin-1“; `match('(')` → Grund enthält „Regex“; mehrere Fehler → alle Gründe.
  - `TestGlyphSubset`: Kombination nur mit konstantem `"Gleis"` → Glyphen genau `{G,l,e,i,s,?}`; mit `"{{gleis}}"` → 224 Glyphen.
  - `TestRLERoundTrip` (Zufallsbilder, einfarbige Flächen, Alpha-Verläufe), `TestImageScaledWhenConstant` (Layer `width: 50` → `_i` hat 50 px Breite, kein `_io`), `TestImageOriginalWhenDynamic` (`width: "{{a.end}}"` → `_io` vorhanden).
  - `TestReadZIPWithTopFolder`: ZIP mit Oberordner `x/` → gleiche Dateien.
  - CLI: `TestExportCmd` schreibt `<out>.zip`; Fehler bei inkompatibel mit Exit ≠ 0 und Gründen auf stderr.
- [ ] **Step 2:** → FAIL. **Step 3:** Implementieren. **Step 4:** grün; `CGO_ENABLED=0 go build ./internal/devexport` ok.
- [ ] **Step 5: Commit** `feat(devexport): ZZA package exporter with compatibility check`

### Task 5: Paket laden im Kern

**Files:**
- Create: `device/zzarender/{package.h,package.cpp}`, `internal/devicerender/load_test.go`
- Modify: `internal/devicerender/devicerender.go` — `func (r *Renderer) Load(p *devexport.Package) error` (übergibt Dateien als `zza_file`-Array; Daten bleiben bis `Close` gepinnt bzw. werden kopiert)

**Interfaces:**
- Produces (C++): `class Package { int load(const zza_file*, int n, ZzaBudget&, std::string& err); const Img* image(const char* id); const Font* font(const char* id); const RegexProg* regex(const std::string& pattern); JsonVariantConst tmpl() const; }` — prüft `format == 1`, Größe und SHA-256 jeder Manifest-Datei, dekodiert RLE in Budget-Speicher, validiert Glyphentabellen (Offsets + Maße innerhalb der Maskendaten, Codepunkte sortiert, Kerning sortiert) — Verstöße → `ZZA_ERR_FORMAT` mit Dateiname.

- [ ] **Step 1: Failing tests** (**Review Focus 4**): `TestLoadShipped` (alle exportierten Templates laden, `MemUsed() > 0`); `TestLoadCorrupt` — Tabelle: Datei fehlt, 1 Byte abgeschnitten, ein Byte gekippt (Hash), `format: 2`, `t.json` = `{`, Glyph-Offset > Maskengröße, RLE-Lauf über Bildende hinaus → jeweils `ErrFormat`, Fehlertext nennt die Datei; `TestLoadNoMem` — Budget = halber Bedarf → `ErrNoMem`, danach `MemUsed() == 0`; `FuzzLoad` — zufällige Mutationen eines echten Pakets (Bytes kippen, kürzen) → nie Crash (unter `go test -fuzz` mind. 60 s im manuellen Testplan).
- [ ] **Step 2:** → FAIL. **Step 3:** Implementieren. **Step 4:** grün.
- [ ] **Step 5: Commit** `feat(device): package loader with hash, RLE and table validation`

### Task 6: Layer-Rendering (Standbild) — alle Layer-Typen

**Files:**
- Create: `device/zzarender/{canvas.h,canvas.cpp,text.h,text.cpp,layers.h,layers.cpp}`
- Create: `testdata/devtemplates/` — Funktionsabdeckungs-Templates (je Ordner `template.yaml`, `default.json`, Assets; Schriften aus `templates/*/` kopiert): `cov-image` (Bilder deckend/halbtransparent/teilweise außerhalb, `width`/`height` konstant und dynamisch inkl. nur eine Seite, `rotate` 0/37,5/90/−15/Ausdruck), `cov-text` (alle `align`/`valign` inkl. `baseline` mit und ohne `height`, `max_width`-Umbruch an Leerzeichen/Bindestrich/erzwungen, `line_height`, `scale_x`, `scale_y`, beides, `fit_items`, `id`/`advance` und `<id>.width`/`<id>.end` in Folge-Koordinaten, Text außerhalb der Leinwand, Kerning-Paare), `cov-logic` (Layer-`if/elif/else`, Block-Nodes verschachtelt, `loop` mit `split_by`/`max_items`/`i`/`loop.index`, `StringOrCond` für `color`/`value`, Koordinaten-Ausdrücke, `rect` mit Alpha, `copy` überlappend nach oben/unten/teilweise außerhalb).
- Create: `internal/devicerender/render_test.go`, `internal/devicerender/compare_test.go` (Hilfe `assertNear(t, goImg, cImg)` mit dem Maß aus den Global Constraints; bei Fehler Vergleichsbild `Go | C++ | |Δ|×4` nach `t.TempDir()` und Pfad loggen)
- Modify: `internal/renderer` — exportierte Testhilfe `NewWithClock(dir string, now func() time.Time) *Renderer` (Datei `export_clock.go`).
- Modify: `internal/devicerender/devicerender.go` — `func (r *Renderer) Render(data []byte, now time.Time) (*image.NRGBA, error)` (`now` bereits in der Template-Zeitzone; RGB888 → NRGBA mit Alpha 255)

**Interfaces:**
- Produces (C++): `int Renderer::render(const char* data, size_t len, const zza_time&, uint8_t* rgb888)`; Leinwand RGB888 startet schwarz (Go: transparent → Vergleich über Schwarz komponiert). Limits wie `internal/renderer` (`maxLayers`, `maxBlockDepth`, `maxTextRunes`, `maxCanvasDimension`, Loop `max_items` Default 20). Fehler, die Go als Render-Fehler meldet (unbekannter Font, ungültige Farbe, ungültiges `rotate`), → `ZZA_ERR_TEMPLATE` mit Text.
- Algorithmen: Blend `round(v/255)` ohne Division (`(v+128 + ((v+128)>>8))>>8`); Bild-Blit mit Zeilenzeigern und Fast-Path α=0/255; Catmull-Rom-Skalierung zur Laufzeit (nur dynamische Größe, Ergebnis pro (Bild, w, h) im Budget gecacht, max. 4 Einträge LRU); Rotation bilinear wie `rotateImageCW` (Bounding-Box-Parität, Mittelpunkt), `float` + schrittweise Koordinaten + Festkomma-Gewichte (Spike-Variante); `copy` zeilenweise `memmove`, Reihenfolge nach Überlappung; Text: Glyphen bei Subpixel 0, Stiftposition `round(pen/64)` bzw. `round(pen·scale_x/64)`, Kerning per binärer Suche, Messen/Umbruch/`fit_items`/Ausrichtung als Port von `renderText`, `wrapText`, `layoutFitItems`, `visibleRun`-Äquivalent (Glyphen außerhalb der Leinwand überspringen).

- [ ] **Step 1: Failing tests**
  - `TestRenderShippedNear`: alle Templates in `templates/` mit ihrem `default.json` und fester Zeit 2026-01-01 10:08:37: `assertNear(goRender, cRender)`.
  - `TestRenderCoverageNear`: alle `testdata/devtemplates/cov-*` ebenso, plus je 3 Datensätze (`default.json`, leere Felder, sehr lange Felder).
  - `TestRenderErrorsLikeGo`: Templates, die in Go einen Render-Fehler liefern (ungültige Farbe, `rotate: "abc"`, unbekannte Font-ID) → C++ liefert `ErrTemplate`.
  - **Review Focus 1:** `TestRenderHostileData`: Zugdaten mit Emojis, `ẞ`, ungültigem UTF-8 und 1 MB-Feld in einem Text-Layer mit `max_width` → kein Fehler, Host-Laufzeit < 200 ms, an Stelle nicht-Latin-1-Zeichen wird `?` gezeichnet (Pixelvergleich gegen dasselbe Template mit `?` im Datensatz: identisch).
  - **Review Focus 5:** `TestDynamicSizeEdge`: `width` ergibt 0, −5, 10 000 → gleiches Verhalten wie Go (Fehler ↔ `ErrTemplate`, kein Bild ↔ unverändertes Bild), kein Absturz.
- [ ] **Step 2:** → FAIL. **Step 3:** Implementieren. **Step 4:** grün.
- [ ] **Step 5: Commit** `feat(device): render all layer types (still image)`

### Task 7: Animation — Split-Flap und Frame-Modus, Dirty-Rechtecke, Vorspann

**Files:**
- Create: `device/zzarender/{splitflap.h,splitflap.cpp,anim.h,anim.cpp}`
- Create: `testdata/devtemplates/cov-flap` (`split: chars` mit/ohne Reel, ganze Tafeln, `min_flips`/`max_flips`/`flip_frames` 1…6, Wert nicht im Reel, leerer Reel, Kind-Layer mit `if`/Bildern, Overlay und `copy` nach den Modulen, `splitflap` in einem `if`-Zweig), `cov-flap-elif` (erster `splitflap` setzt eine `elif`-Kette fort), `cov-frames` (Frame-Modus mit `anim.frame` in `rotate`, Koordinaten, `value`; `loop: true/false`; `frame_delay` 0,02 und 10)
- Create: `internal/devicerender/anim_test.go`
- Modify: `internal/devicerender/devicerender.go` — `func (r *Renderer) RenderAnimation(data []byte, now time.Time) (*renderer.Animation, error)` (sammelt alle Frames; `FPS`/`Delay`/`Loop` wie Go) und `func (r *Renderer) AnimFrames(data []byte, now time.Time, fn func(i int, img *image.NRGBA, delayCS int, dirty []image.Rectangle) error) error`

**Interfaces:**
- Produces (C): `zza_anim_begin` — berechnet den Ablauf (Split-Flap: Seed = SHA-256(Templatename ‖ 0x00 ‖ Datenbytes) wie `AnimationSeed`, Zeitplan wie `RenderAnimation`; Frame-Modus: `frames`, `loop`), `*frames` = Anzahl inkl. Endbild (Split-Flap) bzw. `animation.frames`; Template ohne Animation → `*frames = 1`. `zza_anim_frame(i)` rendert Frame `i` in den übergebenen Puffer, der **den vorigen Frame enthalten muss** (Frame 0: beliebig), liefert `delay_cs` (Split-Flap `round(100/fps)`, letzter Frame 100; Frame-Modus `frame_delay`) und bis zu `max_rects` geänderte Rechtecke (zusammengefasst; mehr Änderungen → ein umfassendes Rechteck); Reihenfolge der Aufrufe frei (Firmware rendert vor oder live).
- **Vorspann-Regel (allgemein):** Im Split-Flap-Modus werden Top-Level-Layer vor dem ersten Layer, der einen `splitflap` enthält (auch verschachtelt), einmal gerendert und pro Frame zurückkopiert — außer der erste solche Layer ist `elif`/`else` (dann keine Optimierung). Begründung: `now` eingefroren, `anim.*` konstant.
- Leaf-Cache und Vorspann im Budget; `ZZA_ERR_NOMEM` während `anim_frame` lässt den Renderer benutzbar (Cache wird verworfen; ein späteres `zza_render` funktioniert).

- [ ] **Step 1: Failing tests**
  - `TestFlapFramesNear`: `templates/faltblatt-105-v1` und `cov-flap`: gleiche Frame-Anzahl wie Go `RenderAnimation` (Seed aus denselben Bytes), jeder Frame `assertNear`, letzter Frame `assertNear` mit Standbild.
  - `TestFlapDeterministic`: zweimal gleiche Eingabe → bytegleiche Frames (C++).
  - `TestFrameModeNear`: Templates mit `meta.animation.frames` (`templates/umuc-105-v1`, `oebb-105-v1`, `nederland-105-v1`, `cov-frames`): gleiche Anzahl, Delay, Loop wie Go; Frames 0, 1, N/2, N−1 `assertNear`.
  - `TestDirtyRects`: für jeden Frame i > 0 gilt: Pixel außerhalb der gemeldeten Rechtecke sind gleich dem Frame i−1 (aus derselben C++-Folge).
  - **Review Focus 5:** `TestPrefixRuleElif`: `cov-flap-elif` → Frames `assertNear` mit Go (Optimierung darf das Ergebnis nicht ändern); `cov-flap` mit `splitflap` nur im nicht genommenen Zweig → 1 Frame wie Go.
  - **Review Focus 2:** `TestAnimNoMem`: Budget = Lade-Bedarf + 50 KB → `zza_anim_begin` oder ein `zza_anim_frame` liefert `ErrNoMem`; danach `Render` mit denselben Daten ok und `assertNear` mit Go; `MemUsed()` zurück auf Lade-Niveau.
- [ ] **Step 2:** → FAIL. **Step 3:** Implementieren (`composeFlip`: Bilinear-Stauchung mit Festkomma; `shade` integer wie Go). **Step 4:** grün.
- [ ] **Step 5: Commit** `feat(device): split-flap and frame animations with dirty rectangles`

### Task 8: Generierte und Worst-Case-Templates

**Files:**
- Create: `internal/devicerender/gen_test.go` (Template-Generator), `testdata/devtemplates/worst-*` (`worst-layers`: Layer-Limit ausgeschöpft; `worst-flap`: 64 Module, 64 Reel-Einträge, `flip_frames: 6`, `duration: 5`, `fps: 25`; `worst-frames`: 125 Frames, Vollbild-`rotate`; `worst-text`: 20 Text-Layer mit großem Schriftgrad und vollem Glyphensatz; `worst-images`: 6 Vollbild-Bilder mit Alpha)

**Interfaces:**
- Consumes: alles aus Task 1–7.
- Generator: `func genTemplate(rng *rand.Rand, assets string) (yaml string, data string)` — erzeugt gültige Templates aus den Bausteinen der Spec (zufällige Layer-Typen, Verschachtelung bis 3, Ausdrücke aus einer Liste von Pipeline-Bausteinen inkl. `match`, Bedingungen, Koordinaten-Ausdrücke, optional Split-Flap oder Frame-Modus), Assets aus `testdata/devtemplates/cov-*`.

- [ ] **Step 1: Tests**
  - `TestGeneratedNear`: 300 Templates (Seeds 1…300, fest): wenn Go rendert, rendert C++ `assertNear`; wenn Go mit Fehler ablehnt (Validierung/Render), liefert Export oder C++ ebenfalls einen Fehler. Bei Abweichung: YAML + Daten + Vergleichsbild nach `t.TempDir()`, Seed im Fehlertext.
  - `TestWorstCaseBudget`: jedes `worst-*`: Export ok oder nachvollziehbar abgelehnt (Größe), Laden + Standbild + vollständige Animation mit Budget 6 MB → ok; `MemPeak()` geloggt (Werte gehen in den manuellen Testplan / Stufe 3); mit Budget 1 MB → `ErrNoMem` ohne Absturz, Standbild danach mit großem Budget ok.
  - `FuzzEvalData`: zufällige Zugdaten-JSONs gegen `templates/*` → kein Absturz, Fehler nur `ErrData` bei ungültigem JSON.
- [ ] **Step 2:** Abweichungen beheben (im Kern, nicht im Generator, außer der Generator erzeugt nachweislich etwas Spec-Widriges).
- [ ] **Step 3: Commit** `test(device): generated and worst-case templates against the Go renderer`

### Task 9: ZZA-Vorschau im Desktop-Editor + CLI

**Files:**
- Create: `internal/devpreview/handler.go`, `internal/devpreview/handler_test.go`
- Modify: `internal/server/server.go` — `RegisterEditor(h *editor.FSHandlers, extra ...EditorRoute)` mit `type EditorRoute struct{ Pattern string; Handler http.Handler }` (Server importiert `devpreview` **nicht**)
- Modify: `cmd/zza/main.go` (`buildHandler`: `devpreview.New(tdir)` als `EditorRoute` registrieren; `export`-Subcommand aus Task 4 nutzt `devexport`)
- Modify: `web/templates/edit-editor.html` (Umschalter), ggf. `web/static/app.css`
- Modify: `docs/user-guide-templates.md` (Abschnitt „ZZA-Vorschau“), `CLAUDE.md` (Route, Pakete)

**Interfaces:**
- Produces (HTTP, nur Desktop): `POST /edit/{template}/zza-render[?format=gif]` — Body = Zugdaten-JSON (wie `/{template}/render`, gleiche Größengrenzen). Ablauf: `devexport.Export` (bei jedem Aufruf neu, kein Cache in v1) → `devicerender.New(16 MB)` → `Load` → `Render` bzw. `RenderAnimation` → PNG bzw. GIF über `renderer.EncodeGIF` (`canvas.colors` des Templates). `now` = aktuelle Zeit in der Template-Zeitzone. Antworten: 200 Bild; 422 `text/plain` mit „nicht ZZA-kompatibel: …“ (alle Gründe, je Zeile) bzw. Kern-Fehlertext; 400 bei ungültigem JSON; Template-Name über `renderer.ValidateTemplateName`.
- Editor-UI: neben „Standbild/Animation“ ein `<select id="preview-engine">` mit „Server“ / „ZZA“ (Standard Server; Auswahl in `localStorage` je Template, Zugriff in try/catch). Bei „ZZA“ geht `renderPreview()` an `zza-render`; Fehlertext erscheint im vorhandenen `#render-error`.

- [ ] **Step 1: Failing tests** `handler_test.go`: 200 + `image/png` für `faltblatt-105-v1`; `?format=gif` → `image/gif` mit > 1 Frame; inkompatibles Test-Template → 422 mit „240 × 240“ im Text; ungültiger Template-Name → 400/404 wie die übrigen Editor-Routen; Body > Limit → 413.
- [ ] **Step 2:** → FAIL. **Step 3:** Implementieren. **Step 4:** grün; Server-Prüfbefehl leer; `go test ./...` grün (mit Mac-`CGO_CXXFLAGS`).
- [ ] **Step 5: Commit** `feat(editor): ZZA preview rendered by the device core`

### Task 10: Build und Doku

**Files:**
- Modify: `build.sh` (Desktop-Builds kompilieren den Kern mit; Windows: `CXX=x86_64-w64-mingw32-g++` bereits gesetzt; macOS amd64 Cross von arm64 mit clang `-arch x86_64` prüfen), `README.md`, `docs/yaml-template-spec.md` (Abschnitt „ZZA-kompatibel“: Canvas, Latin-1, konstante Regex-Muster, Paketgrößen-Grenze), Spec §7.3 (konstante Regex-Muster nachtragen)
- Create: `docs/superpowers/plans/2026-11-16-zza-on-device-stage2-manual-tests.md`

- [ ] **Step 1:** `./build.sh` ohne `DOCKER_PUSH` baut alle drei Desktop-ZIPs; in jedem startet `zza export -t faltblatt-105-v1 -o /tmp/x.zip` bzw. die ZZA-Vorschau (manuell, Testplan).
- [ ] **Step 2:** Manueller Testplan: ZZA-Vorschau Standbild/Animation je mitgeliefertem Template, Inkompatibilitätsmeldung, `zza export` + Paket in ZIP-Tool öffnen, `go test -fuzz=FuzzLoad -fuzztime=60s`, `FuzzRegex`, `FuzzEvalData`, Windows-Build starten (ZZA-Vorschau unter Windows), Größe der Desktop-Binaries vorher/nachher.
- [ ] **Step 3:** Gesamtprüfung: `go test -race ./...`, `go vet ./...`, `node --test 'web/static/test/*.test.mjs'`, `CGO_ENABLED=0 go build ./cmd/zza-server`, Server-Prüfbefehl leer.
- [ ] **Step 4: Commit** `docs: ZZA compatibility, export and preview; build with device core`

---

**Nicht in Stufe 2 (Stufe 3, eigener Plan):** Sync-Script in die Firmware, Firmware-Adapter (eigener Render-Task, PSRAM-Hook, Verteilung auf D1/D2, Push, DMA), Animations-Strategie nach Spec §12.4 (live messen → live, wenn ≤ 70 % des Takts, sonst vorrendern aus dem festen Pool-Anteil des Gleises; endliche Animationen geben die Leihe nach dem Abspielen zurück, Schleifen beim nächsten Wechsel) mit Rückfall-Kette bis zum Endbild, Upload-Seite und `templateCommit`/`templateDelete`, Auswahl pro Gleis, Komplett-Backup, E2E auf DUAL und QUAD. Der Kern liefert dafür in Stufe 2 bereits: frei wählbare Frame-Reihenfolge, Dirty-Rechtecke, Budget und `ZZA_ERR_NOMEM` als regulären Rückgabewert.
