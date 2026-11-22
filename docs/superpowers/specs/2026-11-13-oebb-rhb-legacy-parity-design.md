# ÖBB/RhB an Legacy-PHP angleichen + animierte Galerie — Design

Stand: 2026-11-13

## Ziel

`templates/oebb-105-v1` und `templates/rhb-105-v1` sollen so aussehen wie die
PHP-Themes in `legacy/oebb-105-v1` und `legacy/rhb-105-v1`, einschließlich
aller Sonderfälle. Die ÖBB-Uhr zeigt Sekunden und läuft im GIF. Die Galerie
zeigt für animierte Templates das GIF, mit einem kleinen Neustart-Button.

Nicht-Ziele: 0.96"-Templates (vom Nutzer vorerst entfernt), andere Templates,
Änderungen am Editor.

## Ansätze

1. **Renderer minimal erweitern, Templates rein in YAML** (gewählt). Das PHP
   nutzt drei Dinge, die die Template-Sprache nicht kann: Text an die Breite
   des vorherigen Textes anschließen, vertikale Schrift-Streckung, ein paar
   String-/Zeit-Umformungen. Jede Erweiterung ist klein und allgemein nutzbar.
2. Neuer Layer-Typ `row` (Inline-Fluss mehrerer Texte/Bilder). Mächtiger, aber
   deutlich mehr Code; RhB braucht die Breite auch für ein Rechteck, was `row`
   nicht abdeckt.
3. Feste Positionen in YAML. Kein Renderer-Code, weicht aber bei jedem
   anderen Text vom PHP ab — verfehlt das Ziel.

## Renderer-Erweiterungen

### Text-Layer: `id` und `advance`

```yaml
- type: text
  id: zeit          # [a-z][a-z0-9_]{0,31}
  advance: 3        # optional, Pixel, 0–240, Standard 0
  value: "{{zug1.zeit}}"
  x: 0
```

Nach dem Zeichnen stehen für alle **folgenden** Layer in Koordinaten-
Ausdrücken (`x`, `y`, `width`, `height`, `max_width`) bereit:

- `{{zeit.width}}` — gezeichnete Breite der breitesten Zeile in Pixeln
  (inkl. `scale_x`); leerer Text → 0.
- `{{zeit.end}}` — `x + width + advance`, wenn `width > 0`; sonst `x`
  (leerer Text schiebt nichts weiter — wie `$x += $w + 3` nur im `if`).

`x` ist dabei die linke Kante des gezeichneten Textes (bei `align: left` das
`x` des Layers). Nur `type: text`, nur mit `align: left` (sonst
Template-Fehler beim Laden), nicht zusammen mit `fit_items`. Ein per `if`
übersprungener Layer setzt nichts; wer bedingt anschließen will, nimmt einen
bedingten `value` mit leerem `else`. Unbekannte Variablen ergeben wie bisher
0. IDs auf oberster Ebene gelten auch in Loops; IDs in einem Loop gelten nur
in dieser Iteration. Doppelte `id` im Template: Fehler beim Laden.

### Text-Layer: `scale_y`

Vertikale Streckung, Gegenstück zu `scale_x`: erlaubt 0.05–8, Standard 1,
gleiche Validierung. Verankert an `y` (Oberkante bzw. bei `valign: baseline`
die Grundlinie). Zeilenhöhe und `valign`-Berechnung rechnen mit der
gestreckten Höhe.

### Filter

| Filter | Verhalten |
|---|---|
| `clock` | Minuten → `HH:MM`, modulo 1440 (`963` → `16:03`, `1445` → `00:05`); keine Zahl → `""` |
| `replace('a','b')` | Ersetzt alle `a` durch `b`; leeres `a` → unverändert |
| `match('re', n)` | Erster Treffer des RE2-Ausdrucks, Gruppe `n` (0 = ganzer Treffer); kein Treffer, ungültiger Ausdruck, Ausdruck > 100 Zeichen oder `n` außerhalb → `""`. Kompilierte Ausdrücke werden gecacht (höchstens 256 Einträge). |
| `format('p', sek)` | Optionales zweites Argument: Sekunden-Versatz auf `now` (Zahl, Variable oder Pipeline wie bei den Mathe-Filtern), z. B. `{{now \| format('HH:mm:ss', anim.time)}}` |

`clock` wickelt über Mitternacht um; das PHP würde `24:05` zeigen — bewusste
Abweichung.

## Template ÖBB (`oebb-105-v1`)

Referenz: `legacy/oebb-105-v1/index.php`. Schriftgröße = Legacy × 16/9
(GD rechnet Punkt bei 96 dpi; so steht es auch in `docs/yaml-template-spec.md`).
Das PHP zeichnet die meisten Texte drei- bis viermal übereinander; das
Template wiederholt die Layer entsprechend, damit die Strichstärke stimmt.

- `meta.timezone: Europe/Vienna`; `animation: frames 60, frame_delay 1, loop true`.
- Uhr rechts oben `HH:mm:ss`, rechtsbündig an x 236, läuft über `anim.time`.
- Zeile 2: Abfahrtszeit weiß (Legacy 9, `scale_y 1.2`), bei `abw > 0`
  dahinter die neue Zeit (`zeit + abw` als `HH:MM`) in `#fbef52`, dahinter
  das Ziel (Legacy 12.75) — jeweils 3 px Abstand, fließend. Zeit ohne `:`
  wird unverändert gezeigt, leere Zeit: Ziel beginnt bei x 0.
- Via: ` - ` wird zu `~` (wie PHP), Legacy 8.55, einzeilig.
- Hinweis: Legacy 8.25, `scale_y` nominell 1.2 (im Abgleich 1.28 und
  `scale_x 1.02`, weil der Go-Renderer die Schrift minimal kleiner setzt). Beginnt er mit `*`: gelber Balken
  (`#fbef52`, y 89–113, volle Breite), Text ohne `*` in Schwarz; sonst weiß
  ohne Balken.
- Zuggattung ab x 56: passt `nr` auf `([a-zA-Z]+)([0-9]+)` und ist der
  Buchstabenteil `S` (egal ob groß/klein): `s.png` + Nummer fett (Legacy 12);
  passt es sonst: Typ fett (10.5) + Nummer leicht (9); passt es nicht: ganze
  `nr` fett (10.5).
- Obere Hälfte wird nach y 121 kopiert.

## Template RhB (`rhb-105-v1`)

Referenz: `legacy/rhb-105-v1/index.php`.

- Zugnummer als Punktmatrix-Feld: zehn `•` (LLPIXEL3, Legacy 33,
  `scale_x 0.5`, gelb, Abstand 11 px, Unterkante y 33), darüber `nr` in
  Schwarz (Legacy 23, `scale_x 0.5`, x 3, Unterkante y 31), dann schwarzes
  Rechteck von `nr.width + 3` bis zum rechten Rand, y 0–36. (Im Abgleich
  jeweils +1 px, weil die gemessene Breite 1 px unter dem PHP-Wert liegt.)
- Via: an `-` geteilt, 04B_03 Legacy 10, gelb, ab `x = nr.width + 21`, y 6,
  6 px Abstand, ohne Begrenzung der Anzahl.
- Schriftgrößen ebenfalls Legacy × 16/9.
- Zeit + Ziel: eine Zeile, LLPIXEL3 Legacy 30, `scale_x 0.5`, gelb, x 3,
  Oberkante y 40, kein Umbruch.
- Obere Hälfte wird nach y 120 kopiert.

## Template SBB (`sbb-105-v1`) — Nachtrag

Referenz: `legacy/sbb-105-v1/index.php`. Schriftgrößen Legacy × 16/9, Texte
wie im PHP doppelt gezeichnet, obere Hälfte nach y 120 kopiert.

- Renderer: Bedingungen `and(…)` und `or(…)` (beliebig viele Teilbedingungen,
  verschachtelbar, ohne Argumente falsch).
- Info-Modus, wenn `abw`, `vonnach`, `nr` und `via` leer sind (getrimmt): nur
  der Hinweis, Legacy 12.75, `#fff048`, zweimal mit 1 px Versatz, ohne Uhrzeit.
  Das PHP bricht mit der fetten Schrift um, zeichnet aber normal; das Template
  misst normal mit `max_width 140`.
- Sonst: Abfahrtszeit fett (Legacy 12); Hinweis auf rotem Balken
  (x 3–119, y 78–107), zweizeilig; Ziel unten verankert über dem Hinweis bzw.
  am Feldende, `/` wird zu `/ `.
- Via an `-` geteilt, höchstens 5, Punkt + Text je 16 px.
- `abw > 0` mit Hinweis: blaues Feld `#132a9b` mit `+N’` an Stelle der
  Zuggattung (die entfällt); ohne Hinweis: `+N’` unter der Zeit.
- Zuggattung (Groß-/Kleinschreibung egal, Reihenfolge wie PHP): IC (nicht
  ICE/ICN), EC, ICN, IR, VAE mit Logo + Nummer; RE rot auf Weiß; S/R schwarz
  auf Weiß; `PE BEX`/`PE GEX` und ICE, TGV, NJ, RJX, EXT, CNL, … kursiv weiß
  auf Rot; sonst `nr` unverändert.
- Bewusste Abweichung: Das PHP verankert das Ziel an der Unterkante der Tinte,
  ein Ziel ohne Unterlänge sitzt dort ~4 px tiefer als eines mit. Das Template
  nimmt die Mitte (beide Fälle ±2 px).

Beide Templates werden gegen die PHP-Ausgabe (`php index.php` im
Legacy-Ordner, Ergebnis in `cache/`) pixelweise verglichen und nachjustiert;
Ziel sind höchstens 1–2 px Lageabweichung je Element.

## Galerie

- `gallery.TemplateInfo` bekommt `Animated bool` (`tmpl.IsAnimated()`).
- Animierte Karten laden `/{name}/preview?format=gif` statt des PNG.
- Auf animierten Karten sitzt oben rechts im Vorschaubereich ein kleiner
  Button „↻" (`aria-label="Animation neu starten"`). Klick lädt das GIF neu
  (`&r=<Zähler>` an der URL), ohne zur Detailseite zu navigieren. Der Button
  liegt als Geschwister neben dem Karten-Link, nicht darin.
- Schlägt das GIF fehl (z. B. 503 bei belegten Render-Slots), fällt das Bild
  einmalig auf das PNG zurück.
- Templates mit `now` umgehen den Render-Cache. Damit nicht jeder
  Galerie-Aufruf mehrere 60-Frame-GIFs rendert, hält der Preview-Endpoint
  solche GIFs 10 Sekunden im Speicher (nur `GET /{template}/preview?format=gif`,
  `X-Cache: PREVIEW`); die Uhr in der Galerie kann dadurch bis zu 10 s
  nachgehen. `POST /{template}/render` bleibt unverändert.

## Dem Umsetzer überlassen

Interne Struktur (wo die ID-Variablen gehalten werden), exakte Pixelwerte der
Templates (aus dem Vergleich), CSS des Buttons im Stil von `app.css`.
