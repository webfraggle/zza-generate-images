# device/ — On-Device-Renderer

`device/zzarender/` ist der C++-Kern (C++17, nur ArduinoJson 6.21.6), der auf dem
ESP32-S3-Controller läuft und im Desktop-Build per cgo (`internal/devicerender`)
für die Vorschau eingebunden wird.

**Dies ist die Master-Kopie. Die Firmware-Kopie nie direkt bearbeiten** — Änderungen
passieren hier und werden in die Firmware übernommen.

## Aufbau

- `zzarender_c.h` — C-API (Grenze zur Firmware / zu Go; keine Exceptions darüber hinaus)
- `zzarender.h`, `capi.cpp` — interner Zustand, API-Hülle (`bad_alloc` → `ZZA_ERR_NOMEM`)
- `mem.h` — Budget-Allocator (`ZzaBudget`, `ZzaAlloc`); Hooks `zza_raw_malloc`/`zza_raw_free` für PSRAM
- `work.h` — Rechenaufwand je Aufruf (`zza_set_work_limit`), `RenderError`, `WorkLimitError`
- `package.{h,cpp}` — Paket laden und prüfen (Manifest, SHA-256, `.img`, `.fnt`, `t.json`, `_re`)
- `jsoncheck.{h,cpp}` — Zugdaten-JSON vorab wie Go beurteilen (Syntax, Tiefe, float64-Bereich)
- `decnum.h` — beschränkte kanonische Form langer Zahlen (für `jsoncheck`, `numfmt` und den
  ZZA PATCH in `third_party/ArduinoJson.h`, der `../decnum.h` inkludiert)
- `eval.{h,cpp}` — Evaluator: `{{…}}`, Filter, Bedingungen (Port von `evaluator.go`)
- `regex.{h,cpp}` — Pike-VM für vom Exporter kompilierte RE2-Programme (`match`)
- `intexpr.{h,cpp}` — Koordinaten-Ausdrücke
- `numfmt.{h,cpp}` — Zahlen wie Go formatieren und parsen
- `utf8.{h,cpp}` — UTF-8, Latin-1, Groß-/Kleinschreibung (`casetab.h`)
- `sha256.{h,cpp}` — SHA-256
- `canvas.{h,cpp}` — RGB888-Leinwand: Blend, Bild-Blit, Catmull-Rom-Skalierung, Bilinear-Rotation, copy, rect
- `text.{h,cpp}` — Messen, Umbruch, fit_items, Ausrichtung, Glyphen zeichnen
- `layers.{h,cpp}` — Layer-Liste, if/elif/else, loop, Limits, Standbild (`zza::Renderer`)
- `splitflap.{h,cpp}` — splitflap-Layer, Module, Flip-Zeichnung
- `anim.{h,cpp}` — Animation (`zza_anim_begin`/`zza_anim_frame`): Split-Flap, Frame-Modus, Präfix, Dirty-Rects
- `render_impl.h` — gemeinsamer interner Zustand von `layers`, `splitflap` und `anim`
- `third_party/ArduinoJson.h` — ArduinoJson 6.21.6 als Single-Header (Lizenz: `LICENSE-ArduinoJson.txt`);
  der ZZA-PATCH-Block oben legt die Layout-Einstellungen fest (u. a. 4-Byte-Slot-Offsets) —
  abweichende Definitionen aus dem Build sind ein Fehler

Die `zza_test_*`-Funktionen werden nur mit `-DZZA_TEST_HOOKS` übersetzt (setzt
`internal/devicerender` in seinen cgo-Flags); die Firmware übersetzt ohne.

Der Kern braucht **C++-Exceptions** (`-fexceptions`; Budget-Fehler und Render-Fehler
laufen über Exceptions, die die C-API in Statuscodes übersetzt). Ohne bricht der Build
mit `#error` ab (`mem.h`). Auf dem ESP32 also Exceptions eingeschaltet lassen bzw.
einschalten (ESP-IDF: `CONFIG_COMPILER_CXX_EXCEPTIONS=y`; sonst `-fexceptions`).

## Host-Build / Tests

Der Kern wird vom Go-Paket `internal/devicerender` kompiliert (`zzarender_unity.cpp`
inkludiert alle `device/zzarender/*.cpp`, da cgo nur Dateien im Paketordner übersetzt):

```sh
go test ./internal/devicerender/
```

## Firmware-Kopie synchronisieren

Die Firmware (`Zugzielanzeiger-tpl`) trägt eine Kopie unter `lib/zzarender/`, die nie
direkt bearbeitet wird:

```sh
tools/sync-zzarender.sh <firmware-repo>          # kopiert nach <fw>/lib/zzarender/, löscht Veraltetes
tools/sync-zzarender.sh --check <firmware-repo>  # Exit 0 = Kopie gleich, sonst Exit 1 mit Liste
```

Kopiert werden `device/zzarender/*.{h,cpp}` und `third_party/` (nicht `tools/`).
`SOURCE.txt` enthält `commit <hash>` (mit `-dirty` bei uncommitteten Änderungen in
`device/`), `date <ISO>` und je Datei `<sha256>  <pfad>`. Test: `TestSyncScript`
(nur Temp-Ordner).

### ArduinoJson-Namespace

Der Kern legt sein ArduinoJson in den Namespace `ArduinoJson::ArduinoJson6216_ZZA`
(`ARDUINOJSON_VERSION_NAMESPACE` im ZZA-PATCH-Block von `third_party/ArduinoJson.h`).
Grund: Die Firmware kann eine zweite ArduinoJson-Kopie (`lib_deps`) für ihren eigenen
Code behalten; ohne eigenen Namespace gäbe es doppelte Symbole (ODR) mit
unterschiedlichem Layout (4-Byte-Slot-Offsets, `strtod`-Patch). Der Kern-Code
schreibt weiter `ArduinoJson::`, der Namespace ist `inline`.

### C-API: Uhrzeit und Animation

Die vollständige Beschreibung steht in `zzarender_c.h`. Zur Zeit gilt:

- `int zza_anim_frame(zza_renderer*, int i, const zza_time* now, unsigned char* rgb888, int* delay_cs, zza_rect* dirty, int max_rects, int* n_rects)`
  rendert Frame *i* in `rgb888` (das muss Frame *i*−1 enthalten). `now == NULL`: Der Kern rechnet
  selbst wie Go: im Frame-Modus Startzeit von `zza_anim_begin` + `floor(i * delay_cs / 100)` Sekunden
  mit Übertrag ins Datum; Split-Flap und Standbild (`frames == 1`) nutzen die Startzeit. `now != NULL`:
  diese Zeit gilt für den Frame (Frame-Modus und Standbild); Split-Flap ignoriert sie. Go-Brücke,
  ZZA-Vorschau und MCP-Gerätecheck rufen mit `NULL`; die Firmware gibt die Geräte-Uhr hinein.
- `int zza_clock(const zza_renderer*)` liefert aus dem Manifest-Feld `clock` (`"none"`, `"minute"`,
  `"second"`) `ZZA_CLOCK_NONE`, `ZZA_CLOCK_MINUTE` oder `ZZA_CLOCK_SECOND`: wie oft ein Standbild
  aufgefrischt werden muss. Ohne Paket oder ohne Feld: `ZZA_CLOCK_NONE`; `zza_load` weist andere Werte
  mit `ZZA_ERR_FORMAT` ab.
- `int zza_render_diff(zza_renderer*, const char* data, size_t len, const zza_time* now, unsigned char* rgb888, zza_rect* dirty, int max_rects, int* n_rects)`
  wie `zza_render`, aber `rgb888` enthält das vorige Bild und wird an Ort und Stelle aktualisiert; die
  geänderten Rechtecke werden wie bei `zza_anim_frame` gemeldet (0 = unverändert; mehr als `max_rects`:
  ein umschließendes Rechteck). `ZZA_ERR_STATE`, solange eine Animation offen ist. Der Vergleichspuffer
  (172800 Byte) belegt nur während des Aufrufs Budget-Speicher; bei einem Fehler bleibt `rgb888` unverändert.

Siehe Spec `docs/superpowers/specs/2026-11-20-template-clock-design.md`.

### Vorschau wie das Gerät: `zza render --zza`

```sh
zza render -t <name> -i data.json -o out.png --zza [--time 2026-01-01T10:08:37]
```

Rendert über Exporter und Kern (nur PNG, bitgleich zum Gerät; nicht kompatible
Templates enden mit „nicht ZZA-kompatibel: …“ und Exit ≠ 0). `--time` fixiert die
Uhr für beide Renderer (ohne Zonenangabe in der Zeitzone des Templates).

### macOS: libc++-Eigenheit

Die Apple Command Line Tools finden die libc++-Header nicht von selbst. Vor jedem
cgo-Build/-Test setzen:

```sh
export CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"
```

(Auch für direkte `clang++`-Aufrufe die gleichen Flags verwenden.)

### Compiler-Vorgaben

Muss mit Apple clang, mingw-w64 g++ und GCC 8 (xtensa, gnu++17) übersetzen: keine
Default-Member-Initializer in Aggregaten, die mit `{…}` initialisiert werden.
