# Node-Editor Phase 2 — Manueller Testplan

**Branch:** `feature/node-editor-phase2`
**Vorbedingung:** Server läuft, Template `node-test-phase1` (oder ein anderes mit `template.yaml`) ist unter `/node-test-phase1/edit` erreichbar.

Alle Phase-1-Checks gelten weiterhin (Tab-Umschalter, Pan/Zoom, Drag, Kette umsortieren, Lock-Overlay, etc.). Hier werden nur die neuen Phase-2-Features getestet.

---

## 1. Layer if-Badge

**Setup:** Im NODES-Tab, TEXT-Node vorhanden.

- Im Node-Body ist die **erste Zeile** ein kleines Dropdown `[— | if | elif | else]`
- Dropdown auf `if` setzen → rechts daneben erscheint ein Textfeld für die Bedingung
- Bedingung eingeben: `not(isEmpty(zug1.hinweis))`
- Tab `YAML` anklicken → Layer enthält `if: not(isEmpty(zug1.hinweis))`
- Zurück zu `NODES` → Dropdown zeigt `if`, Bedingung korrekt gesetzt

**elif / else:**
- Dropdown auf `elif` → Bedingungsfeld sichtbar
- Dropdown auf `else` → Bedingungsfeld ausgeblendet (kein Feld nötig)
- Tab YAML: `elif:` bzw. `else: true` im Layer

**Kein Badge:**
- Dropdown auf `—` → kein `if`/`elif`/`else` in YAML

---

## 2. Feld-if (color mit if/then/else)

**Setup:** RECT- oder TEXT-Node auf Canvas.

- Das `color`-Feld zeigt rechts neben dem Farbpicker einen kleinen `[if]`-Button
- `[if]` anklicken → drei Zeilen erscheinen:
  - `color if`: Textfeld für die Bedingung
  - `then`: Farbpicker für den Wahr-Fall
  - `else`: Farbpicker für den Falsch-Fall
- Bedingung eingeben: `greaterThan(zug1.abw, 0)`, Then: `#FF4444`, Else: `#FFFFFF`
- Tab YAML → `color: {if: 'greaterThan(zug1.abw, 0)', then: '#FF4444', else: '#FFFFFF'}`
- `[×]`-Button klickt man: zurück auf einfachen Farbpicker (`then`-Farbe als neuer Wert übernommen)

---

## 3. Filter-Pipeline-Chips

**Setup:** TEXT-Node; Test-JSON-Feld rechts enthält `{"zug1":{"hinweis":"* Abweichung","nr":"icn"}}`.

### 3a. Filter hinzufügen und Vorschau prüfen

- `value`-Feld auf `{{zug1.hinweis}}` setzen (direkt tippen oder via YAML-Tab)
- Unter dem `value`-Feld erscheint eine **Chip-Zeile** mit `[+]`-Button
- `[+]` anklicken → Dropdown mit Gruppen Text / Mathe / Zeit
- `strip(…)` wählen → Argument-Eingabe erscheint → `'*'` eingeben → Enter
- Chip `strip('*')` erscheint in der Zeile
- **Vorschau:** `→  Abweichung` (Leerzeichen + Text, Sternchen entfernt)
- Zweiten Filter: `upper` (kein Argument) → `→  ABWEICHUNG`
- Tab YAML: `value: "{{zug1.hinweis | strip('*') | upper}}"`

### 3b. Filter-Chip entfernen

- `[×]` am Chip → Chip verschwindet, Vorschau aktualisiert sich

### 3c. Filter-Reihenfolge per Drag

- Zwei Chips vorhanden (z.B. `strip('*')` und `upper`)
- `strip('*')`-Chip auf `upper` ziehen → Reihenfolge tauscht sich
- YAML-Wert und Vorschau spiegeln neue Reihenfolge wider

### 3d. Vorschau aus Test-JSON

- Test-JSON-Feld leeren → Vorschau-Zeile leer
- JSON wieder einfügen → Vorschau erscheint sofort (kein Seiten-Reload)

### 3e. image.rotate-Feld

- IMAGE-Node: `rotate`-Feld enthält ebenfalls Chip-Zeile
- `{{now.minute | mul(6)}}` via YAML einlesen → Node zeigt base `{{now.minute}}` + Chip `mul(6)`
- Test-JSON: `{"now":{"minute":30}}` → Vorschau `→ 180`

---

## 4. BLOCK-Node

### 4a. Node erstellen

- Rechtsklick auf leere Canvas-Fläche → Kontextmenü hat neue Gruppe `BLOCK` mit drei Einträgen: `BLOCK-IF`, `BLOCK-ELIF`, `BLOCK-ELSE`
- `BLOCK-IF` wählen → neuer Node erscheint mit **orangem Rand**, Badge `[IF]` im Header
- Bedingungsfeld: `startsWith(nr,'ICN')` eingeben
- Tab YAML → `block: "startsWith(nr,'ICN')"` + `layers: []`

### 4b. BLOCK-ELIF und BLOCK-ELSE

- `BLOCK-ELIF` erstellen → Badge `[ELIF]`, dunkleres Orange
- `BLOCK-ELSE` erstellen → Badge `[ELSE]`, kein Bedingungsfeld sichtbar

### 4c. Body-Nodes in BLOCK-Kette

- IMAGE-Node in BLOCK-IF ziehen (via Port-Drag vom Block-Node auf den IMAGE-Node)
- **Erwartung:** IMAGE-Node wird Body-Node des BLOCK; orangefarbene Bögen verbinden Block → IMAGE → zurück zum Block
- Tab YAML → `block: ..., layers: [{type: 'image', ...}]`

### 4d. Verbindungen

- Hauptkette (BLOCK-IF → BLOCK-ELIF → BLOCK-ELSE) durch graue Linien verbunden
- Jeder BLOCK hat eigene orangefarbene Body-Bogenkurve (wie Loop in Rot)

---

## 5. Roundtrip — bestehende Phase-2-YAML laden

**Setup:** Template mit folgender YAML (direkt in CodeMirror eingeben):

```yaml
layers:
  - type: text
    if: not(isEmpty(zug1.hinweis))
    value: "{{zug1.hinweis | strip('*') | upper}}"
    x: 10
    y: 10
  - type: rect
    color:
      if: greaterThan(zug1.abw, 0)
      then: '#FF4444'
      else: '#FFFFFF'
    x: 0
    y: 0
    width: 100
    height: 20
  - block: "startsWith(nr,'ICN')"
    layers:
      - type: image
        file: icn.png
  - else: true
    layers:
      - type: text
        value: "{{nr}}"
```

- Tab `NODES` anklicken → **kein Lock-Overlay**
- 4 Nodes sichtbar: TEXT (mit if-Badge), RECT (mit orangem if-Button am color-Feld), BLOCK-IF, BLOCK-ELSE
- TEXT-Node: Dropdown zeigt `if`, Bedingung korrekt, Chip `strip('*')` + `upper` vorhanden
- RECT-Node: color-Feld zeigt 3-zeiligen if/then/else-Modus
- BLOCK-IF: Badge `[IF]`, Bedingung `startsWith(nr,'ICN')`, Body-Kette mit IMAGE-Node
- BLOCK-ELSE: Badge `[ELSE]`, kein Bedingungsfeld

**Keine Änderung machen → Tab YAML → YAML identisch mit Original (strukturell)**

---

## 6. Regression — Phase-1-Features weiterhin funktional

- Normaler TEXT/IMAGE/LOOP-Node ohne Phase-2-Features: kein Chip-Row-Overlay, kein unerwünschtes `if:`
- Loop-Verbindungen weiterhin rot, Block-Verbindungen orange, Hauptkette grau
- Nodes verschieben, löschen, Kette umsortieren: wie Phase 1

---

Alle 6 Testgruppen bestanden → Branch bereit für Security- und Code-Review, danach Merge in `develop`.
