# Detailseite: Formular statt JSON — Design

## Problem

Die Vorschau-Seite `GET /{template}` zeigt links ein rohes JSON-Textfeld mit allen
Feldern aus `default.json` — auch solchen, die das Template gar nicht zeichnet
(`intervalTime`, `stationNr`, `reihung` …). Endnutzer (Modellbahner) wollen nur
Ziel, Zeit, Gleis usw. eintragen und das Bild herunterladen. Aktionen (PNG, GIF,
Abspielen) liegen verstreut unten rechts, die Vorschau schwimmt in einer großen
weißen Fläche.

## Ziel

- Formular mit **nur den Feldern, die das Template wirklich verwendet**, gruppiert
  wie im alten PHP-Generator (Bahnsteig, Zug 1, Zug 2, Zug 3).
- Vorschau ist der Mittelpunkt; direkt darunter eine Aktionsleiste:
  **Abspielen** (nur animierte Templates), **PNG herunterladen**, **GIF herunterladen**.
- Kompaktes, zentriertes Layout statt Vollbild-Spalten.
- Entwickler-Infos (Render-URL, JSON, ZIP, Bearbeiten) in einem eingeklappten
  Bereich „Für Entwickler“ — sichtbar, aber nicht im Weg.

## Feld-Erkennung (Server)

`renderer.ReferencedPaths(yamlSrc []byte) (map[string]bool, error)`
parst `template.yaml` als `yaml.Node`-Baum und sammelt aus **allen Skalar-Werten**
(keine Keys, keine Kommentare) alle Bezeichner-Token der Form
`ident(.ident)*`. Ein Feld aus `default.json` gilt als verwendet, wenn sein
Blatt-Pfad (`gleis`, `zug1.vonnach`) als Token vorkommt.

`buildForm(yamlSrc, defaultJSON []byte) ([]formGroup, error)` (internal/server)
liest `default.json` **reihenfolgetreu** (json.Decoder-Token-Stream) und baut:

- Gruppe „Bahnsteig“ aus verwendeten Top-Level-Skalaren,
- je eine Gruppe „Zug N“ pro verwendetem Objekt `zugN` (andere Objekte: Key als Titel),
- leere Gruppen entfallen.

Feld: `Path`, `Label` (deutsche Labels für bekannte Keys, sonst Key), `Value`
(Default als Text), `Kind` (`text` | `number` | `bool`, aus JSON-Typ des Defaults),
`Size` (`s` für kurze Felder wie Zeit/Gleis, sonst `l`), `Placeholder`.
Gruppe: `Title`, `Fields`, `Open` (mindestens ein Default nicht leer).

## Client

- Das komplette Default-Objekt wird im Browser aus dem (eingeklappten) JSON-Feld
  geparst — so bleibt die Key-Reihenfolge von default.json erhalten. Formularwerte werden darauf gelegt → nicht angezeigte
  Felder behalten ihre Defaults.
- `number`-Felder senden eine Zahl, wenn der Text eine ganze Zahl ist, sonst String.
- Live-Rendering bei Eingabe (Debounce 400 ms), kein Absende-Button.
- Gruppen mit leeren Defaults sind eingeklappte `<details>`.
- Fallback: liefert `buildForm` keine Gruppen (kein default.json / Fehler),
  bleibt das JSON-Textfeld im Entwickler-Bereich editierbar.

## Visuelles Konzept

Markenrahmen bleibt (Teal-Hintergrund, Orange als Primäraktion, Aptly + IBM Plex),
damit Galerie und Detailseite zusammengehören. Neu:

- Max. 1280 px breit, zwei Spalten: links die Vorschau (sticky), rechts das Formular.
  Unter 860 px eine Spalte, Vorschau oben.
- Die Vorschau sitzt auf einer weißen Kachel; jede Display-Hälfte hat schwarze
  Blende plus blauen Modulrahmen (`#10289c`) wie die DB-Anzeigen. Die beiden
  Display-Hälften werden in Viertelschritten (1×–3×, `image-rendering: pixelated`) skaliert — das ist das eine
  auffällige Element der Seite.
- Aktionsleiste direkt am Gehäuse; alles andere ruhig: Labels in Satzschreibung,
  keine Versal-Labels über jedem Feld.

## Nicht im Umfang

Editor-Seite (`/edit/…`), Galerie, Render-API.
