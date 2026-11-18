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
