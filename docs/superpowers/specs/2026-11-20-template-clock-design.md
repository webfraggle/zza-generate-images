# Uhrzeit in Templates: laufendes `now` – Entwurf

Stand 2026-11-20. Generator: Branch `feature/zza-on-device-rendering`. Firmware: Branch
`feature/zza-template-render` (Worktree `../Zugzielanzeiger-tpl`, `develop` mit der Uhr ist gemergt, `2bb1e48`).
Das ist Stufe 5 aus `2026-11-16-zza-on-device-rendering-design.md` (§ „Später: NTP/RTC, Live-Uhr“).

## Wozu

Die Firmware hat seit 3.2.17-beta eine geräteweite Uhr (`zzaClock`: Internetzeit oder gestellte Zeit mit
Faktor). Templates auf dem Gerät rendern bisher mit einer Zufallszeit. Sie sollen die Uhr des Geräts zeigen –
laufend, nicht nur beim Zugwechsel. Das GIF vom Server soll bleiben, was es ist: eine Minute im Voraus ab einer
Startzeit.

## Das Problem heute

`now` ist für alle Frames einer Animation eingefroren. Uhr-Templates lassen die Zeit selbst laufen
(`now.second | add(anim.frame)`, `format('HH:mm:ss', anim.time)`). Folgen:

- Gibt das Gerät je Frame die echte Zeit hinein, zählen diese Templates doppelt.
- Minuten- und Stundenzeiger stehen die ganze Schleife lang still (auch im GIF).
- Ein Standbild mit Uhrzeit bleibt bis zum nächsten Zugwechsel stehen.

## Ziele

1. Ein Template beschreibt die Uhr einmal (`now.second`, `format('HH:mm:ss')`) und läuft damit im GIF und auf dem
   Gerät richtig.
2. GIF (Server, Vorschau, ZZA-Vorschau): Frame *i* zeigt Startzeit + *i* × `frame_delay`.
3. Gerät: jedes Bild zeigt die Zeit von `zzaClock` in dem Moment, in dem es gerendert wird.
4. Gerät: ein Standbild mit Uhrzeit aktualisiert sich selbst – jede Minute, oder jede Sekunde, wenn Sekunden
   darin vorkommen (User-Entscheidung 2026-11-20).

## Nicht-Ziele

- Züge nach der Uhr wechseln (`Mode_TimeSim`).
- Datum in der Uhr der Firmware (`Clock` kennt nur Stunde, Minute, Sekunde).
- Änderungen an `Clock` selbst.

## Anti-Ziele

- Ein zweiter Satz Templates für das Gerät.
- Ein Gerät, das wegen einer Uhr dauerhaft die Schleife blockiert (Taster, DCC, Web müssen flüssig bleiben).

## Ansätze

1. **`now` läuft in der Animation mit (gewählt, User 2026-11-20).** Der Aufrufer bestimmt die Zeit je Frame:
   GIF = Start + i × `frame_delay`, Gerät = `zzaClock`. Templates werden einfacher, der Minutenzeiger springt
   auch im GIF richtig weiter. Preis: Bedeutung von `now` ändert sich in Go-Renderer, C++-Kern und Spec; die drei
   Uhr-Templates werden angepasst.
2. Templates bleiben, das Gerät rendert bei jedem Takt „Frame 0“ mit frischer Zeit. Weniger Umbau, aber alles,
   was `anim.frame` für anderes nutzt (blinkender Doppelpunkt), steht auf dem Gerät still.
3. Gerät gibt `Uhr − anim.time` hinein, damit die alte Rechnung aufgeht. Kein Umbau am Kern, aber Minuten- und
   Stundenzeiger hinken bis zu einer Schleifenlänge nach.

## Entwurf

### Bedeutung von `now` (Template-Spec)

| Fall | `now` |
|---|---|
| PNG, Vorschau-Standbild | Zeitpunkt des Renderns (= Frame 0 des GIF, wie bisher) |
| GIF im Frame-Modus | Start + `anim.frame` × `frame_delay`, je Frame neu (Datum läuft über Mitternacht mit) |
| GIF mit Split-Flap | eingefroren beim Start (wie bisher, 0,5–5 s) |
| Gerät | Uhr des Geräts beim Rendern des Bildes (siehe unten) |

`anim.frame`, `anim.frames`, `anim.time` und `format('…', sek)` bleiben unverändert erhalten. Für Uhren braucht
man den Versatz nicht mehr; Spec und `docs/ai-design-guide.md` zeigen nur noch die neue Schreibweise und warnen
vor der alten (doppelte Geschwindigkeit).

Die Zusage „PNG = Frame 0“ und „bytegleiches GIF ohne `now`“ gelten weiter.

### Generator

- **Go-Renderer** (`internal/renderer`): im Frame-Modus bekommt jeder Frame einen Evaluator mit
  Start + i × `frame_delay` (in `meta.timezone`). Split-Flap bleibt eingefroren.
- **C++-Kern** (`device/zzarender`): `zza_anim_frame` bekommt einen Parameter `const zza_time* now`.
  `NULL` = der Kern rechnet selbst wie Go (Frame-Modus: Start + i × `frame_delay` mit Datumsübertrag; Split-Flap:
  Startzeit). Nicht `NULL` = diese Zeit gilt für den Frame (nur Frame-Modus; bei Split-Flap ignoriert).
  Go-Brücke, ZZA-Vorschau und MCP-Gerätecheck rufen mit `NULL`. Paritätstests Go ↔ C++ decken den Übertrag
  Sekunde → Minute → Stunde → Tag ab.
- **Export** (`internal/devexport`): `manifest.json` bekommt das Feld `"clock"`: `"none"`, `"minute"` oder
  `"second"`. Bestimmt aus den Ausdrücken, die `UsesNow` schon findet:
  - `second`: `now.second` oder ein `format`-Muster mit Sekunden, oder ein `format` mit Versatz, der kein
    Zahlen-Literal aus ganzen Minuten ist (`format('HH:mm', 90)`, `format('HH:mm', anim.time)`: die Minute
    wechselt dann nicht auf der Minutengrenze; `format('HH:mm', 120)` bleibt `minute`);
  - `minute`: jede andere Verwendung von `now` (auch nur Datum);
  - `none`: kein `now`.
  Lässt sich ein Muster nicht statisch lesen (Muster aus einer Variablen), gilt `second`.
  Die Formatnummer des Pakets bleibt. Fehlt das Feld, gilt `none`. Alte Pakete werden nicht berücksichtigt
  (User 2026-11-20: es gibt noch keine außerhalb der Entwicklung); sie werden neu exportiert.
- **Templates**: `umuc-105-v1`, `nederland-105-v1` (`now.second | mul(6)`), `oebb-105-v1`
  (`format('HH:mm:ss')`). Änderung über den zza-MCP (`write_file`).
- **Doku**: `docs/yaml-template-spec.md` (Abschnitte Frame-Animation, Variablen, Determinismus),
  `docs/ai-design-guide.md`, On-Device-Spec §„Uhrzeit“ verweist hierher.
- Danach `tools/sync-zzarender.sh` in die Firmware.

### Firmware (`DisplayTemplate`, `tpl/TplAdapter`)

Zeitquelle: `tpl::deviceTime()` ersetzt `randomTime()`. Stunde/Minute/Sekunde aus `zzaClock.now()`. Datum und
Zeitzone bleiben vorerst außen vor (User 2026-11-20): das Datum ist fest 1.1.2026 wie bisher, die Datumsfelder
und `format` mit Datum funktionieren im Kern unverändert weiter. `meta.timezone` des Templates gilt auf dem
Gerät nicht; es zählt, was die Geräte-Uhr liefert. Ohne Internetzeit läuft die Uhr ab 0:00 (so will es die
Uhr-Spec), das Template zeigt das an.

Verhalten je Paket (`clock` aus dem Manifest):

| Bild | `none` | `minute` / `second` |
|---|---|---|
| Standbild | wie bisher | rendert neu, sobald sich Minute bzw. Sekunde der Geräte-Uhr ändert |
| Frame-Modus | wie bisher (live / vorgerendert) | immer **live**, nie vorgerendert; Takt = `frame_delay` in Echtzeit, `now` = Geräte-Uhr je Frame; `anim.frame` zählt weiter |
| Split-Flap | wie bisher | Zeit beim Start eingefroren, Animation wie bisher (auch vorgerendert) |
| Nach dem Ende einer Animation (Split-Flap, Frame-Modus ohne `loop`) | Endbild steht | Endbild folgt der Standbild-Regel |

Regeln dazu:

- **Nur Geändertes zeichnen.** Updates nutzen die Dirty-Rechtecke des Kerns (wie Live-Frames), kein voller
  Push je Sekunde. Wie das Standbild-Update im Kern läuft (z. B. als Animation mit einem Frame und `now` je
  Aufruf), bleibt dem Umsetzenden überlassen.
- **Takt an der Uhr ausrichten.** Bei Faktor 1 darf ein Sekundenzeiger keine Sekunde doppelt zeigen oder
  überspringen. Standbilder lösen an der Änderung der Uhr aus; im Frame-Modus mit `frame_delay` 1 s richtet der
  Adapter den Start so aus, dass Frames nicht auf die Sekundengrenze fallen.
- **Lastgrenzen: drosseln statt zurückfallen** (Nutzerentscheidung 2026-11-21, ersetzt die Fassungen der
  Security-Review 2026-11-20 und des Gerätelaufs 2026-11-20: „500 ms / 50 % des Takts, drei in Folge → Rückfall“).
  Das Template sichtbar zu halten ist wichtiger als ruckelfreier Lauf: zu lange Renderzeit lässt es ruckeln bzw.
  den Sekundenzeiger Sekunden überspringen; das eingebaute Layout gibt es aus Kostengründen nicht.
  - **Eigenkosten** statt Wanduhr-Zeit: Rechenzeit des Kern-Aufrufs (`zza_render_diff`/`zza_render`/
    `zza_anim_frame`), höchstens 2 × Rechenaufwand (`zza_work_used`) × gelernte Zeit je Arbeitsschritt (der
    Aufruf selbst wird auf dem Gerät verdrängt: ~68 ms Rechenzeit maßen bis 1,2 s), plus geschätzte Zeichenzeit
    (gelernte ns je Pixel × gezeichnete Pixel). Wartezeiten
    (Gleis-Mutex, Webserver, Flash-Schreiben, das andere Gleis) gehen nicht ein; Zeichen-Messungen mit mehr als
    dem 4-Fachen der Schätzung lernt die Schätzung nicht. (Gerätelauf 2026-11-20: e2e-clock-sec Wanduhr ≈ 56 ms
    mit Ausreißern bis 775 ms bei gleichbleibender Rechenzeit ≈ 36 ms.)
  - **Atempause:** nach einem Standbild-Update folgt das nächste frühestens nach max(100 ms, 4 × Eigenkosten)
    (höchstens 25 % der Zeit); nach einem Uhr-Frame bekommt die Schleife eine Pause von 4 × Eigenkosten (ohne
    Untergrenze, schnelle billige Animationen behalten ihr Tempo). Ein in die Pause fallender Frame wartet; der
    feste Takt (`tpl::Player`, `fixedRate`) lässt die verpassten Takte aus, kein Aufholen, kein Stau. Der
    Frame-Index zählt um eins weiter (lückenlose Frames, die Dirty-Rechtecke bleiben gültig), `now` ist die
    Geräte-Uhr beim Zeichnen; bei Faktor > 1 werden ohnehin Sekunden übersprungen. `throttled` zählt, wie oft
    ein Update/Frame auf die Pause warten musste.
  - **Harte Grenze nur für krankhafte Eigenkosten:** mehr als 1,5 s, dreimal in Folge (`tpl::CostStreak`) →
    eine Uhr-Animation zeigt das Uhr-Standbild, ein Standbild setzt seine Updates aus (`clockOff`, das Bild
    bleibt stehen; nach 60 s neuer Versuch, wirkt wie ein Minutentakt). Die Arbeitsgrenze je Kern-Aufruf und je
    Animationsdurchlauf bleibt (deterministisch, keine Wanduhr-Zeit).
  - **Kein Rückfall ist endgültig:** jeder Laufzeit-Rückfall (Uhr-Standbild, `clockOff`, eingebautes Layout nach
    Render- oder Speicherfehler, auch nach fehlgeschlagenem Vollbild-Fallback) wird nach 60 s erneut versucht,
    höchstens ein Versuch (ein Laden/Rendern) je 60 s; `retryMs` zeigt die Restzeit. Speichern der
    Gleis-Einstellungen und Installieren des Pakets setzen sofort zurück. Dauerhaft beim eingebauten Layout bleibt
    nur ein Paket, das sich nicht laden lässt (fehlt oder fehlerhaft), bis es neu installiert wird.
  - Kein Speicher für den Vergleich (`zza_render_diff` NOMEM) → ganzes Bild neu; jedes Update versucht den
    Vergleich erneut. Schlägt auch das ganze Bild fehl, zeigt das Gleis wie bei einem fehlgeschlagenen Standbild
    das eingebaute Layout (die genullte Leinwand wird nie gezeichnet) und versucht es nach 60 s erneut.
- **Uhr wird gestellt / NTP synchronisiert.** Die Uhr springt; das nächste Update zeigt die neue Zeit. Kein
  Sonderfall nötig, weil jedes Update die Uhr neu liest.
- **Zugwechsel.** Keine Zufallszeit mehr. Die Abkürzung „gleiche Daten → nur neu zeichnen“ bleibt.
- **Zwei Gleise.** Beide dürfen gleichzeitig eine Uhr zeigen; jedes Gleis höchstens ein Bild je `service()` wie
  bisher.
- `/templateStatus` je Gleis: zusätzlich `"clock":"none|minute|second"`, `"clockFull":n` (Uhr-Updates als ganzes
  Bild seit dem Laden), `"clockOff":true|false` (Uhr-Updates ausgesetzt), `"throttled":n` (auf die Lastpause
  wartende Updates/Frames des Bildes) und `"retryMs":n` (Restzeit bis zum Neuversuch, 0 = keiner).

Bekannte Grenzen:

- GIF der ZZA-Vorschau und GIF des Servers können sich um eine Stunde unterscheiden, wenn innerhalb der
  Animation eine Sommer-/Winterzeit-Umstellung liegt: der Kern addiert Wanduhr-Sekunden, Go eine absolute Dauer.
- Das Endbild einer Frame-Animation ohne `loop` mit Uhr steht, wenn der Kern dabei keinen Speicher bekommt (NOMEM
  beendet die Sitzung); der Neuversuch nach 60 s zeigt das Template neu, spielt die Animation also noch einmal ab.
- Während Flash-Schreibvorgängen (Paket-Installation, Einstellungen) steht die Rechenzeit selbst still (Cache aus):
  die Eigenkosten einzelner Updates steigen dann, die Uhr wird gedrosselt; erst drei Bilder über 1,5 s in Folge
  setzen aus (und kommen nach 60 s zurück).
- Rückfälle von Animationen ohne Uhr (Faltblatt zu langsam oder ohne Speicher → Endbild) werden nicht erneut
  versucht: das Endbild ist das Bild, das ohnehin stehen bliebe.

## Tests

- Go: Frame-Modus-GIF mit festem Start – Frame i zeigt Start + i × delay, Übertrag 23:59:30 → 00:00:29,
  Split-Flap eingefroren, PNG = Frame 0. Export: `clock` für die drei Fälle und für die ausgelieferten Templates.
- Kern: Parität Go ↔ C++ für `now = NULL`; `now` je Frame überschreibt.
- Firmware E2E (Uhr im Modus `set`, damit die Tests die Zeit bestimmen): Standbild mit `HH:mm` ändert sich nach
  dem Minutenwechsel und nicht vorher; mit Sekunden jede Sekunde; Faktor 10 läuft entsprechend schneller;
  Uhr stellen wirkt innerhalb eines Takts; Frame-Uhr (`umuc`) zeigt nach `set 11:59:55` nach 10 s Minute 0 der
  nächsten Stunde; `mode` ist `live` für Frame-Modus mit Uhr.
- Manuell: Sekundenzeiger über 5 Minuten ohne Ruckeln/Doppelsekunde; Web-Oberfläche bleibt bedienbar, während
  beide Gleise eine Sekundenuhr zeigen.

## Auslieferung

Reihenfolge am Server: Templates und Server-Version zusammen aktualisieren (Templates liegen auf der VM als
Bind Mount). Neue Templates auf altem Server: Sekundenzeiger steht. Alte Templates auf neuem Server: doppelte
Geschwindigkeit. Pakete auf den Geräten werden neu exportiert und eingespielt.

## Dem Umsetzenden überlassen

Signaturen im Kern über `zza_anim_frame` hinaus, Aufbau der Update-Schleife in `DisplayTemplate` (neue Phase
oder Erweiterung von `PH_LIVE`), Erkennung der Sekunden in `format`-Mustern, Toleranzen der Zeittests.
