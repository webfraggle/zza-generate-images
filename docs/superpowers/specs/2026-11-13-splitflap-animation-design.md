# Faltblatt-Animation (Split-Flap) als Template-Fähigkeit — Design

Datum: 2026-11-13 · Status: Entwurf zur Freigabe

Legende: **[C]** = von Christoph so entschieden · **[P]** = Entscheidung des Planers (darf Christoph kippen) · **[B]** = dem Umsetzer überlassen

## 1. Absicht und Warum

Das ZZA-Display (ESP32) zeigt Zugzielanzeiger-Bilder, die dieser Dienst rendert. Das Theme „Faltblatt 1.05" bildet eine mechanische Faltblattanzeige nach, ist aber ein Standbild. Christoph möchte, dass der Wechsel auf einen neuen Zug wie bei einer echten Anlage aussieht: Klappen rattern und kommen nacheinander auf dem neuen Text zum Stehen.

Randbedingung des Einsatzes: Das Display kennt beim Wechsel **nur die Daten des neuen Zugs**. Es zeigt das Standbild des alten Zugs und spielt danach die neue Animation ab. Die Animation muss deshalb ohne Wissen über den alten Inhalt glaubwürdig „aus dem Alten heraus" starten und auf dem Standbild des neuen Zugs enden.

## 2. Ziele

1. **[C]** Animation ist eine **Fähigkeit des Template-Formats**, nicht fest für ein Theme verdrahtet. Andere Faltblatt-Templates (andere Größen/Felder) sollen sie später ohne Go-Änderung nutzen können.
2. **[C]** Ausgabeformat ist **animiertes GIF**. Nur GIF.
3. **[C]** Das letzte Frame ist exakt das Standbild (identisch zum PNG-Render derselben Daten, bis auf GIF-Palettierung) und bleibt stehen.
4. **[C]** Verhalten „wie bei einer echten Anlage": jede Klappe blättert für sich durch ihren Vorrat, die Stellen stoppen zu unterschiedlichen Zeiten, große Tafeln (Zugart, Via, Zeit) klappen als Ganzes.
5. **[C]** Das Layout von `faltblatt-105-v1` wird korrigiert, sodass das Standbild dem alten Generator entspricht. Referenz: `docs/superpowers/specs/assets/2026-11-13-faltblatt-referenz.png` (240×240, Daten = `templates/faltblatt-105-v1/default.json`) und die Rendering-Logik in `legacy/faltblatt-105-v1/index.php` + `gfx_functions.inc.php`.
6. **[C]** Erstes und abgestimmtes Template ist nur `faltblatt-105-v1`.

## 3. Nicht-Ziele

- Firmware / Abspielen auf dem ESP32 (läuft unabhängig woanders). **[C]**
- MJPEG oder andere Animationsformate. **[C]**
- Übergang vom *tatsächlichen* alten Inhalt (alte Daten sind nicht bekannt).
- Umbau weiterer Templates, das alte 240×270-Faltblatt, Rendering von `hinweis`/`abw` (macht das Legacy-Theme auch nicht).
- Ton.

## 4. Anti-Ziele (wäre ein Fehlschlag, auch wenn es „funktioniert")

- Bestehendes Verhalten ändert sich: `POST /{template}/render` ohne Format-Angabe liefert weiter PNG, bitgleich wie bisher für alle anderen Templates. Displays im Feld dürfen nichts merken.
- Nicht-deterministische Ausgabe (gleiche Eingabe ⇒ anderes GIF). Bricht den Cache.
- Alle Klappen stoppen gleichzeitig, oder das GIF läuft in Schleife.
- CGO oder neue schwere Abhängigkeiten im Server-Build (stdlib `image/gif` + vorhandenes `x/image` reichen).
- Unbegrenzte Renderkosten über die öffentliche, unauthentifizierte Render-Route.

## 5. Template-Format: neuer Layer-Typ `splitflap` **[P]**

Ein `splitflap`-Layer beschreibt eine oder mehrere gleichartige **Klappenmodule**. Ein Modul ist ein Rechteck mit horizontaler Teilung (Scharnier); sein Inhalt wird durch Kind-Layer gezeichnet, die den aktuellen Wert über eine Variable bekommen (wie bei `loop`).

Grundsätze (aus dem Spec-Check, **[P]**):

- **Modul = ganze physische Klappe.** Das Rechteck deckt das komplette Blatt ab, wie es im Hintergrundbild gezeichnet ist, nicht nur die Fläche um die Schrift. Die Teilung liegt auf dem Scharnier des Overlays; liegt das nicht auf halber Höhe, setzt `split_y` (relativ zum Modul, Default `height/2`) die Linie.
- **Blatt-Hintergrund = Leinwand-Schnappschuss.** An der Stelle des Layers in der Zeichenreihenfolge wird der Leinwandinhalt unter dem Modulrechteck als Blatt-Hintergrund genommen; jeder Wert wird darauf gezeichnet. Ein Blatt ist damit deckend, beim Umklappen scheint nichts vom Nachbarwert durch.
- **Feste Beschriftung auf einer Klappe** (z. B. „Abfahrt" neben der Stunde) ist ein konstanter Kind-Layer und klappt mit.
- `splitflap`-Module dürfen sich nicht gegenseitig überlappen (Template-Fehler).

```yaml
# Zielzeile: 13 Einzelklappen
- type: splitflap
  x: 54            # linke obere Ecke des ersten Moduls (Werte beispielhaft)
  y: 82
  width: 12        # Modulgröße = ganzes Blatt
  height: 29
  split_y: 14      # Scharnier, relativ zum Modul
  cells: 13        # Anzahl Module nebeneinander (Default 1)
  pitch: 14        # Abstand Modul zu Modul in x (Default = width)
  value: "{{zug1.vonnach | upper}}"
  split: chars     # chars = ein Zeichen (Rune) pro Modul; fehlt = ganzer Wert in einem Modul
  reel: " ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ-."   # Vorrat; String = Zeichenliste, oder YAML-Liste von Werten
  var: ch          # Name der Variable für die Kind-Layer
  layers:          # Koordinaten relativ zum Modul
    - type: text
      value: "{{ch}}"
      x: 5
      y: 23
      valign: baseline
      align: center
      scale_x: 0.5
      font: bold
      size: 21.33
      color: "#3a3c3b"

# Zugart: eine Tafel, blättert durch die anderen Zugarten
- type: splitflap
  x: 55
  y: 53
  width: 86
  height: 18
  value: "{{zug1.nr | lower}}"
  reel: ["rb", "re", "ic", "ice", ""]
  var: nr
  layers:
    - if: "startsWith(nr, 'ice')"
      layers: [{type: image, file: ice.png, x: 0, y: 0}]
    - elif: "startsWith(nr, 'ic')"
      layers: [{type: image, file: ic.png, x: 0, y: 0}]
    # …
```

Regeln:

- **Standbild:** Im PNG-Render (und im letzten GIF-Frame) zeichnet jedes Modul seine Kind-Layer mit dem Zielwert. Ein `splitflap`-Layer ist also auch ohne Animation sinnvoll (Einzelzeichen-Raster). Bei `split: chars` werden überzählige Zeichen abgeschnitten, fehlende Module sind leer (Wert `""`/Leerzeichen).
- Kind-Layer dürfen `text`, `image`, `rect` und Block-Nodes (if/elif/else) sein; keine `loop`, `copy` oder verschachtelten `splitflap`. Die Variable `var` ist in Werten **und** Bedingungen nutzbar.
- Zeichnung der Kind-Layer ist auf das Modulrechteck geclippt; Kind-Koordinaten sind relativ zum Modul (anders als bei `loop`).
- Optionale Timing-Felder pro Layer: `flip_frames` (Frames pro Umklappen, Default 2), `min_flips`, `max_flips` (Defaults siehe 6).
- `splitflap` darf in Block-Nodes stehen, nicht in `loop`.
- Alle übrigen Layer (auch `copy`) verhalten sich pro Frame wie heute. Dadurch übernehmen die drei `copy`-Layer des Faltblatt-Templates die Animation automatisch in die untere Bildhälfte, und `fg.png` liegt als Overlay über den Klappen.

Animationseinstellungen im Kopf:

```yaml
meta:
  animation:
    fps: 20          # Default 20, erlaubt 5–25
    duration: 2.5    # maximale Laufzeit in Sekunden, Default 2.5, erlaubt 0.5–5
```

`meta.animation` ist optional; fehlt es, gelten die Defaults, sobald ein GIF angefordert wird.

### Ergänzungen am bestehenden Format (nötig für die Layout-Korrektur) **[P]**

- `text`: `valign: baseline` — `y` ist die Grundlinie (so rechnet das Legacy-PHP; ohne `height`).
- `text`: `scale_x` (Float, Default 1) — horizontale Skalierung der Schrift. Das Legacy staucht fast alle Texte auf halbe Breite (`xFactor 0.5` ⇒ `scale_x: 0.5`); nur Stunde und Minute haben natürliche Breite. Ausrichtung und Einpassung rechnen mit der skalierten Breite. Ohne `scale_x` bleibt der bisherige Zeichenpfad bitgleich.
- Legacy-Schriftgrößen: effektiv Legacy-Wert × 16/9 in px (PHP-Faktor 96/72 mal GDs eigene 96-DPI-Umrechnung; an der Referenz nachgemessen).
- `text`: einzeilige Element-Einpassung für die Via-Zeile: Wert an einem Trenner teilen, Elemente mit festem Abstand nebeneinander setzen, Elemente, die nicht mehr in `max_width` passen, weglassen — Verhalten exakt wie die Via-Schleife in `legacy/faltblatt-105-v1/index.php` (Element wird gezeichnet, wenn `xpos + Breite < x + max_width`; `xpos` rückt auch bei ausgelassenen Elementen um `Breite + gap` weiter). Feldnamen-Vorschlag: `fit_items: {split_by: "-", gap: 2}`.
- Filter, um einen Wert an einem Trenner zu teilen und den Teil davor/danach zu nehmen (für Stunde/Minute aus `13:30`). Benennung nach vorhandener Filter-Konvention **[B]**.
- `ice.png` aus `legacy/` ins Template übernehmen; Zugart-Erkennung unabhängig von Groß/Kleinschreibung, `ice` vor `ic`.

## 6. Animationsmodell **[P]** (Christoph: „wie bei einer echten Anlage")

**Ein Umklappen** von Wert A auf Wert B dauert `flip_frames` Frames (Default 2):

- Phase 1 (Klappe fällt, obere Hälfte): obere Modulhälfte zeigt bereits die obere Hälfte von B; darüber liegt die obere Hälfte von A, vertikal auf etwa die Hälfte gestaucht und zur Teilungslinie hin verankert, leicht abgedunkelt. Untere Hälfte zeigt noch A.
- Phase 2 (Klappe fällt, untere Hälfte): obere Hälfte zeigt B; in der unteren Hälfte liegt die untere Hälfte von B, von der Teilungslinie aus auf etwa die Hälfte gestaucht, leicht abgedunkelt, über dem Rest von A.
- Bei `flip_frames` > 2 werden die Stauchungsgrade feiner abgestuft.
- Solange ein Modul läuft, folgt Umklappen auf Umklappen ohne Ruheframe — genau das ergibt den „Ratter"-/Bewegungsunschärfe-Eindruck. Erst nach dem letzten Umklappen steht das Modul ruhig auf dem Zielwert.

Feinschliff des Aussehens (Abdunklung, Stauchung, optionales Überblenden für Bewegungsunschärfe) ist **[B]**, wird aber vom Planer anhand von Einzelframes abgenommen (siehe 10).

**Reihenfolge der Werte:** Jedes Modul durchläuft `k` Umklapp-Schritte entlang seines `reel` in fester, zyklischer Reihenfolge und endet auf dem Zielwert. Liegt der Zielwert im `reel`, wird die Startposition so gewählt, dass der Lauf natürlich dort ankommt (bei kurzem `reel` mit Umlauf). Liegt er nicht im `reel` (z. B. freier Via-Text, exotisches Zeichen), folgt er als letzter Schritt auf einen beliebigen `reel`-Eintrag. Ohne `reel` blättert das Modul durch leere Klappen.

**Wie viele Schritte:** `k` wird pro Modul pseudozufällig aus `[min_flips, max_flips]` gezogen. Defaults: `min_flips` = 4; `max_flips` = so viele, wie in `duration` passen. Der Zufall ist **deterministisch**: Seed = Hash aus Template-Name, Request-Daten, Layer-Index und Modul-Index.

**Start und Ende:**

- In Frame 0 befinden sich bereits alle Module in Phase 1 — kein ruhiges Startbild. Das kaschiert den Sprung vom alten Standbild.
- Auch Module mit leerem Ziel blättern (die echte Anlage dreht auf die Leerklappe).
- Die Animation endet ein Frame nach dem Stopp des letzten Moduls; dieses letzte Frame ist das Standbild.
- GIF spielt **einmal** (keine Loop-Extension); letztes Frame bekommt eine lange Anzeigedauer (1 s) für Player, die trotzdem schleifen.

Für `faltblatt-105-v1` animiert sind: Zielzeile (13 Zeichenklappen), Zugart, Via, die Klappe „Abfahrt + Stunde" (als Ganzes, „Abfahrt" steht auf jedem Blatt) und Minute. **Nicht** animiert: „Gleis" + Gleisnummer (festes Schild) und die leere Tafel oben rechts. `fg.png` rückt in der Layer-Reihenfolge hinter alle Texte/Klappen, direkt vor die `copy`-Layer. **[P]**

**Determinismus und `now`:** Der Zeitpunkt für `{{now.*}}` wird einmal pro Animation eingefroren. Templates, deren Werte von `now` abhängen, liefern naturgemäß zeitabhängige Ausgaben; die Zusage „gleiche Eingabe ⇒ bytegleiches GIF" gilt für Templates ohne `now`.

## 7. Ausgabe und Schnittstellen

**GIF-Erzeugung** (Paket `internal/renderer`, gemeinsam für Server und CLI):

- Eine globale Palette (≤ 256 Farben, bzw. `meta.canvas.colors` wenn kleiner) für alle Frames, damit nichts flimmert.
- Frames nach dem ersten enthalten nur das geänderte Rechteck (kleinere Dateien). Richtwert für `faltblatt-105-v1` mit Default-Daten: ≤ 400 KB; gemessener Wert wird berichtet. **[P]**
- Ein Template ohne `splitflap` ergibt bei GIF-Anforderung ein GIF mit einem Frame. **[P]**

**Server** **[P]**:

- `POST /{template}/render?format=gif` und `GET /{template}/preview?format=gif` → `Content-Type: image/gif`. Ohne `format` oder mit `format=png`: PNG wie bisher. Anderer Wert: 400.
- Die Ausnahme vom HTTPS-Redirect für `POST …/render` gilt unverändert auch mit Query.
- Cache: Format geht in den Schlüssel ein, Datei-Endung `.gif`. Bestehende PNG-Cache-Schlüssel bleiben gültig.
- Kostenbegrenzung: max. 125 Frames; max. 64 Module pro Template; max. 64 `reel`-Einträge pro Layer; identische Modulinhalte werden nur einmal gerendert. Überschreitung = Template-Fehler (422). Gleichzeitige GIF-Renders sind auf 2 begrenzt (weitere warten).

**CLI (Desktop):** `zza render … -o bild.gif` wählt GIF über die Endung; zusätzlich `--format png|gif`.

**Web:**

- Detailseite: Wenn das Template `splitflap`-Layer hat, gibt es „Animation abspielen" (zeigt das GIF in derselben Zwei-Hälften-Darstellung, erneut abspielbar) und einen GIF-Download neben dem PNG-Download. Standardansicht bleibt das Standbild.
- Galerie: bleibt beim statischen PNG.
- Editor (Desktop): Vorschau mit Umschalter Standbild/Animation und „erneut abspielen".
- Umsetzung der Darstellung **[B]**.

**Doku:** `docs/yaml-template-spec.md`, `docs/user-guide-templates.md`, `README.md` (API), `docs/backlog.md` (Eintrag „Animierte GIFs" abhaken).

## 8. Constraints

- Server-Build bleibt CGO-frei; keine neue Abhängigkeit ohne Not.
- Bestehende Sicherheitsregeln des Renderers gelten weiter (Dateinamen ohne Pfad, `file` nicht interpoliert, Layer-/Canvas-Limits). Das Layer-Limit zählt pro Frame nicht neu; Modulinhalte haben das eigene Limit aus 7.
- Renderzeit für das Default-GIF von `faltblatt-105-v1`: Richtwert < 1 s auf dem Entwicklungsrechner; dafür dürfen Bilder/Font-Faces innerhalb eines Renders gecacht werden. **[P]**
- Server-Tests laufen gegen die echten `templates/`; die Template-Änderung darf sie nicht brechen.

## 9. Dem Umsetzer überlassen **[B]**

Interne Struktur (Frame-Abstraktion, Caches), Namen von Go-Typen, genaue Filter-Namen, Frontend-Umsetzung, Quantisierungsverfahren, exakte Pixelwerte im korrigierten Template (maßgeblich ist die Referenz).

## 10. Abnahme

1. PNG von `faltblatt-105-v1` mit Default-Daten gegen das Referenzbild: gleiche Elemente an gleicher Position (±1 px), gleiche Größen, gleiche Via-Auswahl („Neuhof", „Gelnhausen"; „Hanau" entfällt), Overlay über den Texten, untere Hälfte korrekt umarrangiert. Pixelgleichheit ist wegen anderer Kantenglättung nicht verlangt.
2. GIF: letztes Frame entspricht dem PNG; Module stoppen zu unterschiedlichen Zeiten; kein Loop; gleiche Eingabe ⇒ bytegleiches GIF.
3. Der Umsetzer liefert eine Kontaktbogen-Grafik aller Frames (Hilfsmittel, nicht Teil des Produkts), anhand derer der Planer die Animation beurteilt und nachsteuert, bevor Christoph sie sieht.
4. Alle anderen Templates rendern als PNG bitgleich wie vor der Änderung.
5. `go test ./...` grün; neue Tests für `splitflap` (Standbild, Schrittfolge, Determinismus, Limits), GIF-Encoder, Server-Format-Parameter und Cache.
