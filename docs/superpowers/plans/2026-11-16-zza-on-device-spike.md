# ZZA-Rendering auf dem Gerät — Stufe 1 (Wegwerf-Prototyp) Implementation Plan

> **For agentic workers:** Wegwerf-Prototyp laut Spec §10 Stufe 1: **ohne TDD-Pflicht, ohne Reviews**. Steps use checkbox (`- [ ]`) syntax for tracking. Code wird nicht gemergt; nur die Messwerte und Erkenntnisse gehen als Nachtrag in die Spec.

**Goal:** Klären, ob ein C++-Renderer `faltblatt-105-v1` (inkl. Split-Flap-Animation) und `umuc-105-v1` (Uhr mit `rotate`, Frame-Schleife) auf dem ESP32-S3 N16R8 schnell genug und mit vertretbarem Speicher rendert → Go/No-Go plus Messwerte.

**Architecture:** Ein Wegwerf-Exporter in Go erzeugt ein Paketverzeichnis (JSON + RGBA-Bilder + vorgerasterte Glyphen). Ein minimaler C++17-Renderer (nur die Funktionen der zwei Templates) rendert es auf dem Mac (Vergleich mit dem Go-Renderer, Host-Zeiten) und — als Kopie im Firmware-Spike-Branch — auf dem Gerät (Zeiten per Serial).

**Tech Stack:** Go (vorhandener `internal/renderer`, `golang.org/x/image`), C++17, ArduinoJson 6.21.6 (aus `.pio/libdeps` der Firmware), PlatformIO-Umgebung `ZZA_Universal_Display_Controller_105_DUAL_Debug`.

**Spec:** `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md`

## Global Constraints

- Generator-Repo: Branch `spike/zza-render` (von `feature/zza-on-device-rendering`), Code unter `spike/` — wird nie gemergt.
- Firmware-Repo: Branch `spike/template-render` von `develop`; die uncommitteten Änderungen auf `feature/animated-gif` bleiben unangetastet (vorher prüfen: `git status`, ggf. Worktree statt Checkout).
- Canvas 240 × 240 = ein Gleis; Zeilen 0–119 → `sprite2` (D2), 120–239 → `sprite1` (D1), wie `DisplayPng`.
- Zeichen: Latin-1 0x20–0xFF; andere → `?`.
- `now`: zufällige Uhrzeit (Spec §6.3).
- Paket-Inhalt laut Spec §7.1, aber ohne ZIP/Manifest/Hashes (Spike): Verzeichnis `<name>/` mit `template.json`, `img/*.rgba`, `font/*.bin`.

## Review Focus

Entfällt für den Spike (keine Reviews). Beobachten und im Ergebnis notieren:
- Abweichung C++ ↔ Go-Renderer (Text mit `scale_x`, gedrehte Zeiger) — sichtbar, nicht bitgenau gefordert.
- Renderzeit Standbild, Frame-Zeit Split-Flap und Uhr, Heap/PSRAM, Paketgröße, Flash-/RAM-Belegung der Firmware.

---

### Task 1: Wegwerf-Exporter

**Files:** Create `spike/export/main.go`

- [ ] Aufruf: `go run ./spike/export -templates templates -t <name> -o spike/out/<name>`.
- [ ] `template.json`: YAML generisch nach JSON (Ausdrücke bleiben Strings). Jeder `text`-Layer bekommt `"_font": "<id>"` für seine Kombination (font, size, scale_x, scale_y).
- [ ] Bilder: jede im Template referenzierte Datei als `img/<datei>.rgba`: `u16 w, u16 h` (LE), dann RGBA8888 (nicht vormultipliziert). Konstante `width`/`height` → vorab mit `xdraw.CatmullRom` skalieren (in den zwei Templates nicht genutzt, nur Fehler bei dynamischen Werten).
- [ ] Glyphen: `font/<id>.bin` pro Kombination: Header `i16 ascent, i16 descent, i16 height` (px, gerundet, `scale_y` eingerechnet), dann 224 Einträge (0x20–0xFF) `i32 advance26_6 (× scale_x), i16 bx, i16 by, u16 w, u16 h, u32 off`, danach 8-Bit-Alpha-Masken. Masken mit `face.Glyph` bei Subpixel 0 gerastert, bei `scale_x`/`scale_y` ≠ 1 per `xdraw.CatmullRom` skaliert. Kerning: weglassen, aber Anzahl der Kern-Paare ≠ 0 je Kombination auf stdout ausgeben.
- [ ] Ausgabe: Dateigrößen je Teil und gesamt auf stdout.
- [ ] Prüfen: `go run ./spike/export … -t faltblatt-105-v1` und `-t umuc-105-v1` laufen, Verzeichnisse entstehen.
- [ ] Commit.

### Task 2: Minimaler C++-Renderer

**Files:** Create `spike/zzarender/zzarender.h`, `spike/zzarender/zzarender.cpp`

- [ ] Schnittstelle:
  ```cpp
  struct ZzaFs { virtual bool read(const char* path, std::vector<uint8_t>& out) = 0; };
  struct ZzaTime { int hour, minute, second; };
  class ZzaRenderer {
   public:
    bool load(ZzaFs& fs, const char* dir);             // template.json + Bilder + Glyphen
    bool render(const char* dataJson, ZzaTime now, int animFrame, uint8_t* rgb888);  // 240*240*3
    int  frames() const;  int frameDelayCs() const;     // Frame-Modus, sonst 0
    // Split-Flap: Zeitplan + Frames
    bool beginFlap(const char* dataJson, ZzaTime now, uint32_t seed);
    bool flapFrame(int i, uint8_t* rgb888, int& dirtyX, int& dirtyY, int& dirtyW, int& dirtyH);
    int  flapFrames() const;
    const char* error() const;
  };
  ```
- [ ] Evaluator-Teilmenge: Pfade (`zug1.zeit`), `now.hour|hour12|minute|second`, `anim.frame`; Filter `before after lower upper trim minutes sub add mul div mod`; Bedingungen `startsWith equals`; Verschachtelung wie Go (`evaluator.go`).
- [ ] Layer: `image` (+ `rotate` bilinear wie `rotateImageCW`), `rect`, `text` (`align`, `valign: baseline|top`, `max_width` + `fit_items`, Umbruch), `copy`, `if/elif/else`-Blöcke, `splitflap` (Module, `split: chars`, Reel, Blatt-Schnappschuss, Kinder relativ zum Modul).
- [ ] Split-Flap-Animation nach `animation.go` (`flipCount`, `flipSequence`, `flipPhase`, `composeFlip`, `shade`); Seed als Zahl übergeben (Spike, kein SHA-256).
- [ ] Commit.

### Task 3: Host-Vergleich

**Files:** Create `spike/host/main.cpp`, `spike/host/build.sh`, `spike/compare/main.go`

- [ ] `spike/host/build.sh`: `c++ -std=c++17 -O2 -I<ArduinoJson/src> …` → `spike/out/zzahost`.
- [ ] `zzahost <paketdir> <data.json> <out.ppm> [frame]` rendert und gibt Renderzeit (µs) aus; mit `flap` rendert es alle Split-Flap-Frames und gibt Zeit/Frame und Dirty-Rechtecke aus.
- [ ] `spike/compare/main.go`: rendert dasselbe mit `internal/renderer` (gleiche feste Zeit) und vergleicht mit der PPM: Anteil Pixel mit |Δ| > 8 je Kanal, maximales Δ; schreibt `diff.png`.
- [ ] Prüfen: beide Templates mit `default.json`; Ergebnis in `spike/RESULTS.md`.
- [ ] Commit.

### Task 4: Firmware-Spike

**Files (Firmware-Repo, Branch `spike/template-render`):** Create `lib/zzarender/` (Kopie aus Task 2), `src/SpikeTemplate.{h,cpp}`, `data/tpl_faltblatt-105-v1/…`, `data/tpl_umuc-105-v1/…`; Modify `src/main.cpp` (minimaler Hook).

- [ ] `SpikeTemplate`: lädt per LittleFS, rendert GleisA aus `Config::getConfigString()` in einen PSRAM-RGB888-Puffer, wandelt nach RGB565 in `sprite2`/`sprite1`, `pushDual()`; misst mit `micros()` Laden, Rendern, Umwandeln, Push; loggt `ESP.getFreeHeap()`, `ESP.getFreePsram()`.
- [ ] Umschalten per Compile-Flag `-DSPIKE_TEMPLATE=\"faltblatt-105-v1\"`; bei Split-Flap alle Frames mit Teil-Updates abspielen und Frame-Zeit loggen; bei Frame-Modus Schleife mit `frame_delay`.
- [ ] `pio run -e ZZA_Universal_Display_Controller_105_DUAL_Debug` baut; Flash/RAM-Ausgabe und Partitionstabelle (LittleFS-Größe) in `spike/RESULTS.md` (Generator-Repo) notieren.
- [ ] Gerätemessung (Christoph): `pio run -t uploadfs` + `-t upload`, Serial-Log in `spike/RESULTS.md`.
- [ ] Commit (Firmware-Spike-Branch).

### Task 5: Ergebnis

- [ ] Nachtrag „Messwerte Stufe 1“ in der Spec mit Go/No-Go-Empfehlung und den Format-Entscheidungen (Bilder roh/RLE, Glyphen vorgeskaliert ok?, Kerning nötig?).
- [ ] Commit auf `feature/zza-on-device-rendering`.
