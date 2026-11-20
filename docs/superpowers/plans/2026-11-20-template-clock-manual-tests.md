# Uhrzeit in Templates (laufendes `now`) — Manueller Testplan

Spec: `docs/superpowers/specs/2026-11-20-template-clock-design.md`. Alle Punkte vor einem Merge von
`feature/zza-on-device-rendering` → `develop` durchgehen (Firmware: `feature/zza-template-render`). Bei einem fehlgeschlagenen Punkt nicht weitermachen.

**Stand:** Nichts davon ist gemerged, gepusht oder deployed. Die Geräte-Tests brauchen ein QUAD (1,05"), für Punkt 5 zusätzlich ein DUAL.

## Vorbereitung

- Generator: `export CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"`, Server lokal:
  `PORT=18080 TEMPLATES_DIR=./templates CACHE_DIR=/tmp/zza-manual/cache go run ./cmd/zza-server serve`; `export B=http://localhost:18080`.
- Pakete neu exportieren (`zza export`, Format unverändert, aber das Manifest enthält jetzt `clock`): `umuc-105-v1`, `nederland-105-v1`, `oebb-105-v1`.
  `"clock"` im `manifest.json` prüfen: umuc und nederland `second`, oebb `second` (HH:mm:ss). Alte Pakete ohne Feld gelten als `none`.
- Firmware: Release-Build (`ZZA_Universal_Display_Controller_105_QUAD_Release`) flashen, Pakete hochladen, Gleis A auf `umuc-105-v1`.
- Uhr stellen: Controller-Einstellungen „Uhr“ (Modus `ntp` oder `set`, Faktor 0,1–100) oder `GET /api/clock/set?t=HH:MM[:SS][&factor=x]`.
  Ohne Internetzeit läuft die Uhr ab 0:00.

## 1. Server: GIF-Vorschau über den Minutenwechsel

- [ ] `umuc-105-v1/preview?format=gif` etwa bei Sekunde 30 einer Minute aufrufen (das GIF beginnt beim Rendern, also kurz vor der vollen Minute). Der Minutenzeiger springt im GIF beim Übergang auf die nächste Minute, der Sekundenzeiger läuft bei :00 weiter (kein Verharren, keine doppelte Geschwindigkeit).
- [ ] `oebb-105-v1/preview?format=gif`: die digitale Uhr zählt HH:mm:ss, springt bei :59 → :00 sauber auf die nächste Minute.
- [ ] PNG (`…/preview`) = Frame 0 des GIF. GIF ohne `now` (z. B. `faltblatt-105-v1`) unverändert gegenüber vorher (bytegleich).
- [ ] Templates und Server-Version werden zusammen deployt: alte Templates auf neuem Server laufen doppelt so schnell, neue auf altem Server stehen.

## 2. Standbild und Sekundenzeiger (MAN-TPL-005)

- [ ] Uhr `set`, Faktor 1, `umuc-105-v1`, Zugwechsel. 5 Minuten beobachten: keine doppelte oder übersprungene Sekunde, Minutenzeiger springt bei :00, Stundenzeiger läuft mit.
- [ ] `/templateStatus`: `clock: "second"`, `mode: "live"`, `clockOff: false`; `clockFull` (Zähler ganzer Bild-Updates) bleibt auf dem QUAD mit den mitgelieferten Templates 0 und wächst nicht mit jeder Sekunde.
- [ ] Standbild-Template mit nur `HH:mm` (Minutenuhr, Fixture `e2e/fixtures/zza/e2e-clock-min` im Firmware-Repo): Anzeige ändert sich nur beim Minutenwechsel, `clock: "minute"`. Gegenprobe mit `e2e/fixtures/zza/e2e-clock-sec` (`HH:mm:ss`): ändert sich jede Sekunde, `clock: "second"`.
- [ ] Faktor 4 im Modus `set`, dann auf `ntp` umstellen: Sekundenzeiger (`umuc-105-v1`) läuft weiter ohne doppelte/übersprungene Sekunde.
- [ ] Uhr stellen (`/api/clock/set?t=11:59:55`): nach 10 s zeigt die Anzeige 12:00:05. Die Anzeige reagiert innerhalb eines Takts.

## 3. Uhrmodi

- [ ] Modus `ntp` ohne Internet: Uhr läuft ab 0:00, Template zeigt das. Nach WLAN/Internet springt die Anzeige auf die echte Zeit.
- [ ] Faktor 4: der Sekundenzeiger läuft viermal so schnell.
- [ ] Faktor 100: es wird höchstens alle 100 ms gezeichnet, Sekunden werden übersprungen; Weboberfläche und Taster bleiben bedienbar.
  Grenzen (Spec „Schnelle Modellbahnuhr“, „Zu langsam“): Uhr-Updates belegen höchstens 25 % der Zeit (Abstand ≥ 100 ms und ≥ 4 × letzte Update-Dauer); drei Uhr-Updates in Folge > 500 ms (oder eines > 1,5 s) halten das Bild an (`clockOff: true` bis zu neuen Zugdaten), einzelne Ausreißer nicht; drei Uhr-Frames in Folge > 500 ms oder > 50 % ihrer Frame-Dauer brechen die Animation ab, das Gleis zeigt das Standbild der Uhr (`mode: "still"`).
- [ ] Faktor 0,1: die Uhr läuft zehnmal langsamer, kein Flackern.

## 4. Beide Gleise (MAN-TPL-006, QUAD)

- [ ] Gleis A `umuc-105-v1`, Gleis B `oebb-105-v1` (oder `nederland-105-v1`), Uhr `set`, Faktor 1. Beide Uhren laufen gleichmäßig.
- [ ] Messwerte (QUAD): umuc-Frame etwa 190–200 ms (unter 50 % der 1-s-Frame-Dauer, bleibt live), Standbild-Update 65–192 ms (unter 500 ms): ______ ms / ______ ms.
- [ ] Während beider Uhren: Weboberfläche bedienen, Taster (BTN 0 kurz und lang), Zugwechsel auf beiden Gleisen. Alles reagiert flüssig, kein Flackern, keine Farbblöcke.
- [ ] `/templateStatus` je Gleis: `clock`, `clockFull` (bleibt 0) und `clockOff: false`.

## 5. DUAL

- [ ] DUAL-Release, `umuc-105-v1` auf Gleis A: obere und untere Hälfte zeigen dieselbe Zeit, ohne Versatz, Sekundenzeiger läuft über beide Displays.

## 6. Abwärtsverhalten

- [ ] Paket ohne `clock` im Manifest: `clock: "none"`, Template verhält sich wie vorher (keine laufende Uhr, kein Fehler).
- [ ] Split-Flap-Template mit Uhr (Fixture `e2e/fixtures/zza/e2e-clock-flap` im Firmware-Repo): Zeit beim Start eingefroren, Animation wie bisher, Endbild folgt der Standbild-Regel.
