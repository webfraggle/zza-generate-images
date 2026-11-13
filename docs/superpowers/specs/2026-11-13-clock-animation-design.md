# Generische Frame-Animation + laufende Uhr in `umuc-105-v1` — Design

Datum: 2026-11-13 · Status: Entwurf zur Freigabe · Branch: `feature/clock-animation`

Legende: **[C]** = von Christoph so entschieden · **[P]** = Entscheidung des Planers (darf Christoph kippen) · **[B]** = dem Umsetzer überlassen

## 1. Absicht und Warum

`umuc-105-v1` (U-Bahn München, 240×240) zeigt eine analoge Bahnhofsuhr. Heute ist sie falsch (Stundenzeiger im 24-h-Takt, ohne Minutenanteil), friert im Server-Cache ein (der Cache-Key kennt die Zeit nicht) und läuft in der Zeitzone des Servers statt `Europe/Berlin`. Die Spalte „Min." zeigt die Abfahrtszeit („10:15") statt der Minuten bis zur Abfahrt wie im PHP-Original (5 / 8 / 16).

Christoph möchte die Uhr wie im Original (`legacy/umuc-105-v1/index.php`) und zusätzlich einen dünnen roten Sekundenzeiger, der im GIF läuft. **[C]** Leitlinie: **so allgemein wie möglich**. Keine Uhr-Sonderlogik im Renderer, sondern Bausteine, die jedes Template nutzen kann:

- ein **Frame-Modus** für Animationen (N Frames, fester Abstand, optional Endlosschleife),
- **Animationsvariablen** (`anim.frame`, …),
- **Ausdrücke mit mehreren Variablen** in Filter-Pipelines,
- eine feste **Zeitzone** pro Template,
- ein Cache, der zeitabhängige Templates erkennt.

## 2. Ziele

1. **[C]** `umuc-105-v1` rendert als GIF 60 Frames à 1 s und läuft dann endlos weiter. Stunden- und Minutenzeiger stehen über das ganze GIF fest (Zeit des Render-Starts). Nur der Sekundenzeiger läuft, beginnend bei der aktuellen Sekunde. Nach 60 Frames ist er wieder an der Startposition, die Schleife springt also nicht.
2. **[C]** Das PNG von `umuc-105-v1` zeigt alle drei Zeiger zur aktuellen Zeit (Sekundenzeiger = aktuelle Sekunde) und ist identisch mit Frame 0 des GIFs.
3. **[C]** Stundenzeiger wie im PHP: `(Stunde mod 12 + Minute/60) · 30°`. Minutenzeiger: `Minute · 6°`.
4. Die Spalte „Min." zeigt wie im Legacy-Original `minutes(zugN.zeit) − minutes(zug1.zeit) + 5`, also 5 / 8 / 16 bei `default.json`. Über Mitternacht bleibt der Wert positiv (23:58 → 00:04 = 11).
5. **[C]** Die Bausteine sind generisch: Frame-Modus, `anim.*`, Variablen als Filter-Argumente, `mod`, `minutes`, `meta.timezone` und die Cache-Erkennung kann jedes Template nutzen. Keine Uhr- oder Template-Sonderfälle in Go.
6. Zeitabhängige Templates (alle, die `now` oder `format(…)` benutzen) werden nicht mehr aus dem Cache bedient. Das behebt eingefrorene Uhren auch in `nederland-*`, `oebb-*` und `umuc-096-v1`.
7. `umuc-105-v1` wird nach dem Muster von `faltblatt-105-v1` aufgeräumt: Header-Kommentar zum Canvas-Aufbau, korrekte Kommentare ohne PHP-Zeilenverweise, Text-Fallback für Nicht-U-Linien in allen drei Zeilen und `meta.animation`.
8. Die Detailseite und der Editor bieten den GIF-Umschalter für jedes animierte Template an, nicht nur für `splitflap`.

## 3. Nicht-Ziele

- `umuc-096-v1` anfassen. **[C]** Auch dort ist der Stundenzeiger falsch (24 h), ebenso in `nederland-096-v1`. Beides kommt ins Backlog.
- Kein neuer Layer-Typ, also weder `line` noch `clock`. Der Sekundenzeiger ist ein PNG. **[C]**
- Keine Infix-Arithmetik (`a + b * 6`) in String-Feldern (siehe §5).
- Variablen als Argument nur für die numerischen Filter (`add`, `sub`, `mul`, `div`, `mod`). Text-Filter (`strip`, `prefix`, …) behalten reine Literal-Argumente.
- Kein `file`-Interpolieren. Die 24 Badge-Layer bleiben, aus Sicherheitsgründen.
- Frame-Modus und `splitflap` lassen sich nicht im selben Template kombinieren.
- Keine Optimierung „statische Layer nur einmal rendern". Jeder Frame durchläuft alle Layer, wie im Splitflap-Modus.
- Firmware bzw. ESP32-Wiedergabe von Loop-GIFs.

## 4. Anti-Ziele (wäre ein Fehlschlag, auch wenn es „funktioniert")

- Bestehende Templates rendern anders. Jede Interpolation in `templates/*/template.yaml` ergibt mit jeder `default.json` bei fester Zeit dasselbe wie vorher. Ein Kompatibilitätstest mit eingefrorener Kopie des alten Evaluators sichert das ab. Erlaubte, gewollte Abweichungen:
  - `now` läuft in `Europe/Berlin` statt in der Server-Zeitzone,
  - Drehachse von `rotate` ±0,5 px (§11).
- `splitflap`-GIFs ändern sich (Bytes, Timing, Loop-Verhalten ohne `loop: true`).
- Uhr-Sonderlogik in Go (Winkelvariablen, `clock`-Layer, Template-Namen im Code).
- Mehrdeutige Syntax: ein Ausdruck, der vorher etwas Sinnvolles ergab, bedeutet jetzt etwas anderes.
- Unbegrenzte Renderkosten über die öffentliche Render-Route.

## 5. Ansätze

### 5.1 Mehrere Variablen in einem Ausdruck (Punkt 3 der Richtung)

| | Ansatz | Bewertung |
|---|---|---|
| **A** | **Numerische Filter akzeptieren als Argument eine Variable oder eine Unter-Pipeline:** `{{now.second \| add(anim.frame) \| mul(6)}}`, `{{zug2.zeit \| minutes \| sub(zug1.zeit \| minutes)}}` | Fügt sich in die vorhandene Pipeline-Syntax ein, die Node-Editor und Doku schon kennen. Eindeutig: Ein Argument ist ein Literal (in Anführungszeichen oder als Zahl) oder ein Ausdruck. Bestehende Templates nutzen nur Zahlen-Literale als Argumente und laufen deshalb unverändert. Ein kleiner Parser (Klammer- und Anführungszeichen-bewusstes Splitten) genügt. |
| B | Echte Infix-Arithmetik in `{{ }}`: `{{(now.second + anim.frame) * 6}}` | Lesbarer für Formeln. In String-Feldern aber mehrdeutig: Datenpfade können `-` enthalten (JSON-Schlüssel wie `zug-1`), und `/` bzw. `*` kommen in Texten vor. Man bräuchte Heuristiken („ist das eine Formel?"), die bestehende Ausdrücke anders lesen könnten. Zwei Ausdruckssprachen nebeneinander (Pipeline und Infix) machen Doku und Node-Editor komplizierter. Die Stunden-mit-Minuten-Formel braucht trotzdem Filter. |
| C | Zusätzliche fertige Variablen (`now.hour_angle`, `now.minute_of_day`) bzw. ein `clock`-Layer | Uhr-Sonderlogik, die Christoph ausdrücklich nicht will. Hilft der „Min."-Spalte nicht. |

**Gewählt: A [P].** Das ist die allgemeinere Lösung ohne Mehrdeutigkeit. Mit ihr lassen sich Stundenzeiger, Sekundenzeiger und Minutenspalte ausdrücken, ohne Uhr-Variablen. Die Unter-Pipeline als Argument (statt nur einer nackten Variablen) braucht man für `sub(zug1.zeit | minutes)`. Doppelte Klammern wie `sub((…))` sind nicht nötig: Das Argument wird an `,` und `)` auf oberster Ebene begrenzt, `|` innerhalb der Klammern gehört zum Argument.

### 5.2 Frame-Modus (Punkt 1 der Richtung)

| | Ansatz | Bewertung |
|---|---|---|
| **A** | **Neue Felder in `meta.animation`:** `frames`, `frame_delay`, `loop` | Ein Ort für alle Animationseinstellungen. Schließt sich mit `fps`/`duration` gegenseitig aus, Widersprüche fallen beim Laden auf. `loop` gilt für beide Modi. |
| B | Eigener Block `meta.frames: {count, delay}` | Zwei Orte für Animation. Die Regeln, wann was gilt, werden unklarer. |
| C | Frame-Modus aus `fps`/`duration` ableiten (z.B. `fps: 1, duration: 60`) | Kollidiert mit den Splitflap-Grenzen (fps 5–25, duration ≤ 5) und deren Bedeutung („maximale Laufzeit, Klappen bestimmen das Ende"). Ein Halte-Frame am Ende ist im Loop falsch. |

**Gewählt: A [P].**

### 5.3 Abweichungen von der Orchestrator-Richtung (begründet)

1. **Bedingungen:** Das erste Argument einer Bedingungsfunktion darf ebenfalls eine Pipeline sein (`greaterThan(zug1.zeit | minutes, 600)`). Für einfache Pfade ist das Ergebnis identisch. Der Aufwand ist eine Zeile, weil derselbe Pipeline-Auswerter genutzt wird. **[P]**
2. **Drehachse von `rotate` korrigiert (§11):** `rotateImageCW` liefert bei manchen Winkeln eine Bounding-Box ungerader Größe. Dann liegt die Drehachse 0,5 px daneben, und ein dünner Sekundenzeiger würde von Sekunde zu Sekunde sichtbar zittern. Der Fix ist generisch (gerade bzw. ungerade Größe wie das Quellbild) und verschiebt gedrehte Bilder anderer Templates um höchstens 0,5 px. **[P]**
3. **Zeiger-PNGs um 1 px verschoben:** `clock-hour.png` und `clock-minutes.png` haben ihre Zeigermitte bei x = 36,0 statt in der Bildmitte 35,0. Die Uhr steht dafür bei `x: 38`. Gemessen: Das Zifferblatt in `bg.png` hat seine Mitte bei (39,0 | 56,0). Bei 0° sitzt der Zeiger damit richtig, bei 180° aber 2 px daneben. Korrektur: Den Inhalt beider PNGs verlustfrei um 1 px nach links verschieben und die Uhr auf `x: 39, y: 56` setzen. **[P]**
4. **Node-Editor:** `web/static/node-filters.js` muss angepasst werden, obwohl keine Layer-Felder dazukommen. Sein Filter-Parser splittet stur an `' | '` und würde `sub(zug1.zeit | minutes)` beim Zurückschreiben zerstören. **[P]**
5. **`anim.time`** wird zusätzlich zu `anim.frame` und `anim.frames` angeboten (Sekunden seit Animationsbeginn). `anim.progress` entfällt, weil es sich als `anim.frame | div(anim.frames)` ausdrücken lässt. **[P]**

## 6. Ausdrücke **[P]**

### 6.1 Grammatik innerhalb `{{ … }}`

```
pipeline := operand ( "|" filter )*
operand  := Pfad                       (wie bisher: zug1.zeit, now.second, anim.frame, …)
filter   := name [ "(" arg ( "," arg )* ")" ]
arg      := 'text' | "text" | Zahl | pipeline     (pipeline ohne {{ }})
```

- `|` und `,` trennen nur **auf oberster Ebene**, also außerhalb von `'…'`/`"…"` und außerhalb von `( … )`. Das korrigiert nebenbei einen alten Fehler: `stripAll('|')` wurde bisher am `|` im Literal zerschnitten.
- Leerzeichen um `|` sind optional (wie bisher).
- **Zahl** = alles, was `strconv.ParseFloat` nach `TrimSpace` akzeptiert (wie bisher, also auch `-6`, `+5`, `1e3`).
- Rekursionstiefe: höchstens `maxPipelineDepth = 4` verschachtelte Argument-Pipelines. Ist sie überschritten, ergibt die betroffene Pipeline `""`.

### 6.2 Argumente numerischer Filter (`add`, `sub`, `mul`, `div`, `mod`)

| Argument | Bedeutung | Ist das Argument keine Zahl … |
|---|---|---|
| `'6'` / `"6"` (in Anführungszeichen) | Literal (wie bisher) | … bleibt der Wert unverändert (wie bisher) |
| `6`, `-0.5` (Zahl) | Literal (wie bisher) | — |
| alles andere, z.B. `anim.frame`, `zug1.zeit \| minutes` | wird als Pipeline ausgewertet | … ist das **Ergebnis `""`** (neu) |

Ist der **Wert** (links) keine Zahl, bleibt er unverändert (wie bisher). Damit pflanzt sich ein leerer Wert durch die ganze Kette fort, und eine leere oder ungültige Zeit ergibt eine leere Zelle.

Verhaltensänderung nur für einen Fall, den kein Template nutzt: Ein Argument ohne Anführungszeichen, das keine Zahl ist (z.B. `mul(abc)`), ließ den Wert bisher unverändert. Jetzt wird es als Variable gelesen. Der Kompatibilitätstest (Plan Task 1) belegt, dass kein Template betroffen ist.

Text-Filter (`strip`, `stripAll`, `stripBetween`, `before`, `after`, `prefix`, `suffix`, `format`) bleiben bei Literal-Argumenten: Das Argument wird wie bisher nur „entquotet".

### 6.3 Neue Filter

| Filter | Wirkung | Beispiele |
|---|---|---|
| `mod(n)` | Modulo mit dem Vorzeichen des Teilers (floored, wie Python), Float-fähig. `n = 0` → `0` (analog `div`). | `-3 \| mod(1440)` → `1437`; `7 \| mod(3)` → `1`; `7.5 \| mod(2)` → `1.5`; `-7 \| mod(-3)` → `-1`; `5 \| mod(0)` → `0` |
| `minutes` | `"H:MM"`/`"HH:MM"` → Minuten seit Mitternacht. Regel wie PHP `timeToMinutes`: Nach `TrimSpace` muss der Wert mit `^(\d{1,2}):(\d{1,2})` beginnen (Rest egal), Ergebnis `h·60+m` ohne Bereichsprüfung. Passt das Muster nicht, ist das Ergebnis `""` (PHP: 0). | `10:15` → `615`; `0:04` → `4`; `23:58` → `1438`; `10:15 Uhr` → `615`; `10:5` → `605`; `""`, `ca. 10`, `1015` → `""` |

### 6.4 Bedingungen

Das erste Argument von `startsWith`, `endsWith`, `contains`, `isEmpty`, `eq`/`equals` und `greaterThan` wird als Pipeline ausgewertet: `greaterThan(zug1.zeit | minutes, 600)`. Ein einfacher Pfad ergibt dasselbe wie bisher. Die Argumente werden klammerbewusst an `,` getrennt. Das zweite Argument bleibt ein Literal.

### 6.5 Beispiele (verbindlich, mit Erwartungswerten in den Tests)

```yaml
# Stundenzeiger: 10:15 → 307.5 ; 12:00 → 360 ; 00:30 → 375 (≡ 15°)
rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"
# Sekundenzeiger: Start bei now.second, pro Frame +6°
rotate: "{{now.second | add(anim.frame) | mul(6)}}"
# Minuten bis Abfahrt relativ zu Zug 1 (+5 wie im Legacy): 10:15/10:18 → 8 ; 23:58/00:04 → 11
value: "{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"
```

`rotate` verträgt Winkel über 360° (Trigonometrie), `mod(360)` ist also nicht nötig.

## 7. Frame-Modus **[P]**

### 7.1 Schema

```yaml
meta:
  animation:
    frames: 60        # Frame-Modus: Anzahl Frames, 2–125
    frame_delay: 1    # Sekunden pro Frame, 0.02–10 in 0.01-Schritten; fehlt = 1
    loop: true        # GIF endlos wiederholen; fehlt = false (einmal abspielen)
```

| Feld | Typ | Modus | Default | Erlaubt |
|---|---|---|---|---|
| `fps` | int | splitflap | 20 | 5–25 (unverändert) |
| `duration` | float | splitflap | 2.5 | 0.5–5 (unverändert) |
| `frames` | int | frame | 0 = aus | 2–125 |
| `frame_delay` | float (s) | frame | 1 | 0.02–10, Vielfaches von 0.01 |
| `loop` | bool | beide | false | — |

### 7.2 Welcher Modus gilt (Validierung in `LoadTemplate`, Fehler = Template-Fehler)

1. `frames ≠ 0`: **Frame-Modus.** Fehler, wenn `frames` außerhalb 2–125, wenn zusätzlich `fps` oder `duration` gesetzt (≠ 0) sind, wenn das Template einen `splitflap`-Layer enthält (auch verschachtelt), wenn `frame_delay` außerhalb 0.02–10 oder kein Vielfaches von 0.01 ist, oder wenn `frames × frame_delay > 120` s.
2. `frames = 0` und `splitflap` vorhanden: **Splitflap-Modus** wie bisher. Ein gesetztes `frame_delay` ist ein Fehler.
3. Sonst: **nicht animiert.** Ein GIF hat 1 Frame wie bisher. Ein gesetztes `frame_delay` ist ein Fehler, `loop` wird ignoriert.

Fehlertexte (englisch, Präfix `renderer: LoadTemplate: `):

- `animation.frames must be between 2 and 125 (got %d)`
- `animation.frames cannot be combined with animation.fps or animation.duration`
- `animation.frames cannot be used in a template with splitflap layers`
- `animation.frame_delay requires animation.frames`
- `animation.frame_delay must be between 0.02 and 10 seconds in steps of 0.01 (got %g)`
- `animation.frames × animation.frame_delay must not exceed 120 seconds (got %g)`

`Template.IsAnimated()` = Frame-Modus oder `splitflap` vorhanden. Es ersetzt `HasSplitflap()` überall, wo „animiert?" gefragt ist. `HasSplitflap()` bleibt für die Splitflap-internen Prüfungen.

### 7.3 Variablen `anim.*`

| Variable | Frame-Modus, GIF | Frame-Modus, PNG | sonst (PNG, Splitflap-GIF) |
|---|---|---|---|
| `anim.frame` | 0 … frames−1 | 0 | 0 |
| `anim.frames` | `frames` | `frames` | 1 |
| `anim.time` | `frame × frame_delay` in s (`0`, `1`, … bzw. `0.3`) | 0 | 0 |

- `anim.*` gibt es in allen interpolierten Feldern und in Bedingungen. `anim.frame` und `anim.frames` gibt es zusätzlich in Koordinaten-Ausdrücken (`x: "{{anim.frame * 2}}"`), weil sie in `intVars` stehen.
- `anim` ist reserviert wie `now`: Ein JSON-Feld `anim` ist in Ausdrücken nicht erreichbar.
- Loop-Variablen (`withLoopVars`) erben `anim.*`.
- `anim.time` wird als `frame·delay_cs/100` gerechnet, damit `0.30000000000000004` nicht auftaucht.

### 7.4 Ablauf `RenderAnimation` im Frame-Modus

1. Prüfungen wie bisher (Canvas), dann das Pixelbudget: `frames × W × H ≤ maxAnimPixels` (32 000 000). Sonst `animation too large: %d frames of %dx%d exceed %d pixels`.
2. **Ein** Wurzel-Evaluator, `now` einmal eingefroren (in der Template-Zeitzone).
3. Für `f = 0 … frames−1`: frischer `renderCtx` (eigener `maxLayers`-Zähler) mit `root.withAnim(f, frames, delay)`, Bild-Cache über alle Frames geteilt. Die ganze Layer-Liste wird durchlaufen, `copy`-Layer übernehmen den Zeiger also in jedem Frame.
4. Ergebnis: `Animation{Frames, Delay: frame_delay·100 (cs), Loop: loop}`. Es gibt keinen zusätzlichen Halte-Frame. Frame 0 ist pixelgleich mit `Render`.

`seed` wird im Frame-Modus nicht gebraucht. Gleiche Eingabe und gleiches `now` ergeben gleiche Frames.

### 7.5 GIF-Ausgabe

`Animation` bekommt zwei Felder: `Delay int` (cs, > 0 = Frame-Modus) und `Loop bool`.

| | Splitflap (unverändert) | Frame-Modus |
|---|---|---|
| Delay je Frame | `round(100/fps)` cs | `Delay` cs |
| letztes Frame | `gifFinalDelay` = 100 cs | `Delay` cs (kein Halten) |
| `LoopCount` | `loop ? 0 : -1` | `loop ? 0 : -1` |

Gleiche Folgeframes werden weiter zusammengefasst (Delays addiert). Delta-Rechtecke und Palette bleiben unverändert. `Delay` außerhalb 0–1000 ist ein Encoder-Fehler. Bei `Delay = 0` muss `FPS > 0` sein (wie bisher).

## 8. Zeitzone **[P]**

- `meta.timezone` (optional): IANA-Name, Default `Europe/Berlin` (wie PHP `date_default_timezone_set('Europe/Berlin')`).
- Validierung in `LoadTemplate`:
  - Länge ≤ 64,
  - Muster `^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`,
  - nicht `Local`,
  - danach `time.LoadLocation`.
  Ungültig ergibt `renderer: LoadTemplate: invalid meta.timezone %q`. `UTC` ist erlaubt.
- `import _ "time/tzdata"` im Paket `renderer`. Die Zeitzonen-Datenbank ist damit eingebettet (≈ 450 KB), was für Windows (Desktop-Build) und das Alpine-Image ohne `tzdata` nötig ist. Kein CGO.
- `now` = `Uhr().In(location)`. Die Uhr ist im `Renderer` für Tests injizierbar (`nowFunc`). `NewEvaluator` (exportiert) bleibt bei `time.Now()`.
- **Gewollte Verhaltensänderung:** Alle Templates mit `now` zeigen Berliner Zeit statt Serverzeit. Der Server-Container läuft vermutlich in UTC, dort war die Uhr also bisher eine Stunde falsch.

## 9. Cache und Server **[P]**

- `Template.UsesNow() bool`: `true`, wenn irgendein ausgewertetes Feld (auch in verschachtelten `layers` von Blöcken, `loop` und `splitflap`) die Zeit liest. Geprüft werden:
  - in `value`, `color`, `rotate` (Rohwert und alle `then`/`else`-Zweige): jedes `{{…}}` (Regex `interpolateRe`), das das Token `now` (`(^|[^A-Za-z0-9_.])now([^A-Za-z0-9_]|$)`) oder einen Filter `format(` enthält,
  - in `if`, `elif` und den `if`/`elif`-Ausdrücken bedingter Felder: der ganze Ausdruck mit demselben Token-Muster.

  Die Erkennung ist konservativ: lieber einmal zu oft ungecacht als eine eingefrorene Uhr. Text außerhalb `{{ }}` (z.B. „Unknown") zählt nicht. Erwartung bei den echten Templates: `true` für `nederland-*`, `oebb-*`, `umuc-*`, `false` für `faltblatt-105-v1`, `rhb-*`, `sbb-*`.
- `renderAndServe` lädt das Template **vor** dem Cache-Lookup, weil es `UsesNow` kennen muss. 404 bleibt 404. Ist es zeitabhängig:
  - kein `cache.GetExt`, kein `cache.Set`/`SetExt`, auch nicht beim erneuten Prüfen nach der GIF-Wartezeit,
  - Antwort-Header `X-Cache: BYPASS` und `Cache-Control: no-store`.
- Alle anderen Templates verhalten sich wie bisher (`HIT`/`MISS`).

**DoS-Betrachtung:**

- **PNG:** Ungecachte PNG-Renders konnte ein Angreifer schon immer erzwingen, indem er den Body variiert. Es gibt keinen neuen Vektor. Der Body ist auf `maxRequestBodyBytes` begrenzt, Layer auf 256, Canvas auf 16384.
- **GIF:** Jeder ungecachte GIF-Render läuft weiter durch `gifSem` (höchstens 2 parallel, Wartezeit 15 s, danach 503 mit `Retry-After`), auch im Bypass-Fall. Der Frame-Modus erhöht die Kosten pro Render (60 statt ~50 Frames bei 240×240 ≈ 3,5 Mio. Pixel, unter dem Budget von 32 Mio.).
- **Template laden:** Das neue YAML-Parsen vor dem Cache-Hit kostet bei diesen Templates Mikrosekunden.
- **Grenzen:** Die harten Obergrenzen (125 Frames, 32 Mio. Pixel, 120 s Gesamtdauer, `maxPipelineDepth`) gelten pro Request.

## 10. Web und Editor

- `detailData.Animated` und `editorPageData.Animated` kommen aus `tmpl.IsAnimated()`. Damit erscheinen „Animation abspielen", „GIF herunterladen" und der Umschalter im Editor auch für Frame-Modus-Templates.
- „Erneut abspielen" bzw. „Animation abspielen" holt das GIF neu (neue Blob-URL). Bei einem Loop-GIF heißt das: Neustart mit der dann aktuellen Zeit. Das passt, weitere Anpassungen am JS gibt es nicht. Nur die Kommentare „nur für splitflap" werden korrigiert.
- Node-Editor (`web/static/node-filters.js`):
  - `parseValueAndFilters` trennt klammer- und anführungszeichenbewusst an `|` auf oberster Ebene,
  - `FILTER_DEFS` bekommt `mod` (math, mit Argument) und `minutes` (time, ohne Argument),
  - die Live-Vorschau kennt `mod`/`minutes` und wertet Variablen-Argumente gegen das Test-JSON aus (`now.*`/`anim.*` sind in der Vorschau leer, wie heute schon `now`).
- `meta`-Felder bearbeitet der Node-Editor nicht. Er übernimmt `meta` beim Zurückschreiben unverändert (`{...parsed, layers}`), und `node-types.js` bleibt unverändert.

## 11. Drehachse von `rotate` **[P]**

`rotateImageCW` rundet die Ziel-Bounding-Box auf `ceil(…)`. Bei ungerader Breite oder Höhe des Ergebnisses (z.B. 99 bei 45° und 70×70) liegt die Bildmitte bei x,5, `renderImage` platziert aber mit Ganzzahl-Division `x − Dx/2`, und die Achse rutscht um 0,5 px. Fix: `newW`/`newH` um 1 vergrößern, wenn `(newW − srcW)` bzw. `(newH − srcH)` ungerade ist. Der Zielmittelpunkt bleibt `newW/2`. Die Achse liegt dann für jeden Winkel exakt auf `(x, y)`. Betrifft nur Layer mit `rotate` (alles Uhren, also zeitabhängig).

## 12. Template `umuc-105-v1` (Ziel)

Neue bzw. geänderte Dateien:

- `clock-seconds.png` (neu): 70×70 RGBA, transparent. Roter Zeiger `#e2001a`, 1,5 px breit, von y = 2 bis y = 46 bei x-Mitte 35,0 (Spitze 33 px über, Schwanz 11 px unter der Achse), dazu ein Achsenpunkt mit Radius 2,5 px um (35,0 | 35,0). Kantenglättung durch 8×8-Supersampling. Bei 0° zeigt er nach oben. Erzeugt wird er per Wegwerf-Go-Skript, das nicht ins Repo kommt.
- `clock-hour.png` und `clock-minutes.png`: Inhalt 1 px nach links verschoben (§5.3 Nr. 3), sonst unverändert.

Fragment (das vollständige Ziel-YAML steht im Plan, Task 7):

```yaml
meta:
  name: "U-Muc 1.05\""
  description: "U-Bahn München, 240×240px, 1.05 Zoll, Bahnhofsuhr mit laufendem Sekundenzeiger"
  author: "christoph"
  version: "2.0"
  canvas:
    width: 240
    height: 240
  timezone: "Europe/Berlin"
  animation:
    frames: 60        # eine Minute …
    frame_delay: 1    # … in Sekundenschritten, danach von vorn
    loop: true

layers:
  # … Hintergrund …

  # Uhr: Zifferblatt aus bg.png, Mittelpunkt (39,56). Zeiger-PNGs 70×70,
  # Drehpunkt = Bildmitte, bei 0° nach oben; rotate dreht im Uhrzeigersinn,
  # x/y sind der Drehpunkt. Stunde und Minute bleiben im GIF stehen (now ist
  # pro Render eingefroren), nur der Sekundenzeiger läuft über anim.frame.
  - type: image
    file: clock-hour.png
    x: 39
    y: 56
    rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"
  - type: image
    file: clock-minutes.png
    x: 39
    y: 56
    rotate: "{{now.minute | mul(6)}}"
  - type: image
    file: clock-seconds.png
    x: 39
    y: 56
    rotate: "{{now.second | add(anim.frame) | mul(6)}}"

  # … Zug 2 (Muster für alle drei Zeilen) …
  - type: image
    if: "equals(zug2.nr, 'U1')"
    file: U1.png
    x: 120
    y: 62
  # … elif U2 … U8 …
  - type: text
    else: true
    value: "{{zug2.nr}}"
    x: 121
    y: 65
    font: regular
    size: 12
    color: "#ffffff"
  - type: text
    value: "{{zug2.vonnach}}"
    # …
  - type: text
    value: "{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"
    x: 238
    y: 73
    align: right
    # …

  # … zwei copy-Layer am Ende (unverändert) …
```

Die Badge-Kette pro Zeile ist eine Layer-Kette `if`/`elif`…/`else`: genau ein Badge oder der Text. Das deckt auch `U9` und leere Nummern ab, eine `if`-Bedingung pro Text ist nicht mehr nötig. Leere Werte zeichnen keinen Text.

## 13. Limits (Übersicht)

| Grenze | Wert | Wo |
|---|---|---|
| Frames (beide Modi) | ≤ 125 (Frame-Modus ≥ 2) | `LoadTemplate` / `RenderAnimation` |
| Pixelbudget | frames × W × H ≤ 32 000 000 | `RenderAnimation` |
| Gesamtdauer Frame-Modus | ≤ 120 s | `LoadTemplate` |
| `frame_delay` | 0.02–10 s, 0.01-Raster | `LoadTemplate` |
| GIF-Delay | 0–1000 cs | `EncodeGIF` |
| Pipeline-Verschachtelung | 4 | Evaluator |
| GIF-Renders parallel | 2, Wartezeit 15 s | Server (unverändert) |
| `meta.timezone` | ≤ 64 Zeichen, Muster s.o. | `LoadTemplate` |

## 14. Sicherheit

- `file` bleibt uninterpoliert, die Badges bleiben einzelne Layer.
- Zeitzonen-Name ist Template-Inhalt, nicht Request-Inhalt. Die Validierung blockiert trotzdem Pfad-artige Namen (`..`, absolute Pfade) vor `time.LoadLocation`, das zusätzlich selbst prüft. `Local` ist ausgeschlossen, weil es von der Host-Konfiguration abhängt.
- Der Pipeline-Parser ist linear in der Ausdruckslänge, die Rekursion ist durch `maxPipelineDepth` begrenzt, und Fehler ergeben `""` statt Panic. Fuzz-artiger Test im Plan (unbalancierte Klammern und Quotes).
- Die Request-Daten können `anim`/`now` nicht überschreiben (reserviert).
- Cache-Bypass: siehe DoS-Betrachtung §9.
- Server-Build bleibt CGO-frei, `time/tzdata` ist stdlib.

## 15. Dem Umsetzer überlassen **[B]**

- Interne Hilfsfunktionsnamen jenseits der im Plan festgelegten Signaturen.
- Ob `UsesNow` in `template.go` oder einer eigenen Datei liegt (Plan: eigene Datei).
- Die exakte Pixelgeometrie des Sekundenzeigers innerhalb der Vorgaben aus §12, nach Sichtprüfung.

## 16. Abnahme

1. `go test ./...`, `go vet ./...`, `node --test web/static/test/*.mjs` grün; `CGO_ENABLED=0 go build ./cmd/zza-server` ok.
2. `POST /umuc-105-v1/render?format=gif` mit `default.json` ergibt ein GIF mit 60 Frames, alle Delays 100 cs, `LoopCount` 0, `X-Cache: BYPASS`. Der Sekundenzeiger läuft in beiden Uhren (oben links und unten rechts), Stunde und Minute stehen.
3. PNG zeigt die Berliner Zeit, „Min." 5 / 8 / 16.
4. Kompatibilitätstest des Evaluators grün, alle Templates laden und rendern (PNG + GIF).
5. Faltblatt-GIF bytegleich zu vorher (bestehender Determinismus-Test plus manueller Vergleich).
6. Manueller Testplan `docs/superpowers/plans/2026-11-13-clock-animation-manual-tests.md` durchlaufen.
