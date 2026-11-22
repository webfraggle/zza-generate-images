# Manueller Testplan: Vorschauseite live und ZZA-Vorschau

Voraussetzung: Desktop-App (GUI-Fenster) mit einem ZZA-kompatiblen Template, z. B. `umuc-105-v1`.

1. Vorschauseite öffnen: In der Aktionsleiste steht „Vorschau“ mit „PNG/GIF“ und „ZZA“.
2. Auf „ZZA“ stellen: Das Bild wird neu gerendert (bei animierten Templates spielt die Animation). Seite neu laden: „ZZA“ ist noch gewählt. Editor desselben Templates öffnen: Dort steht die Engine ebenfalls auf „ZZA“.
3. „PNG herunterladen“ bei „ZZA“: Die Datei zeigt das ZZA-Bild.
4. Nicht ZZA-kompatibles Template auf „ZZA“ stellen: Unter der Vorschau steht „nicht ZZA-kompatibel: …“.
5. KI über MCP (oder ein Texteditor) ändert `template.yaml`: Das Bild aktualisiert sich ohne Zutun, Formularwerte bleiben.
6. KI ändert `default.json`, ohne dass auf der Seite etwas eingegeben wurde: Die Seite lädt neu und zeigt die neuen Beispieldaten.
7. Erst ein Formularfeld ändern, dann ändert die KI `default.json`: Die Eingabe bleibt stehen, nur das Bild wird neu gerendert.
8. gen.yuv.de bzw. `zza-server`: Die Vorschauseite hat kein Auswahlfeld „Vorschau“.
