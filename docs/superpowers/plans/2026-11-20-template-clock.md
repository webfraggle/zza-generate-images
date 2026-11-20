# Uhrzeit in Templates (laufendes `now`) – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Templates zeigen im GIF eine ab der Startzeit laufende Uhr und auf dem Gerät die Geräte-Uhr (`zzaClock`), auch in Standbildern.

**Architecture:** `now` läuft im Frame-Modus mit. Der Go-Renderer und der C++-Kern rechnen Start + i × `frame_delay`; das Gerät gibt dem Kern je Bild die Zeit von `zzaClock`. Der Export schreibt ins Manifest, ob ein Paket die Uhr auf Minuten oder Sekunden braucht; die Firmware aktualisiert danach.

**Tech Stack:** Go (Renderer, Export, cgo-Brücke), C++17 (Kern `device/zzarender`), ESP32-Arduino/PlatformIO (Firmware), Playwright/Node (E2E).

**Spec:** `docs/superpowers/specs/2026-11-20-template-clock-design.md`

## Global Constraints

- Zwei Repos: Generator hier (`feature/zza-on-device-rendering`), Firmware in `../Zugzielanzeiger-tpl` (`feature/zza-template-render`). Nichts nach `develop` mergen, nicht pushen ohne Auftrag.
- Kern nur in `device/zzarender/` ändern, dann `tools/sync-zzarender.sh ../Zugzielanzeiger-tpl`; `tools/sync-zzarender.sh --check ../Zugzielanzeiger-tpl` muss Exit 0 liefern.
- Server-Build bleibt CGO-frei (Prüfbefehl in `CLAUDE.md`).
- Go-Tests der Brücke auf dem Mac: `export CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"`.
- Template-Dateien direkt bearbeiten (User 2026-11-20, kein MCP nötig), nur Latin-1.
- Datum und Zeitzone auf dem Gerät: Datum fest 1.1.2026 (Donnerstag), `meta.timezone` gilt dort nicht.
- Fehlt `clock` im Manifest, gilt `none`. Formatnummer des Pakets bleibt 1.
- „PNG = Frame 0“ und „bytegleiches GIF ohne `now`“ gelten weiter. Split-Flap friert `now` beim Start ein.
- Firmware: je Gleis höchstens ein Bild je `service()`; Flashen mit ~10 s Pause zwischen Reset-Schritten.
- PlatformIO: `~/.platformio/penv/bin/pio`.

## Review Focus

1. Zeit springt rückwärts (Uhr gestellt, NTP-Sync, Mitternacht 23:59:59 → 00:00:00): das Bild muss folgen, nicht auf „größer als“ warten. → Task 6 (Test `due_on_any_change`).
2. Faktor 100 mit `clock=second`: die Schleife darf nicht dauernd rendern. → Task 6 (`min_gap`).
3. `frame_delay` mit Bruchteilen (0,3 s) und Startzeit mit Nanosekunden: Go und Kern müssen dieselbe Sekunde zeigen. → Task 1 und 3.
4. Kein Speicher für den Vergleichspuffer beim Standbild-Update (`ZZA_ERR_NOMEM`): Bild muss trotzdem aktualisiert werden. → Task 4 und 7.
5. `format`-Muster aus einer Variablen bzw. `format` nur mit Datum: Einstufung `second` bzw. `minute`. → Task 2.

---

## Generator

### Task 1: Go-Renderer – `now` läuft im Frame-Modus

**Files:**
- Modify: `internal/renderer/animation.go` (`renderFrameMode`), `internal/renderer/evaluator.go`
- Test: `internal/renderer/animation_clock_test.go` (neu)

**Interfaces:**
- Produces: `func (e *Evaluator) withNow(t time.Time) *Evaluator` (Kopie mit anderem `now`). Regel: Start = `r.clock()` in `tmpl.location()`, **auf die Sekunde abgeschnitten**; Frame f: `start.Add(time.Duration(f*delayCS) * 10 * time.Millisecond)`.

- [ ] **Step 1: Failing tests** (Renderer über `NewWithClock`, Template im Test mit `frames: 60`, `frame_delay: 1`, ein Text-Layer `{{now | format('HH:mm:ss')}}`; Prüfung über `EvalForTest`-artigen Zugriff oder Pixelvergleich mit einem PNG-Render zur selben Zeit):
  - `TestFrameModeNowAdvances`: Start 10:08:37 → Frame 0 = PNG zur Zeit 10:08:37, Frame 23 = PNG zur Zeit 10:09:00.
  - `TestFrameModeNowCrossesMidnight`: Start 23:59:30 → Frame 30 = PNG zur Zeit 00:00:00 des Folgetags (`now.day` +1).
  - `TestFrameModeFractionalDelay`: `frame_delay: 0.3`, Start 10:00:00.900 → Frame 3 zeigt 10:00:00, Frame 4 zeigt 10:00:01 (Start abgeschnitten).
  - `TestSplitflapNowFrozen`: Split-Flap-Template mit `now.second`-Text → alle Frames dieselbe Sekunde.
- [ ] **Step 2:** `go test ./internal/renderer/ -run 'FrameModeNow|SplitflapNowFrozen'` → FAIL.
- [ ] **Step 3:** `withNow` und die Regel in `renderFrameMode` umsetzen; Kommentare „now is frozen“ dort korrigieren.
- [ ] **Step 4:** `go test ./internal/renderer/` → PASS (bestehende Tests, die eingefrorenes `now` im Frame-Modus prüfen, auf die neue Regel umstellen).
- [ ] **Step 5:** Commit `feat(renderer): now advances per frame in frame mode`.

### Task 2: Uhr-Stufe eines Templates und Manifest-Feld `clock`

**Files:**
- Modify: `internal/renderer/usesnow.go`, `internal/devexport/export.go`
- Test: `internal/renderer/usesnow_test.go`, `internal/devexport/export_test.go`

**Interfaces:**
- Produces: `func (t *Template) ClockGranularity() string` → `"none" | "minute" | "second"`. Manifest: `"clock":"…"` nach `features`.
- Regel: `second`, wenn irgendein ausgewerteter Ausdruck `now.second` enthält oder `format(` mit einem Literal-Muster, das `ss` enthält, oder `format(` mit einem Muster, das kein Zeichenketten-Literal ist; sonst `minute`, wenn `UsesNow()`; sonst `none`.

- [ ] **Step 1: Failing tests** `TestClockGranularity` (Tabelle): kein `now` → `none`; `{{now}}` → `minute`; `{{now.day}}` → `minute`; `{{now | format('dd.MM.')}}` → `minute`; `{{now.second | mul(6)}}` → `second`; `{{now | format('HH:mm:ss')}}` → `second`; `{{now | format(muster)}}` → `second`; `now.second` nur in `if:` eines verschachtelten Layers → `second`; Text `now.second` außerhalb `{{…}}` → `none`. Export: `TestManifestClock` – Manifest von `umuc-105-v1` hat `"clock":"second"`, von `faltblatt-105-v1` den Wert laut `ClockGranularity()`.
- [ ] **Step 2:** `go test ./internal/renderer/ ./internal/devexport/ -run 'ClockGranularity|ManifestClock'` → FAIL.
- [ ] **Step 3:** Umsetzen (gleiche Traversierung wie `layersUseNow`).
- [ ] **Step 4:** `go test ./internal/renderer/ ./internal/devexport/` → PASS.
- [ ] **Step 5:** Commit `feat(export): manifest clock granularity`.

### Task 3: Kern – Zeit je Frame (`zza_anim_frame`)

**Files:**
- Modify: `device/zzarender/zzarender_c.h`, `capi.cpp`, `anim.cpp`, `anim.h`, `layers.h` (Deklaration `animFrame`), `internal/devicerender/devicerender.go`, `internal/devicerender/testhooks.go`
- Test: `internal/devicerender/anim_test.go`

**Interfaces:**
- Produces (ersetzt die alte Signatur):
  ```c
  int zza_anim_frame(zza_renderer*, int i, const zza_time* now, unsigned char* rgb888, int* delay_cs,
                     zza_rect* dirty, int max_rects, int* n_rects);
  ```
  `now == NULL`: Frame-Modus = Startzeit aus `zza_anim_begin` + `floor(i × delay_cs / 100)` Sekunden mit Übertrag bis ins Datum (`daysFromCivil` ist in `eval.cpp` vorhanden); Split-Flap und Standbild = Startzeit. `now != NULL`: gilt im Frame-Modus und im Standbild-Modus (`frames == 1`) für diesen Frame; bei Split-Flap ignoriert.
- Go-Brücke (`Animate`) ruft mit `nil`. Testhook: `testAnimFrameAt(i int, now *time.Time) (code int, img *image.NRGBA)`.

- [ ] **Step 1: Failing tests:**
  - `TestAnimFrameNowParity`: für `umuc-105-v1` (nach Task 5 neu, bis dahin ein Testpaket mit `{{now | format('HH:mm:ss')}}`, 60 Frames) Start 23:59:30 – jeder Frame von `Animate` pixelgleich zum Go-`RenderAnimation` mit derselben Startzeit (Vergleich wie in `compare_test.go`).
  - `TestAnimFrameNowOverride`: Frame 5 mit `now` = 12:00:00 ist pixelgleich zu `Render(data, 12:00:00)` eines Templates ohne `anim.*`.
  - `TestAnimFrameNowIgnoredForSplitflap`: Split-Flap, Frame k mit und ohne `now` identisch.
- [ ] **Step 2:** `go test ./internal/devicerender/ -run AnimFrameNow` → FAIL (kompiliert nicht / falsche Pixel).
- [ ] **Step 3:** Kern und Brücke umsetzen; Kommentar an `zza_anim_begin` („now is frozen“) anpassen.
- [ ] **Step 4:** `go test ./internal/devicerender/ ./internal/devpreview/ ./internal/mcp/` → PASS.
- [ ] **Step 5:** Commit `feat(zzarender): per-frame now in zza_anim_frame`.

### Task 4: Kern – `zza_clock` und `zza_render_diff`

**Files:**
- Modify: `device/zzarender/zzarender_c.h`, `capi.cpp`, `package.cpp`/`package.h`, `layers.cpp`/`layers.h`, `internal/devicerender/testhooks.go`
- Test: `internal/devicerender/load_test.go`, `internal/devicerender/render_test.go`

**Interfaces:**
- Produces:
  ```c
  enum { ZZA_CLOCK_NONE = 0, ZZA_CLOCK_MINUTE = 1, ZZA_CLOCK_SECOND = 2 };
  int zza_clock(const zza_renderer*);   /* aus manifest "clock"; ohne Paket oder ohne Feld: NONE */
  /* Wie zza_render, aber rgb888 muss das vorige Bild enthalten; meldet die geaenderten Rechtecke
     wie zza_anim_frame (0 = unveraendert). ZZA_ERR_STATE,
     solange eine Animation offen ist. Der Vergleichspuffer (172800 Bytes) ist Budget-Speicher nur
     waehrend des Aufrufs; ZZA_ERR_NOMEM laesst rgb888 unveraendert. */
  int zza_render_diff(zza_renderer*, const char* data, size_t len, const zza_time* now,
                      unsigned char* rgb888, zza_rect* dirty, int max_rects, int* n_rects);
  ```
  Manifest `clock` mit anderem Wert als `none|minute|second` → `ZZA_ERR_FORMAT` („manifest.json: invalid clock“).
- Testhooks: `testClock() int`, `testRenderDiff(data []byte, now time.Time, prev *image.NRGBA, maxRects int) (code int, img *image.NRGBA, rects []image.Rectangle)`.

- [ ] **Step 1: Failing tests:** `TestClockFromManifest` (fehlend → 0, `minute` → 1, `second` → 2, `"hourly"` → Format-Fehler); `TestRenderDiff`: Bild zur Zeit A, dann Diff zur Zeit B = `Render(B)`, alle geänderten Pixel liegen in den gemeldeten Rechtecken, gleiche Zeit → 0 Rechtecke; `TestRenderDiffNoMem` (Budget so knapp, dass der Puffer fehlt): `ErrNoMem`, Bild unverändert; `TestRenderDiffDuringAnimation` → State-Fehler.
- [ ] **Step 2:** `go test ./internal/devicerender/ -run 'ClockFromManifest|RenderDiff'` → FAIL.
- [ ] **Step 3:** Umsetzen (Rechtecke mit `diffFrames` aus `anim.h`).
- [ ] **Step 4:** `go test ./internal/devicerender/` → PASS; Server-Import-Prüfung aus `CLAUDE.md` leer.
- [ ] **Step 5:** Commit `feat(zzarender): zza_clock and zza_render_diff`.

### Task 5: Templates und Doku

**Files:**
- Modify (direkt): `templates/umuc-105-v1/template.yaml`, `templates/nederland-105-v1/template.yaml` (`{{now.second | mul(6)}}`), `templates/oebb-105-v1/template.yaml` (`{{now | format('HH:mm:ss')}}`)
- Modify: `docs/yaml-template-spec.md` (meta/Determinismus Z. ~378, Frame-Animation Z. ~401–426, Variablen ~508–546), `docs/ai-design-guide.md`, `device/README.md` (API), `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md` (§ Uhrzeit: Verweis auf die neue Spec)

- [ ] **Step 1:** Die drei Templates direkt ändern. Jedes muss sich weiter exportieren lassen (`go run ./cmd/zza export` bzw. der vorhandene Export-Test über alle Templates).
- [ ] **Step 2:** Test `TestShippedClockTemplates` in `internal/renderer/animation_clock_test.go`: für die drei Templates, Start 10:08:57 – Frame 3 ist pixelgleich zum PNG zur Zeit 10:09:00 (Sekunden laufen einfach, Minute springt). `go test ./...` → PASS.
- [ ] **Step 3:** Doku: neue Tabelle „Bedeutung von `now`“ aus der Spec übernehmen; Beispiele auf `now.second` / `format('HH:mm:ss')` umstellen; Hinweis, dass `add(anim.frame)` bzw. `anim.time` als Versatz die Uhr doppelt zählt; Manifest-Feld `clock`.
- [ ] **Step 4:** `go test ./docs/... ./internal/mcp/` (Guide-Tests) → PASS. Commit `feat(templates): clocks use running now; docs`.

---

## Firmware (`../Zugzielanzeiger-tpl`)

### Task 6: Kern übernehmen, Geräte-Zeit, Takt-Logik (Host-Tests)

**Files:**
- Modify: `lib/zzarender/*` (nur per Sync), `src/DisplayTemplate.cpp` (Aufrufe von `zza_anim_frame` mit `nullptr`, `deviceTime` statt `randomTime`), `src/tpl/TplAdapter.h/.cpp`
- Create: `src/tpl/TplClockTick.h/.cpp`, `test/native/test_tpl_clocktick.cpp`
- Test: `test/native/test_tpl_adapter.cpp`

**Interfaces:**
- Produces:
  ```cpp
  namespace tpl {
  zza_time deviceTime(uint8_t h, uint8_t m, uint8_t s);   // Datum 2026-01-01, weekday 4; ersetzt randomTime
  class ClockTick {                                        // wann ein Uhr-Update faellig ist
   public:
    void reset();                                          // naechstes due() ist sofort wahr
    // clock: ZZA_CLOCK_*; true, wenn sich (h,m) bzw. (h,m,s) gegen das zuletzt gezeigte geaendert hat
    // und seit dem letzten Update mindestens gapMs() vergangen sind. NONE: nie.
    bool due(int clock, const zza_time& now, uint32_t nowMs) const;
    void shown(const zza_time& now, uint32_t nowMs, uint32_t costUs);
    uint32_t gapMs() const;                                // max(100, costUs / 700)  (70-%-Regel)
  };
  }
  ```
- `DisplayTemplate` liest die Zeit an genau einer Stelle: `zza_time DisplayTemplate::clockNow()` (`zzaClock.now()` → `deviceTime`).

- [ ] **Step 1:** Im Generator `tools/sync-zzarender.sh ../Zugzielanzeiger-tpl`; `--check` → Exit 0.
- [ ] **Step 2: Failing host tests:** `deviceTime(23,59,58)` → Felder wie oben; `ClockTick`: `due_first_call` (nach `reset` wahr); `due_minute_ignores_seconds` (10:08:01 → 10:08:59 falsch, 10:09:00 wahr); `due_second`; `due_on_any_change` (23:59:59 → 00:00:00 und 12:00 → 11:59 wahr); `min_gap` (Sekunde geändert, aber 40 ms nach `shown` → falsch; nach 100 ms wahr; `costUs` 210000 → Abstand 300 ms); `none_never`; `millis`-Überlauf (`nowMs` läuft über `0xFFFFFFFF`).
- [ ] **Step 3:** `tools/test-native.sh` → FAIL.
- [ ] **Step 4:** Umsetzen; `randomTime` und `timeSet_`-Zufallslogik entfernen (Zeit beim Start einer Darstellung = `clockNow()`); alle `zza_anim_frame`-Aufrufe vorerst mit `nullptr`.
- [ ] **Step 5:** `tools/test-native.sh` → PASS; `pio run -e ZZA_Universal_Display_Controller_105_QUAD_Release` und `…_DUAL_Release` → SUCCESS.
- [ ] **Step 6:** Commit `feat(tpl): device clock as template time, ClockTick`.

### Task 7: `DisplayTemplate` – laufende Uhr

**Files:**
- Modify: `src/DisplayTemplate.h/.cpp`, `src/WebServerTask.cpp` (`/templateStatus`), `docs/API.md`, `docs/openapi.yaml` + Kopie `data/app/docs/openapi.yaml`
- Test: `e2e/tests/api/templates.spec.ts`, `e2e/tests/api/helpers.ts` (`GleisTplStatus.clock`)

**Interfaces:**
- Consumes: `zza_clock`, `zza_render_diff`, `zza_anim_frame(…, now, …)`, `tpl::ClockTick`, `clockNow()`, E2E-Helfer `withClock` (aus develop).
- Produces: `/templateStatus` je Gleis `"clock":"none|minute|second"`.

Verhalten (Spec, Tabelle „Verhalten je Paket“), `clock_ = zza_clock(r_)` nach dem Laden:

| Zustand | Regel |
|---|---|
| Standbild gezeigt, `clock_ != NONE` | in `service()`: `tick_.due(clock_, clockNow(), millis())` → `zza_render_diff`, nur die Rechtecke zeichnen (`paintRects`), `tick_.shown(…)`. `ZZA_ERR_NOMEM` → `zza_render` + `paintAll`. Anderer Fehler oder Kosten über den Grenzen (`tooCostly`-Regeln: > 1,5 s) → Updates für diese Daten aus, Bild bleibt. |
| Frame-Modus, `clock_ != NONE` | kein `PH_MEASURE`/`PH_PRERENDER`: direkt `PH_LIVE`; jeder Frame `zza_anim_frame(i, &now, …)` mit `now = clockNow()`. Ist ein Frame zu spät, fallen Takte aus (kein Aufholen). `loop: false`: nach dem letzten Frame bleibt die Sitzung offen und Frame `frames-1` wird nach `tick_` neu gerendert. |
| Frame-Modus, `clock_ == SECOND`, `delay` = 100 cs, `zzaClock.factor() == 1` | Start (und jeder Schleifen-Neustart bei Frame 0) erst 250 ms nach einem beobachteten Sekundenwechsel der Uhr, damit kein Frame auf die Sekundengrenze fällt. |
| Split-Flap | unverändert (`nullptr`); nach `finish()` gilt die Standbild-Regel (`tick_.reset()` nicht nötig: `shown` mit der Startzeit). |

- [ ] **Step 1: Failing E2E-Tests** (QUAD, Uhr über `withClock(dev, {clockMode:'set', clockFactor:1, clockTime:…})`; Pakete aus `ZZA_CLI export`; Testpakete als kleine Templates unter `e2e/fixtures/` nach dem Muster der vorhandenen):
  - `API-TPL-020` Standbild `HH:mm`: `clock` = `minute`; nach `set 10:08:50` ist der Screenshot 5 s lang unverändert, spätestens 12 s später gleich dem Referenzbild `--time …T10:09:00` (`render --zza`).
  - `API-TPL-021` Standbild `HH:mm:ss`: `clock` = `second`; zwei Screenshots im Abstand von 2,5 s unterscheiden sich; nach `set 12:00:00` zeigt der Screenshot binnen 2 s eine Referenz aus 12:00:00…12:00:02.
  - `API-TPL-022` Faktor 10, `HH:mm`: nach `set 10:08:00` Minute 09 nach ≤ 8 s.
  - `API-TPL-023` `umuc-105-v1`: `mode` = `live`, `clock` = `second`; nach `set 11:59:55` und 10 s Warten gleicht der Screenshot einer Referenz aus 12:00:03…12:00:08 (Minutenzeiger auf 12).
  - `API-TPL-024` Template ohne `now`: `clock` = `none`, Screenshot über 5 s stabil (wie bisher).
  - `API-TPL-025` Split-Flap mit `HH:mm` im Endbild: nach dem Ende der Animation folgt das Bild dem Minutenwechsel.
- [ ] **Step 2:** Tests gegen die Firmware aus Task 6 laufen lassen (`ZZA_CLI=… node run.ts --quad $QUAD --yes`, siehe `e2e/README.md`) → 020–023 und 025 FAIL.
- [ ] **Step 3:** Umsetzen. `publish()`/Status um `clock` ergänzen.
- [ ] **Step 4:** Flashen (QUAD Release, 10 s Pausen), E2E `API-TPL-*` und `API-CLK-*` → PASS; `npm run unit` und `npm run catalog` in `e2e/`.
- [ ] **Step 5:** `docs/API.md` (Abschnitt „Templates installieren“: Feld `clock`, Verhalten der Uhr, Zufallszeit gestrichen), OpenAPI beide Kopien.
- [ ] **Step 6:** Commit `feat(tpl): templates follow the device clock`.

### Task 8: Manueller Testplan und Abschluss

**Files:**
- Create (Generator): `docs/superpowers/plans/2026-11-20-template-clock-manual-tests.md`
- Modify (Firmware): `e2e/manual/checklist.md` (`MAN-TPL-005`, `MAN-TPL-006`), `RELEASE_NOTES.md`

- [ ] **Step 1:** Manuelle Tests schreiben: `MAN-TPL-005` Sekundenzeiger (`umuc`) 5 Minuten beobachten – keine doppelte oder übersprungene Sekunde, Minutenzeiger springt bei :00; `MAN-TPL-006` beide Gleise des QUAD mit Sekundenuhr – Weboberfläche, Taster und Zugwechsel reagieren flüssig; dazu Modus `ntp` (Sprung von 0:00 auf die echte Zeit nach WLAN), Faktor 4 und 100, Server: GIF-Vorschau von `umuc`/`oebb` läuft über den Minutenwechsel.
- [ ] **Step 2:** Generator: `go test ./...`, Server-Import-Prüfung, `tools/sync-zzarender.sh --check ../Zugzielanzeiger-tpl`. Firmware: `tools/test-native.sh`, beide Release-Builds.
- [ ] **Step 3:** security-reviewer und code-reviewer auf beide Branch-Diffs (Workflow `CLAUDE.md` Schritt 5); Befunde beheben.
- [ ] **Step 4:** Commit in beiden Repos; Stand melden (kein Merge nach `develop`).
