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
- `sha256.{h,cpp}` — SHA-256
- `third_party/ArduinoJson.h` — ArduinoJson 6.21.6 als Single-Header (Lizenz: `LICENSE-ArduinoJson.txt`)

Die `zza_test_*`-Funktionen sind immer einkompiliert und nur für Tests gedacht.

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
