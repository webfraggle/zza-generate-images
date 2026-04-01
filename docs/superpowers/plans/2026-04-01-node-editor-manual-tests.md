# Node Editor Phase 1 — Manueller Testplan

Vorbedingung: Server läuft, Template `test-node-editor` existiert unter `/test-node-editor/edit`.

---

### 1. Tab-Umschalter sichtbar

- Editor öffnen: `GET /test-node-editor/edit`
- Datei `template.yaml` in der linken Spalte anklicken
- **Erwartung:** Toolbar der mittleren Spalte zeigt zwei Tabs — `YAML` (aktiv) und `NODES`

---

### 2. YAML → Nodes parsen (Happy Path)

- Tab `NODES` anklicken
- **Erwartung:**
  - Canvas erscheint (hellgrauer Hintergrund `#F8F6F3`)
  - 3 Nodes sichtbar: `IMAGE`, `TEXT`, `LOOP`
  - LOOP-Node hat roten linken Rand; die anderen teal
  - Unter dem LOOP-Node: eingerückte Sub-Kette mit einem `TEXT`-Node
  - Kein Lock-Overlay sichtbar

---

### 3. Pan & Zoom

- Canvas-Hintergrund anklicken und ziehen → Canvas verschiebt sich
- Mausrad → Canvas zoomt (Cursor als Zoom-Zentrum)
- Zoom zwischen 0.3× und 2.0× clampiert (nicht weiter raus/rein)

---

### 4. Node verschieben

- Einen Node-Header anklicken und ziehen
- **Erwartung:** Node folgt der Maus; Verbindungslinien aktualisieren sich live

---

### 5. Verbindungslinien

- Bei 3 Top-Level-Nodes: 2 gestrichelte Bezier-Linien mit Pfeilspitze sichtbar
- Nach Verschieben eines Nodes zeigen Linien weiterhin korrekt auf die Ports

---

### 6. Felder editieren

- TEXT-Node: `value`-Feld anklicken → `{{zug1.zeit}}` editieren, z.B. in `{{zug1.gleis}}` ändern
- LOOP-Node: `var`-Feld → `item` ändern
- **Erwartung:** Feld akzeptiert Eingabe, kein Fehler

---

### 7. Nodes → YAML (Round-Trip)

- Felder editiert lassen → Tab `YAML` anklicken
- **Erwartung:** CodeMirror zeigt aktualisiertes YAML mit den geänderten Werten

---

### 8. Nodes → YAML Speichern

- Im NODES-Tab: `Speichern`-Button klicken (oder Shortcut)
- **Erwartung:** YAML wird aus Graph generiert und gespeichert; kein leeres `layers: []`

---

### 9. Kontextmenü — Node hinzufügen

- Rechtsklick auf leere Canvas-Fläche
- **Erwartung:** Kontextmenü mit Gruppen `LAYER` (image, rect, text, copy) und `LOOP` (loop)
- Einen Typ anklicken → neuer Node erscheint auf Canvas, am Ende der Kette angehängt

---

### 10. Node löschen

- `×`-Button in Node-Header klicken
- **Erwartung:** Node verschwindet; Verbindungslinien aktualisieren sich; gelöschter Node nicht mehr in YAML bei Tab-Wechsel

---

### 11. Lock-Overlay bei nicht unterstütztem YAML

- Andere Template-Datei öffnen, die `if:` auf Layer-Ebene enthält (z.B. aus `templates/default/template.yaml`)
- Tab `NODES` anklicken
- **Erwartung:** Lock-Overlay mit Hinweis `"Diese YAML enthält Features die im Node-Editor nicht darstellbar sind."` — Canvas nicht bedienbar

---

### 12. Lock verhindert Überschreiben

- Im gesperrten Zustand: `Speichern` klicken
- **Erwartung:** YAML bleibt unverändert (Original-YAML wird nicht mit leerem Graph überschrieben)

---

### 13. Port-Drag — Kette umsortieren

- Output-Port (unterer Kreis) eines Nodes anklicken und auf einen anderen Node ziehen
- **Erwartung:** Kette wird umsortiert; Linien zeigen neue Reihenfolge; Tab YAML spiegelt neue Layer-Reihenfolge wider

---

Alle 13 Checks bestanden → Branch bereit zum Merge.
