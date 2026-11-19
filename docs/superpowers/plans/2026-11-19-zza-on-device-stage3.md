# ZZA-Rendering auf dem Gerät — Stufe 3: Firmware-Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Der ZZA-Controller (DUAL und QUAD) rendert installierte Templates selbst, pro Gleis wählbar, mit Split-Flap- und Frame-Animation, Upload über die Firmware-Web-UI und sauberem Rückfall auf das eingebaute Layout.

**Architecture:** Der C++-Kern wird per Script aus dem Generator-Repo nach `lib/zzarender/` kopiert. Ein Adapter `DisplayTemplate` je Gleis lädt das Paket aus LittleFS, rendert in die vorhandenen Sprites (`sprite2` = Zeilen 0–119/D2, `sprite1` = 120–239/D1) und spielt Animationen als Zustandsmaschine ab, die je `loop()`-Durchlauf höchstens einen Frame rechnet. Reine Logik (Namen, Manifest, Ablage, Strategie, Zeitplan) liegt Arduino-frei in `src/tpl/` und wird auf dem Host getestet; das Gerät wird über E2E (Playwright) gegen die ZZA-Vorschau geprüft.

**Tech Stack:** C++17 (Exceptions an), Arduino-ESP32 (PlatformIO `espressif32`, Board `lolin_s3`), TFT_eSPI (mobadi), ArduinoJson 6.21.6 (Firmware: `lib_deps`; Kern: eigene gepatchte Kopie), LittleFS, Vanilla-JS, Node `node:test`, Playwright 1.49, Go (Generator-Repo).

**Spec:** `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md` (Generator-Repo), insb. §2, §4, §6.1, §6.3, §8.2, §9, §10 Stufe 3, §12, §13.

**Repos und Branches:**
- Generator: `zza-generate-images`, Branch `feature/zza-on-device-rendering` (weiter, kein Merge vor Ende aller Stufen).
- Firmware: `Zugzielanzeiger-GIT`, Worktree `~/Documents/_Projects/Zugzielanzeiger/Zugzielanzeiger-tpl`, Branch `feature/zza-template-render` (von `feature/animated-gif`, Entscheidung Christoph 2026-11-19). Nichts nach `develop` mergen.

## Global Constraints

- **Allgemeingültig (Spec Ziel 8):** keine Regel, Grenze oder Optimierung anhand von Template-Namen oder heutigen Templates. Strategien (live/vorrendern/Rückfall) entscheiden nach **gemessenen Kosten**.
- **Anti-Ziele (Spec §4):** Gleise ohne Template verhalten sich exakt wie heute; ein defektes Paket führt nie zu Boot-Schleife oder Absturz und legt das bisherige Template nicht lahm; kein unbegrenzter Speicher- oder Rechenaufwand.
- **Rückfall-Kette immer vorhanden (Spec §12.4 [C]):** vorrendern → live (langsamer) → nur Endbild → eingebautes Layout mit Fehlermeldung. Jede Stufe ohne Absturz.
- **Speicher je Gleis fest getrennt (Spec §12.4):** jedes Gleis leiht nur aus seinem festen Anteil; ein Gleis kann dem anderen nie Speicher nehmen und wartet nie auf das andere.
- Canvas 240 × 240 = ein Gleis; Zeilen 0–119 → `sprite2` (D2), 120–239 → `sprite1` (D1), wie `DisplayPng`. Screenshot, `/rawX` und Client-Modus funktionieren dadurch unverändert.
- Zeit: **zufällige Uhrzeit** je Start bzw. Zugwechsel [C], kein NTP/RTC.
- `lib/zzarender/` wird **nie direkt bearbeitet**; Änderungen am Kern nur im Generator-Repo (`device/zzarender/`) und per `tools/sync-zzarender.sh` übernehmen.
- Mobadi-Regel der Firmware: der `lib_deps`-Block bleibt identisch mit Werbeanzeige/Tankstelle (ArduinoJson bleibt drin, siehe Task 1).
- Firmware muss weiter für alle Umgebungen bauen: `ZZA_Universal_Display_Controller_105_{QUAD,DUAL}_{Debug,Release}` und `Zugzielanzeiger_ST7735_096_QUAD_Debug` (S2 ohne Templates: der Kern wird dort nicht gelinkt).
- Interner Heap nach Laden und nach Animation ≥ 120 KB frei (Spec §12.1); Loop-Stack-Reserve ≥ 4 KB auf allen worst-*-Templates.
- Generator-Repo: Server-Build bleibt CGO-frei (Prüfbefehl aus `CLAUDE.md`); `go vet ./...`, `go test ./...` mit `CGO_CXXFLAGS` (Mac) vor jedem Commit.
- Firmware-Repo: jedes Feature bekommt Test-IDs (`API-TPL-*`, `UI-TPL-*`, `MAN-TPL-*`), `docs/TESTKATALOG.md` per `npm run catalog`; Gerätetests nur mit dem vom Nutzer genannten Controller (QUAD am USB `/dev/cu.usbmodem2014301`, IP aus dem seriellen Log). Flashen/`uploadfs` ist erlaubt (Christoph 2026-11-16); `uploadfs` überschreibt das LittleFS.
- Nicht anfassen/committen: Generator `VERSION`, `.claude/settings.local.json`, `templates/wienerlinien-105-v1/`; Firmware `.claude/settings.local.json`, `src/Version.h`/`UI_VERSION` (Version setzt Christoph beim Release). Dateien einzeln stagen.
- Commits: Conventional Commits, Ende mit `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` und `Claude-Session: https://claude.ai/code/session_01WMHEz74Z9aB4eU1Gkf9CBp`. Kein Push.

## Review Focus

1. **Upload bricht mittendrin ab** (Browser zu, WLAN weg) → altes Template bleibt aktiv, der halbe `.new`-Ordner stört weder den nächsten Upload noch den Start. → Test in Task 3 (`TplStore`) und Task 4/5 (API-TPL-006).
2. **Strom weg während `templateCommit`** (zwischen den beiden Umbenennungen) → beim Start wird der letzte vollständige Stand wiederhergestellt. → Test in Task 3.
3. **Template eines Gleises fehlt oder wird gelöscht** (auch beim Booten) → eingebautes Layout, Status „nicht installiert“, kein Absturz; nach erneutem Upload rendert das Gleis wieder ohne Neustart. → Test in Task 5 (API-TPL-008) und Task 6.
4. **Zugwechsel während Animation oder Vorrendern** (Intervall 1 s, DCC-Doppelklick) → laufende Animation endet sauber, Leihe zurück, Speicher wieder auf Ausgangswert, neue Animation startet. → Test in Task 2 (`TplPlayer`) und Task 6 (API-TPL-013, Speicher über `/templateStatus`).
5. **QUAD: beide Gleise mit demselben bzw. zwei großen animierten Templates** → beide laufen (ggf. live oder als Endbild), keines blockiert das andere länger als einen Frame. → Test in Task 6 (API-TPL-014) und Task 7 (Messung).

---

## Dateistruktur

```
Generator-Repo (zza-generate-images)
  device/zzarender/third_party/ArduinoJson.h   ZZA PATCH: eigener Namespace (Task 1)
  device/zzarender/eval.cpp                    Textsuchen ins Rechenbudget (R16, Task 1)
  internal/cli/render.go                       --zza, --time (Task 1)
  tools/sync-zzarender.sh                      Kern in die Firmware kopieren / --check (Task 1)

Firmware-Repo (Zugzielanzeiger-tpl)
  lib/zzarender/                               Kopie des Kerns + SOURCE.txt (nie direkt bearbeiten)
  src/tpl/TplName.{h,cpp}                      Namensregeln, Ordnernamen
  src/tpl/TplManifest.{h,cpp}                  manifest.json prüfen
  src/tpl/TplStore.{h,cpp}                     Ablage: begin/commit/delete/Aufräumen (über TplFs)
  src/tpl/TplFs.h                              Dateisystem-Schnittstelle (Host: Speicher, Gerät: LittleFS)
  src/tpl/TplPixels.{h,cpp}                    RGB888→RGB565 (byte-vertauscht), Rechteck an y=120 teilen
  src/tpl/TplStrategy.{h,cpp}                  live/vorrendern/langsam/Endbild, Anteil je Gleis
  src/tpl/TplPlayer.{h,cpp}                    Frame-Zeitplan (fällig, dehnen, Schleife)
  src/TplFsLittle.{h,cpp}                      TplFs über LittleFS
  src/DisplayTemplate.{h,cpp}                  Adapter je Gleis (Kern, Sprites, Push, Animation, Status)
  src/WebServerTask.cpp, src/main.cpp, src/Config.{h,cpp}, src/ProjectSetup.h
  test/native/*.cpp, tools/test-native.sh      Host-Tests der src/tpl/-Module (clang++/g++)
  data/app/templates.htm, data/app/js/app-templates.js, data/app/js/zip.js
  data/app/js/app-komplett-backup.js, data/app/js/control2.js, data/app/control2.htm, data/app/js/burger-menu.js
  e2e/fixtures/zza/<name>/                     Test-Templates (YAML-Quellen)
  e2e/lib/zzaCli.ts, e2e/tests/api/templates.spec.ts, e2e/tests/ui/templates.spec.ts, e2e/tests/ui/app-templates.test.ts
  docs/API.md, docs/openapi.yaml (+ Kopie data/app/docs/openapi.yaml), CLAUDE.md, RELEASE_NOTES.md, e2e/manual/checklist.md
```

## Entscheidungen des Planers (dürfen gekippt werden)

- **P1 Bildquelle:** Neues Gleis-Feld `template` in `config.json` (Name oder leer). Ist es gesetzt, rendern die **Zugmodi** (Manuell, Intervall, Live, Client-Züge) lokal statt Online-Bild (`imageUrl` bleibt gespeichert, wird aber ignoriert). Bildmodi und Client-Bild bleiben unverändert. Grund: Die Online-Bild-Logik liegt im Code in den Zugmodi, nicht in `Mode_Png*` (Spec §6.3 [P] korrigiert).
- **P2 Stack:** Der Kern läuft im Loop-Task mit `SET_LOOP_TASK_STACK_SIZE(TPL_LOOP_STACK)` (Start 20 KB, Spike bewies 24 KB) statt in einem eigenen Task. Grund: Sprites/SPI bleiben in einem Task, keine Synchronisation. Lange Blockaden vermeidet die Zustandsmaschine (≤ 1 Frame je Durchlauf); nur das Laden (≈ 1,4 s) blockiert. Kosten bei Fehler: späterer Umbau auf eigenen Task.
- **P3 ArduinoJson:** Die Firmware behält `lib_deps` ArduinoJson (Mobadi-Regel); der Kern-Header bekommt per ZZA PATCH einen eigenen Namespace, damit beide Kopien ohne ODR-Konflikt nebeneinander stehen. Ersetzt Ruling R4 („einzige Kopie“).
- **P4 Upload-Ablauf:** zusätzlich `POST /templateBegin?name=` (leert/erstellt `/tpl_<name>.new`), damit ein Wiederholungsversuch nicht an Resten scheitert. `templateCommit` prüft und benennt nur um; das Löschen alter Ordner (`.old`, `.del`) erledigt der `webServerTask` danach in Häppchen (wie `wipeUserData`), Reste räumt der Start auf.
- **P5 Prüfung beim Commit:** Manifest (Format 1, Canvas 240 × 240, Name = Ordnername, jede gelistete Datei vorhanden mit Größe und SHA-256, keine fremden Dateien). Den Kern-Ladetest gibt es erst beim Laden im Gleis (Speicher); scheitert er, Rückfall + Fehlermeldung.
- **P6 Namenslänge:** Template-Namen auf dem Gerät `[a-z0-9-]`, 1–32 Zeichen (LittleFS-Pfadlänge: `/tpl_<32>.new/<12>` < 64). Der Generator lässt 64 zu; längere Namen lehnt die Web-UI vor dem Upload mit Klartext ab.
- **P7 Speicher-Startwerte** (in `ProjectSetup.h`, Task 7 misst und passt an): Kern-Budget je Gleis DUAL 4 MB / QUAD 2,5 MB; Vorrender-Anteil DUAL 3 MB / QUAD je 1,25 MB. Ein Template, dessen Kern-Bedarf das Budget übersteigt, fällt nach Spec auf Endbild bzw. eingebautes Layout zurück.
- **P8 Zufallszeit:** Datum 2026-01-01, Stunde/Minute/Sekunde aus `esp_random()`, Wochentag passend; neu bei Start und jedem Zugwechsel.
- **P9 Erzwingen der Rückfallstufen:** nur in Debug-Builds `GET /templateTune?gleis=a|b&share=<bytes>&budget=<bytes>&live=0|1` (Werte bis Neustart); Release ohne diese Route.

---

### Task 1: Generator — R16, eigener ArduinoJson-Namespace, `render --zza`, Sync-Script

**Files:**
- Modify: `device/zzarender/eval.cpp`, `device/zzarender/work.h` (falls nötig), `device/zzarender/third_party/ArduinoJson.h` (ZZA-PATCH-Block oben)
- Modify: `internal/devicerender/zzarender_unity.cpp` (Compile-Zeit-Prüfung unter `ZZA_TEST_HOOKS`), `internal/devicerender/work_test.go`
- Modify: `internal/cli/render.go`, `internal/cli/render_test.go`
- Create: `tools/sync-zzarender.sh`, `internal/devicerender/sync_test.go`
- Modify: `device/README.md`, Spec §6.1 (R4-Nachtrag ersetzen), §13 (R16 erledigt)

**Interfaces:**
- Produces: Namespace des Kern-ArduinoJson `ArduinoJson::ArduinoJson6216_ZZA` (`#define ARDUINOJSON_VERSION_NAMESPACE ArduinoJson6216_ZZA` im Patch-Block, vor der Default-Definition).
- Produces (CLI): `zza render -t <name> -i <data.json> -o <out.png> --zza [--time 2026-01-01T10:08:37]` rendert über `devexport.Export` + `devicerender` (nur PNG; mit `--format gif` und `--zza` → Fehler „--zza kann nur PNG“); `--time` (RFC 3339 ohne Zone = Zeitzone des Templates) gilt für beide Renderer. Inkompatibel → Exit ≠ 0, Meldung „nicht ZZA-kompatibel: …“.
- Produces (Script): `tools/sync-zzarender.sh <firmware-repo>` kopiert `device/zzarender/*.{h,cpp}` und `third_party/` (ohne `tools/`) nach `<fw>/lib/zzarender/`, löscht dort veraltete Dateien, schreibt `SOURCE.txt`: Zeile 1 `commit <git-hash>` (`-dirty` bei uncommitteten Änderungen in `device/`), Zeile 2 `date <ISO>`, danach je Datei `<sha256>  <pfad>` sortiert. `--check <fw>` → Exit 0 wenn gleich, sonst Exit 1 mit Liste der Abweichungen.

- [ ] **Step 1: Failing tests:**
  - `TestWorkLimitSubstringSearch` (`work_test.go`): Paket mit einem Text-Layer `{{a | replace('x','y') | before('q') | after('q') | contains...}}` über ein 256-KB-Datum ohne Treffer, mehrfach verkettet (je 32 Filter); `testSetWorkLimit(r, 2_000_000)`; `Render` → `ErrTemplate` mit „Rechenaufwand zu hoch“ in < 2 s Host-Zeit; `testWorkUsed` nach einem Render mit Limit 0 (Standard) ist ≥ Summe der durchsuchten Bytes. Dazu `TestWorkLimitMargin` unverändert grün.
  - `zzarender_unity.cpp`: `static_assert(sizeof(ArduinoJson::ArduinoJson6216_ZZA::JsonDocument) > 0, "…")` unter `#ifdef ZZA_TEST_HOOKS` (Build bricht ohne Namespace).
  - `TestRenderZZA` (`render_test.go`): `render --zza` auf `cov-text` (Templates-Dir `testdata/devtemplates`) schreibt PNG, Pixel == `devicerender.Render` derselben Daten/Zeit; `--zza --format gif` → Fehler; inkompatibles Template → Fehler mit „nicht ZZA-kompatibel“.
  - `TestSyncScript` (`sync_test.go`, überspringt ohne `bash`): Sync in Temp-Ordner, `--check` → 0; eine Datei in der Kopie ändern → `--check` 1 und nennt die Datei; zusätzliche Datei in der Kopie → nach erneutem Sync weg.
- [ ] **Step 2:** `go clean -cache && go test ./internal/devicerender ./internal/cli` → FAIL.
- [ ] **Step 3:** Implementieren. R16: jede Suche (`find` in `replace`, `before`, `after`, `strip*`, `contains`, `startsWith`, `endsWith`, `split_by`, Interpolations-Scan) belastet das Rechenbudget mit der Länge des durchsuchten Bereichs (bei `replace` je Treffer-Suche), über dieselbe Zählstelle wie die übrigen Evaluator-Bytes.
- [ ] **Step 4:** Tests grün, `go vet ./...`, `go test ./...`, Server-Prüfbefehl leer. README: Sync-Ablauf, Namespace-Grund. Spec §6.1: R4-Absatz durch P3 ersetzen; §13: R16 erledigt.
- [ ] **Step 5: Commits** `fix(device): charge substring searches to the work budget`, `feat(device): own ArduinoJson namespace, render --zza and sync script`

### Task 2: Firmware — Kern einbinden, Host-Testgerüst, reine Logik (`src/tpl/`)

**Files:**
- Create: `lib/zzarender/` (per `tools/sync-zzarender.sh`), `lib/zzarender/library.json` (`{"name":"zzarender","build":{"flags":["-std=gnu++17"]}}`, kein `srcFilter` auf `third_party`)
- Create: `src/tpl/TplName.{h,cpp}`, `src/tpl/TplPixels.{h,cpp}`, `src/tpl/TplStrategy.{h,cpp}`, `src/tpl/TplPlayer.{h,cpp}`
- Create: `test/native/test_main.cpp` (Mini-Harness: `CHECK(expr)`, `CHECK_EQ(a,b)`, Zählung, Exit-Code), `test/native/test_tpl_*.cpp`, `tools/test-native.sh`
- Create: `e2e/lib/zzarenderSync.test.ts`
- Modify: `platformio.ini` (`build_unflags = -std=gnu++11` / `build_flags = -std=gnu++17` für die 105-Umgebungen, falls nötig), `src/ProjectSetup.h` (Konstanten P7, `TPL_LOOP_STACK`)

**Interfaces (alle Arduino-frei, Namespace `tpl`):**
```cpp
bool validName(const char* s);                         // [a-z0-9-], 1..32
std::string dirOf(const std::string& name);            // "/tpl_<name>"
std::string newDirOf(const std::string& name);         // "/tpl_<name>.new"
bool validFileName(const char* s);                     // [a-z0-9.], 1..12 Zeichen, oder "manifest.json"
// Pixel: rgb888 (240*240*3) → sprite-Puffer (byte-vertauschtes RGB565 wie TFT_eSprite)
void toSprites(const uint8_t* rgb, int x, int y, int w, int h, uint16_t* top /*sprite2*/, uint16_t* bottom /*sprite1*/);
struct Part { bool top; int x, y, w, h; };             // y lokal im Display (0..119)
int splitAtHalf(int x, int y, int w, int h, Part out[2]); // Anzahl Teile 0..2
enum class Mode { Live, Prerender, LiveSlow, StillOnly };
Mode decide(uint32_t frameCostUs, uint32_t delayCs, size_t prerenderBytes, size_t shareBytes, bool animMemOk);
//  frameCostUs = gemessen (render + konvertieren + push); Live, wenn frameCostUs <= 0.7 * delayCs*10000;
//  sonst Prerender, wenn prerenderBytes <= shareBytes; sonst LiveSlow; !animMemOk → StillOnly.
class Lease { public: explicit Lease(size_t share); bool take(size_t n); void giveBack(); size_t used() const; };
class Player { // Zeitplan, reine Logik; Zeiten in ms
 public:
  void start(int frames, bool loop, uint32_t nowMs);
  int  due(uint32_t nowMs) const;          // Index des fälligen Frames oder -1
  void shown(int i, uint32_t delayCs, uint32_t nowMs); // nächster Termin = max(now, geplant) (Frames dehnen sich, nie überspringen)
  bool finished() const;                   // !loop und letzter Frame gezeigt
  void stop();
};
```

- [ ] **Step 1: Failing Host-Tests** (`test/native/`):
  - `TplName`: gültig `a`, `faltblatt-105-v1`, 32×`a`; ungültig `""`, 33×`a`, `A`, `a_b`, `a/b`, `..`, `a.b`, `ä`. `validFileName`: `t.json`, `f12.fnt`, `manifest.json` ja; `.x`, `a/b`, 13 Zeichen (außer manifest.json), `T.JSON` nein.
  - `TplPixels`: Pixel (255,0,0) → `0x00F8` im Puffer (byte-vertauscht von `0xF800`); Zeile 119 landet in `top`, 120 in `bottom` Zeile 0; `splitAtHalf(10,100,20,40)` → zwei Teile `{top,10,100,20,20}` und `{bottom,10,0,20,20}`; Rechteck nur oben/unten → ein Teil; `w<=0` → 0.
  - `TplStrategy`: 45 000 µs bei 100 cs → Live; 70 000 µs bei 5 cs → Prerender, wenn Bytes ≤ Anteil; sonst LiveSlow; `animMemOk=false` → StillOnly; `Lease(100)`: `take(60)` ja, `take(50)` nein, `giveBack()` → `used()==0`.
  - `TplPlayer`: 3 Frames, kein Loop: Frame 0 sofort fällig, nach `shown(0, 5, t)` Frame 1 bei t+50; verspätetes `shown` verschiebt die Folge (kein Überspringen); nach Frame 2 `finished()`; Loop springt nach letztem Frame auf 0; `stop()` → `due()==-1`. **Review Focus 4:** `start()` während laufender Wiedergabe setzt sauber neu auf.
  - `tools/test-native.sh`: baut `src/tpl/*.cpp` + `test/native/*.cpp` mit `c++ -std=c++17 -Wall -Wextra -Werror` (Mac: dieselben `-nostdinc++ -isystem …`-Flags wie im Generator, automatisch wenn `xcrun` vorhanden) und führt aus; Exit ≠ 0 bei Fehler.
  - `e2e/lib/zzarenderSync.test.ts`: liest `lib/zzarender/SOURCE.txt`, prüft SHA-256 jeder Datei und dass keine Datei fehlt oder zusätzlich da ist (erkennt direkte Änderungen an der Kopie).
- [ ] **Step 2:** `tools/test-native.sh` → FAIL (Module fehlen); `npm run unit` → FAIL (kein SOURCE.txt).
- [ ] **Step 3:** Sync ausführen (`tools/sync-zzarender.sh ../Zugzielanzeiger-tpl` im Generator-Repo), Module implementieren. In `main.cpp` noch nichts aufrufen außer einem Link-Test unter `#ifdef DEBUG`: einmal `zza_version()` loggen, damit der Kern gelinkt wird.
- [ ] **Step 4:** `tools/test-native.sh` grün; `npm run unit` grün; `pio run` für alle fünf Umgebungen grün (S2 ohne Kern, prüfen: Flash-Größe S2 unverändert ±1 KB). Fehlende Exceptions würden am `#error` scheitern — nicht abschalten.
- [ ] **Step 5: Commit** `feat(tpl): renderer core copy, host test harness and pure template logic`

### Task 3: Firmware — Ablage `TplStore` (Upload, Commit, Löschen, Aufräumen)

**Files:**
- Create: `src/tpl/TplFs.h`, `src/tpl/TplManifest.{h,cpp}`, `src/tpl/TplStore.{h,cpp}`, `src/TplFsLittle.{h,cpp}`, `test/native/test_tpl_store.cpp`, `test/native/MemFs.h`

**Interfaces:**
```cpp
struct TplFs { // alle Pfade absolut, eine Ordnerebene
  virtual bool exists(const std::string& p) = 0;
  virtual bool isDir(const std::string& p) = 0;
  virtual bool mkdir(const std::string& p) = 0;
  virtual bool rename(const std::string& from, const std::string& to) = 0; // Ordner und Dateien
  virtual bool remove(const std::string& p) = 0;                          // Datei
  virtual bool rmdir(const std::string& p) = 0;                           // leerer Ordner
  virtual std::vector<std::string> list(const std::string& dir) = 0;      // Namen ohne Pfad
  virtual bool readAll(const std::string& p, std::vector<uint8_t>& out) = 0;
  virtual bool readChunks(const std::string& p, const std::function<void(const uint8_t*, size_t)>& fn) = 0;
  virtual ~TplFs() = default;
};
struct ManifestFile { std::string n; size_t s; std::string h; };
bool parseManifest(const uint8_t* d, size_t n, const std::string& expectName, std::vector<ManifestFile>& out, std::string& err);
class TplStore {
 public:
  explicit TplStore(TplFs& fs);
  bool begin(const std::string& name, std::string& err);   // .new leeren/anlegen
  bool commit(const std::string& name, std::string& err);  // prüfen (P5), tpl_x → .old, .new → tpl_x
  bool remove(const std::string& name, std::string& err);  // tpl_x → tpl_x.del
  int  cleanupStep(int maxFiles);                          // löscht bis maxFiles Dateien aus *.old/*.del, gibt Rest zurück
  void bootRecover();                                      // siehe Step 1
  std::vector<std::string> installed();                    // Namen mit gültigem Ordner tpl_<name>
};
```
Fehlertexte (deutsch, erscheinen in der Web-UI): „Name ungültig“, „Upload fehlt“, „manifest.json fehlt oder ist kein gültiges Manifest“, „Format <n> wird nicht unterstützt“, „Leinwand muss 240 × 240 sein“, „Name im Manifest passt nicht“, „Datei fehlt: <n>“, „Größe falsch: <n>“, „Prüfsumme falsch: <n>“, „Unerwartete Datei: <n>“, „nicht installiert“.

- [ ] **Step 1: Failing Host-Tests** (`MemFs` = Speicher-Dateisystem mit Fehler-Injektion: `failRenameAfter(k)`, `failWrite`):
  - Manifest: gültiges Manifest aus einem echten Export (`e2e/fixtures/zza/…` exportiert, als Byte-Array im Test) → ok; jede Fehlermeldung oben einzeln ausgelöst; Format 2, Canvas `[240,120]`, Dateiname mit `/`, doppelte Datei, `s` negativ/zu groß, `h` nicht 64 Hex.
  - Commit: vollständiges `.new` → `tpl_x` enthält die Dateien, `.new` weg, altes `tpl_x` als `.old`; `cleanupStep(10)` bis 0 löscht `.old`. Prüfsumme falsch → Commit scheitert, altes `tpl_x` unverändert, `.new` bleibt (für Diagnose) und wird beim nächsten `begin` geleert. **Review Focus 1:** halbes `.new` + `begin` + vollständiger Upload + `commit` → ok.
  - **Review Focus 2:** `failRenameAfter(1)` (Strom weg nach `tpl_x → .old`): `bootRecover()` stellt `tpl_x` aus `.old` wieder her, `.new` gelöscht; Variante „beide Umbenennungen fertig, .old noch da“ → `.old` gelöscht, `tpl_x` neu.
  - `remove`: `tpl_x` weg (als `.del`), `installed()` ohne x; nicht vorhanden → „nicht installiert“.
  - `bootRecover` entfernt `*.new`, `*.del`, `*.old` (nach Wiederherstellung) vollständig.
- [ ] **Step 2:** `tools/test-native.sh` → FAIL.
- [ ] **Step 3:** Implementieren; SHA-256 über `ZzaSha256` aus `lib/zzarender/sha256.h` (Host-Test kompiliert `lib/zzarender/sha256.cpp` mit). `TplFsLittle` setzt die Schnittstelle mit `LittleFS` um (`readChunks` in 4-KB-Blöcken, `rename` von Ordnern über `LittleFS.rename`).
- [ ] **Step 4:** Host-Tests grün; `pio run` QUAD/DUAL Debug grün.
- [ ] **Step 5: Commit** `feat(tpl): template store with verified commit and crash recovery`

### Task 4: Firmware — Web-API für Templates, Backup-Verhalten (erstes Flashen auf QUAD)

**Files:**
- Modify: `src/WebServerTask.cpp` (`isUserDir`, `isUserFilePath`, neue Routen, Aufräumen im Task-Loop), `src/main.cpp` (Boot: `bootRecover()` vor dem ersten Laden), `data/app/js/app-komplett-backup.js` (`isUserDir`-Kopie, Wipe-Token)
- Modify: `docs/API.md`, `docs/openapi.yaml`, `data/app/docs/openapi.yaml` (Kopie), `e2e/tests/api/helpers.ts` (`CONFIG_KEYS` + `template`), `e2e/tests/ui/app-komplett-backup.test.ts`
- Create: `e2e/tests/api/templates.spec.ts`, `e2e/lib/zzaCli.ts` (`zzaExport(name): Buffer`, `zzaRender(name, dataJson): Buffer` (PNG über `zza render --zza`), Binärpfad aus `ZZA_CLI`, Templates-Dir `e2e/fixtures/zza`; ohne `ZZA_CLI` überspringen wie bei `MBD_CLI`), `installTemplate(dev, zipBuffer, name)` und `removeAllTemplates(dev)` in `e2e/tests/api/helpers.ts`, Fixture `e2e/fixtures/zza/e2e-static/`

**Interfaces (HTTP):**
- `isUserDir`: zusätzlich `tpl_<name>` (gilt für Liste, Backup, Restore). `isUserFilePath` für `POST /userFile` zusätzlich `tpl_<name>.new`. Namen nach `tpl::validName`.
- `POST /templateBegin?name=` → 200 `{"result":true}` | 400 `{"result":false,"error":"Name ungültig"}`.
- `POST /templateCommit?name=` → 200 `{"result":true}` | 400 mit Fehlertext aus Task 3 | 503 `{"result":false,"error":"busy"}` (Lade-Sperre > 3 s). Danach `invalidate()` für jedes Gleis mit diesem Template und `updateGleisX = true` (über `commandBus`, neuer Befehl `CMD_TPL_RELOAD`).
- `POST /templateDelete?name=` → 200 | 404 `{"result":false,"error":"nicht installiert"}`; Gleise mit diesem Template → Rückfall, Status `error:"nicht installiert"`, `config.json` behält den Namen (nach erneutem Upload rendert es wieder, **Review Focus 3**).
- `GET /templateStatus` → `{"installed":[{"name":"x","size":n}],"free":n,"a":{…statusJson},"b":{…}}` (`b` nur QUAD).
- `/wipeUserData?gleise=` versteht zusätzlich das Token `tpl` (löscht alle `tpl_*`-Dateien). Komplett-Backup: „Alles ersetzen“ schickt `tpl` mit, die anderen Modi nicht (Spec §6.3 Vorschlag).
- Sperre: ein FreeRTOS-Mutex `tplFsMutex`, gehalten vom Adapter beim Laden und von Commit/Delete beim Umbenennen; Commit wartet höchstens 3 s.

- [ ] **Step 1: Failing E2E** (`templates.spec.ts`, jede Prüfung räumt in `finally` auf: alle `tpl_*` löschen, `template` leeren):
  - `API-TPL-005 Upload, Commit, Liste`: Begin, alle Dateien hochladen, Commit → `templateStatus.installed` enthält den Namen mit Summe der Dateigrößen; `/userFiles` listet `/tpl_<name>/…`, aber keine `.new`-Pfade.
  - `API-TPL-006 Abgebrochener Upload`: Begin, nur die Hälfte hochladen, erneut Begin, vollständig, Commit → ok; ohne zweites Begin, aber mit falscher Datei → Commit 400 „Prüfsumme falsch: …“, vorher installiertes Template rendert weiter (Screenshot unverändert).
  - `API-TPL-007 Ungültige Namen und Pfade`: `name=../x`, `A`, 33 Zeichen → 400; `/userFile?p=/tpl_x.new/../config.json` → 400; `/userFile?p=/tpl_x.old/a` → 400.
  - `API-TPL-009 Backup enthält Templates`: Komplett-Backup-Liste (`/userFiles`) enthält `tpl_<name>`-Dateien; Restore-Pfad: `wipeUserData?gleise=tpl` nach `restoreBegin` löscht sie (im Batch, `remaining` bis 0), danach Neustart wie im bestehenden Backup-Test.
  - Node-Unit (`app-komplett-backup.test.ts`): `isUserFilePath('/tpl_ab-1/t.json', false)` true, `'/tpl_ab-1.new/t.json'` false (Backup), `'/tpl_A/t.json'` false; Plan „Alles ersetzen“ enthält `tpl` im Wipe.
- [ ] **Step 2:** Gerät mit Branch-Stand flashen (QUAD Release + `uploadfs` aus `data/`; vorher Christoph nicht erneut fragen, Freigabe liegt vor), `node run.ts --quad <ip> --yes --grep "API-TPL-00[5-79]"` → FAIL.
- [ ] **Step 3:** Implementieren; `templateStatus` liefert für die Gleise bis Task 5 `{"template":"<name>","state":"off"}`. API.md/OpenAPI ergänzen, Kopie nach `data/app/docs/openapi.yaml` (UI-DOCS-001).
- [ ] **Step 4:** Gerätelauf: API-TPL-005, 006, 007, 009 grün auf QUAD; bestehende `@smoke`- und Komplett-Backup-Tests grün.
- [ ] **Step 5: Commit** `feat(api): template upload, commit, delete and status`

### Task 5: Firmware — `DisplayTemplate` Standbild, Auswahl pro Gleis, Rückfall

**Files:**
- Create: `src/DisplayTemplate.{h,cpp}`
- Modify: `src/Config.{h,cpp}` (Feld `templateName`, JSON-Schlüssel `template`), `src/WebServerTask.cpp` (`/saveConfig` übernimmt `template`, wenn vorhanden), `src/main.cpp` (Aufrufe, Info-Display-Zeile), `src/ProjectSetup.h`

**Interfaces:**
```cpp
class DisplayTemplate {
 public:
  DisplayTemplate(DisplayBuffer& buf, char gleis /*'A'|'B'*/);
  void begin(size_t coreBudget, size_t shareBytes);         // zza_new; PSRAM-Hooks einmal global setzen
  bool active(const Config& cfg) const;                      // cfg.templateName != "" && Zugmodus
  bool show(const String& data);                             // lädt bei Bedarf, rendert Standbild, pushDual; false → Rückfall
  void service();                                            // Task 6; hier no-op
  void stop();                                               // laufende Animation beenden
  void invalidate();                                         // Paket neu laden beim nächsten show() (nach Commit/Delete)
  String statusJson() const;  // {"template":"x","state":"off|ok|error|loading","error":"…","mem":n,"peak":n,"mode":"still|live|prerender|liveSlow"}
};
extern DisplayTemplate tplA; // QUAD zusätzlich tplB
```
Laden: alle Dateien laut Manifest aus `/tpl_<name>/` in PSRAM-Puffer lesen, `zza_load`, Puffer freigeben (der Kern behält nichts). Boot-Schutz: vor dem ersten Laden nach dem Start `/tplboot.json` = `{"A":"<name>"}` schreiben, nach erfolgreichem erstem Render entfernen; findet der Start die Datei vor, gilt dieses Template als defekt bis zum nächsten Commit (Status „error: beim letzten Start abgestürzt“), Gleis zeigt eingebautes Layout. Beim Start zeigt jedes Gleis zuerst das eingebaute Layout (wie heute), danach lädt es sein Template (Spec §12.6). Rückfall bei jedem Fehler: `dispX.updateDisplay()` (eingebautes Layout) und Status `error` mit `zza_error()`. Zeit: P8.

Integration in `loop()`: Wo heute `hasImageUrlFlag` die Online-Bild-Abfrage auslöst, kommt davor: `if (tplX.active(cfg) && updateGleisX) { if (tplX.show(cfg.getConfigString()) ) updateGleisX = false; }` (sonst läuft der bestehende Pfad, also eingebautes Layout); `dispX.doTick()` und der 60-s-Online-Refresh entfallen, solange `active()`. `/showImage` als Zwischenbild: `tplX.stop()`, Template übermalt nichts, bis der Zug neu gezeichnet wird. Mode-Wechsel oder `template` geleert → `stop()` und eingebautes Layout (wie beim Leeren von `imageUrl`: `resetDispX`).

- [ ] **Step 1: Failing E2E** (`e2e/tests/api/templates.spec.ts`; Hilfen aus Task 4: `zzaExport`, `zzaRender`, `installTemplate`). Fixture `e2e/fixtures/zza/e2e-toolong/` anlegen (YAML-Quelle, im Generator mit `zza export` geprüft).
  - `API-TPL-001 Template rendert auf dem Gerät wie die ZZA-Vorschau`: Fixture `e2e-static` (zeitunabhängig, nutzt `zug1.*`, Zahlen mit Nachkommastellen und `format`, Bild, Kerning-Text; untere Hälfte enthält alles) installieren, Gleis A `template` setzen, Testzug setzen, `/GleisA/config.json` holen, `zzaRender` damit; `/screenshotA` muss der unteren Hälfte des erwarteten Bildes **nach RGB565-Quantisierung exakt** gleichen (prüft auch newlib `strtod`/`printf`).
  - `API-TPL-002 Fehlendes Template → eingebautes Layout`: `template` auf nicht installierten Namen → Screenshot gleicht dem eingebauten Layout ohne Template (Vergleich mit Screenshot bei `template:""`), `/templateStatus` `state:error`, `error:"nicht installiert"`.
  - `API-TPL-003 Gleis ohne Template unverändert`: QUAD: A mit Template, B ohne → B-Screenshot identisch zu vorher.
  - `API-TPL-004 Rückfall bei Kern-Fehler`: Fixture `e2e-toolong` (gültig exportiert, aber Speicherbedarf > Budget, z. B. viele große Bilder) → eingebautes Layout, Status `error`, Fehlertext enthält „Speicher“; Gerät läuft weiter (`/ping` uptime steigt).
  - `API-TPL-008 Löschen eines benutzten Templates` (**Review Focus 3**): Template auf A, Delete → eingebautes Layout, Status „nicht installiert“; erneuter Upload + Commit → A rendert wieder ohne Neustart.
  - `API-TPL-006` ergänzen: nach dem gescheiterten Commit rendert das vorher installierte Template weiter (Screenshot unverändert).
- [ ] **Step 2:** `pio run -e ZZA_Universal_Display_Controller_105_QUAD_Debug` → FAIL (Klasse fehlt).
- [ ] **Step 3:** Implementieren; `SET_LOOP_TASK_STACK_SIZE(TPL_LOOP_STACK)` in `DisplayTemplate.cpp`; PSRAM-Hooks `zza_raw_malloc = heap_caps_malloc(n, MALLOC_CAP_SPIRAM|MALLOC_CAP_8BIT)`, `zza_raw_free = heap_caps_free`. `CMD_TPL_RELOAD` aus Task 4 ruft `invalidate()`. Info-Display: eine Zeile `Tpl A: <name> ok|Fehler` unter der IP (nur wenn ein Gleis ein Template hat).
- [ ] **Step 4:** Alle fünf Umgebungen bauen; QUAD Release + `uploadfs` flashen; `node run.ts --quad <ip> --yes --grep API-TPL` grün (001–009); `@smoke` grün; serielles Log (Debug-Build einmal) ohne Panic. Messwerte (Ladezeit, Standbild-Zeit, freier Heap/PSRAM, Stack-Hochwasser) ins Task-Protokoll.
- [ ] **Step 5: Commit** `feat(display): render installed templates per Gleis with fallback to the built-in layout`

### Task 6: Firmware — Animation: live, vorrendern, Rückfall-Kette

**Files:**
- Create: Fixtures `e2e/fixtures/zza/e2e-flap/` (Split-Flap, zeitunabhängig) und `e2e/fixtures/zza/e2e-frames/` (Frame-Modus, `loop`)
- Modify: `src/DisplayTemplate.{h,cpp}` (Zustandsmaschine), `src/main.cpp` (`tplX.service()` in jedem `loop()`-Durchlauf, wenn `tickActive`), `src/WebServerTask.cpp` (`/templateTune` nur `#ifdef DEBUG`, P9)

**Ablauf (allgemein für Split-Flap und Frame-Modus):**
1. `show(data)`: neue Zufallszeit; laufende Animation `stop()` (= `zza_anim_end`, Leihe zurück); `zza_anim_begin`. `frames == 1` → Standbild-Pfad. `ZZA_ERR_NOMEM` → `zza_render` (Endbild); scheitert auch das → Rückfall eingebautes Layout.
2. Frames 0 und 1 werden live gerechnet, gezeigt und vermessen (render + `toSprites` + Push der Dirty-Teile). `tpl::decide()` mit dem größeren der beiden Werte (Frame 0 ist immer Vollbild und zählt nur, wenn es nur einen Frame gibt) entscheidet.
3. **Live:** je `service()` höchstens ein Frame, wenn `Player::due()`.
4. **Vorrendern:** Bedarf vorab = Summe der Dirty-Flächen ist unbekannt → Leihe wächst je Frame (`Lease::take` je gespeichertem Teilrechteck, RGB565 je Display-Hälfte, plus Kopf 12 B); scheitert `take` oder `heap_caps_malloc`, wird das Gespeicherte freigegeben und **LiveSlow** gespielt. Je `service()` höchstens ein Frame vorrendern; das Display zeigt solange Frame 1. Danach Wiedergabe aus dem Speicher (nur kopieren + pushen), alle Frames inkl. 0 sind gespeichert (Schleifen).
5. **Ende:** endliche Animation → nach dem letzten Frame `zza_anim_end` und Leihe zurück, Endbild steht. Schleife → läuft bis zum nächsten `show()`/`stop()`; Leihe bleibt.
6. Jede Wiedergabe schreibt auch in `sprite1`/`sprite2` (Screenshot zeigt den aktuellen Frame).

- [ ] **Step 1: Failing E2E:**
  - `API-TPL-010 Split-Flap läuft und endet auf dem Endbild`: Fixture `e2e-flap` (zeitunabhängig); nach Zugwechsel innerhalb von 0,5 s Screenshot ≠ Endbild (Animation läuft), nach `frames × delay + 3 s` Screenshot == `zzaRender` (Endbild, exakt nach 565-Quantisierung); `templateStatus.a.mode` ∈ {live, prerender, liveSlow}; danach `mem` wieder auf dem Wert vor dem Zugwechsel (±0).
  - `API-TPL-011 Schleife läuft weiter`: Fixture `e2e-frames` (Frame-Modus, `loop`, 2 Frames mit sichtbar verschiedenen Farben, 1 s): über 5 s mindestens 3 Wechsel der Farbe einer Probefläche.
  - `API-TPL-012 Rückfallstufen erzwungen` (`@debug`, nur Debug-Build; sonst übersprungen): `/templateTune?share=0` → `mode:liveSlow`, Endbild korrekt; `budget=<knapp>` (so gewählt, dass Laden + Standbild passen, Animation nicht) → `mode:still`, Endbild korrekt; `live=0` erzwingt Vorrendern, Ergebnis gleich.
  - `API-TPL-013 Zugwechsel während der Animation` (**Review Focus 4**): 5 Zugwechsel im Abstand von 300 ms → kein Neustart, letzter Zug als Endbild, `mem` nach Ende = Ausgangswert, `peak` ≤ Budget.
  - `API-TPL-014 QUAD: zwei Animationen gleichzeitig` (**Review Focus 5**): A `e2e-flap`, B `e2e-frames`; während A animiert, wechselt B weiter die Farbe (≥ 2 Wechsel in 3 s); beide Endzustände korrekt.
- [ ] **Step 2:** Flashen, `--grep API-TPL-01` → FAIL.
- [ ] **Step 3:** Implementieren.
- [ ] **Step 4:** API-TPL-001..014 grün auf QUAD (012 mit Debug-Build, danach wieder Release). Serielles Log je Animation: Modus, Frame-Kosten, fps, Leihe, Heap/PSRAM, Stack-Hochwasser.
- [ ] **Step 5: Commit** `feat(display): template animations with live, prerendered and fallback playback`

### Task 7: Messung auf QUAD und DUAL, Grenzwerte festlegen

**Files:**
- Create: `e2e/scripts/tpl-measure.ts` (installiert Templates, schaltet Zugwechsel, liest `/templateStatus` und das serielle Log nicht — Messwerte kommen aus `templateStatus` erweitert um `lastLoadMs`, `lastFrameUs`, `fps`, `heapFree`, `heapMin`, `psramFree`, `stackFree`)
- Modify: `src/DisplayTemplate.cpp` (`statusJson` um die Messfelder), `src/ProjectSetup.h` (endgültige Werte P7, `TPL_LOOP_STACK`, ggf. `TPL_WORK_LIMIT`)
- Modify (Generator): Spec neuer Abschnitt §14 „Messwerte Stufe 3“

- [ ] **Step 1:** Messplan in `e2e/scripts/tpl-measure.ts` je Template (alle ausgelieferten + `worst-*` aus dem Generator, als ZIP per `ZZA_CLI` exportiert): Ladezeit, Standbild-Zeit, Modus und fps je Animation, `peak`, Heap frei/min, PSRAM frei, Stack-Hochwasser; QUAD: A und B gleichzeitig animiert (faltblatt + umuc, faltblatt + faltblatt).
- [ ] **Step 2:** Rechenrate: `worst-layers` mit gemessenem `zza_work_used` und Zeit → Operationen/s. Firmware-Limit `TPL_WORK_LIMIT = min(ZZA_DEFAULT_WORK_LIMIT, 5 s × Rate)`, aber ≥ 2 × größter legitimer Wert aus §13 (sonst Befund melden, nicht stillschweigend senken).
- [ ] **Step 3:** Verschachtelung: Paket mit `t.json`-Tiefe 80 und Zugdaten-Tiefe am `jsoncheck`-Limit → kein Absturz, Stack-Hochwasser ≥ 4 KB Reserve; sonst `TPL_LOOP_STACK` erhöhen.
- [ ] **Step 4:** Grenzwerte festlegen (Budget, Anteil, Stack, Work-Limit), jeweils mit Messwert begründet; Kriterien prüfen: Heap ≥ 120 KB frei, keine Panic, Rückfall statt Absturz bei `worst-text` (4,1 MB) auf QUAD. DUAL: einmal DUAL-Release flashen (`node run.ts --dual <ip> --yes` nach Christophs Ablauf), Smoke + API-TPL-001/010, danach wieder QUAD.
- [ ] **Step 5:** Spec §14 im Generator-Repo mit Tabelle; Commits `perf(tpl): memory shares and limits from device measurements` (Firmware), `docs(device): stage 3 measurements` (Generator).

### Task 8: Firmware — Web-UI: Templates-Seite, Auswahl je Gleis

**Files:**
- Create: `data/app/templates.htm`, `data/app/js/app-templates.js`, `data/app/js/zip.js`, `e2e/tests/ui/app-templates.test.ts`, `e2e/tests/ui/templates.spec.ts`
- Modify: `data/app/js/app-komplett-backup.js` (nutzt `zip.js`, exportiert die Helfer für die bestehenden Tests weiter), `data/app/komplett-backup.htm` (Script-Tag), `data/app/js/burger-menu.js` (Eintrag „Templates“), `data/app/control2.htm` + `data/app/js/control2.js` (Auswahl „Template“ im Gleis-Dialog: „— keins —“ + installierte, Quelle `/templateStatus`; Hinweis „ersetzt das Online-Bild in den Zugmodi“)

**Interfaces (`app-templates.js`, unter Node `module.exports`):**
```js
checkPackage(entries, freeBytes) → { name, files:[{name,data}], errors:[string] }
// entries aus zipEntries + inflate; prüft: manifest.json vorhanden/parsebar, format 1, canvas [240,240],
// Name nach P6 (sonst „Name zu lang für den ZZA (max. 32 Zeichen)“), jede Datei vorhanden mit Größe und SHA-256 (sha256.js),
// keine fremden Dateien, Summe ≤ freeBytes (sonst „Zu wenig Platz: <n> KB nötig, <m> KB frei“). Ein ZIP mit einem Oberordner wird akzeptiert.
installSteps(name, files) → [{method, url, body?}]   // templateBegin, je Datei userFile, templateCommit
```
Seite: ZIP wählen → Prüfergebnis → „Installieren“ mit Fortschritt (Datei n von m) → Ergebnis; Liste installierter Templates mit Größe, Nutzung durch Gleis A/B und „Löschen“ (eigene Bestätigung in der Seite, kein `confirm()`); Platzanzeige.

- [ ] **Step 1: Failing Tests:**
  - Node-Unit `app-templates.test.ts`: gültiges Fixture-ZIP (aus `ZZA_CLI` oder eingecheckt unter `e2e/fixtures/zza/e2e-static.zip`) → keine Fehler; je ein Test pro Fehlertext (Manifest fehlt, Format 2, Canvas falsch, Datei fehlt, Prüfsumme falsch, fremde Datei, Name 33 Zeichen, zu wenig Platz); Oberordner-ZIP ok; `installSteps` Reihenfolge Begin → Dateien → Commit, URLs korrekt kodiert.
  - Bestehende `app-komplett-backup.test.ts` bleiben grün (Helfer jetzt aus `zip.js`).
  - UI-E2E `templates.spec.ts`: `UI-TPL-001` ZIP hochladen → erscheint in der Liste; `UI-TPL-002` kaputtes ZIP → Fehlertext, nichts installiert; `UI-TPL-003` im Gleis-Dialog Template wählen → Screenshot-Kachel zeigt das Template; `UI-TPL-004` Löschen mit Bestätigung; `UI-TPL-005` mobil (iPhone 13) bedienbar, keine horizontale Scroll.
- [ ] **Step 2:** `npm run unit` → FAIL; UI-E2E → FAIL.
- [ ] **Step 3:** Implementieren (Vanilla-JS, Stil wie `komplett-backup.htm`).
- [ ] **Step 4:** Unit grün; `uploadfs` + `node run.ts --quad <ip> --yes --grep UI-TPL` grün; `@smoke` grün.
- [ ] **Step 5: Commit** `feat(webui): templates page and per-Gleis template selection`

### Task 9: Doku, Testkatalog, Abschluss

**Files:**
- Modify (Firmware): `CLAUDE.md` (Abschnitt Templates: Architektur, `src/tpl/`, Host-Tests, Sync-Regel, Endpunkte, E2E mit `ZZA_CLI`), `RELEASE_NOTES.md` (Eintrag „Templates direkt auf dem ZZA“, ohne Versionsnummer), `e2e/manual/checklist.md` (`MAN-TPL-001` Animation flüssig ansehen, `MAN-TPL-002` Strom ziehen während Commit, `MAN-TPL-003` Booten mit defektem Paket, `MAN-TPL-004` DUAL komplett), `docs/TESTKATALOG.md` (`npm run catalog`), `e2e/README.md` (`ZZA_CLI`)
- Modify (Generator): `docs/backlog.md` (Stufe 3 erledigt, Reste), `CLAUDE.md` (Hinweis Sync-Script), manueller Testplan `docs/superpowers/plans/2026-11-19-zza-on-device-stage3-manual-tests.md` abhaken

- [ ] **Step 1:** Doku schreiben; `npm run catalog` und `npm run unit`.
- [ ] **Step 2:** Volllauf: Generator `go vet ./...` + `go test ./...` + Server-Prüfung; Firmware `tools/test-native.sh`, alle fünf `pio run`, `tools/sync-zzarender.sh --check ../Zugzielanzeiger-tpl` = 0, `node run.ts --quad <ip> --yes` (ohne `@second`/`@dual`) grün.
- [ ] **Step 3: Commits** `docs: templates on the device` (beide Repos)
