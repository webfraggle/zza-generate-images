# Vorschauseite: Live-Aktualisierung und ZZA-Vorschau (Desktop)

## Ziel

Die Vorschauseite (`GET /{template}`) der Desktop-App soll zwei Dinge können, die bisher nur der Editor kann:

1. Sich aktualisieren, wenn Dateien des Templates außerhalb der Seite geändert werden (KI über MCP, anderer Editor).
2. Zwischen der PNG/GIF-Vorschau (Go-Renderer) und der ZZA-Vorschau (Renderer-Kern des Controllers) wählen lassen.

Der Server-Build (gen.yuv.de) bleibt unverändert: Beides braucht die Desktop-Routen unter `/edit/`.

## Verhalten

- Beides erscheint nur, wenn `EditorEnabled` gesetzt ist (Desktop). Die Server-Seite enthält weiterhin keinen Verweis auf `/edit/`.
- **Engine-Auswahl:** Auswahlfeld „Vorschau“ über dem Bild, in einer Zeile mit dem Replay-Knopf (ohne Rahmen um die Zeile), mit „PNG/GIF“ (Standard) und „ZZA“. ZZA rendert über `POST /edit/{template}/zza-render[?format=gif]`. Die Wahl wird je Template in `localStorage` gemerkt, unter demselben Schlüssel wie im Editor (`zza-preview-engine:<template>`). „PNG herunterladen“ und „GIF herunterladen“ liefern das Bild der gewählten Engine. Fehler (422 „nicht ZZA-kompatibel“) stehen unter der Vorschau.
- **Live-Aktualisierung:** Long-Poll auf `GET /edit/{template}/events` wie im Editor. Die erste Antwort ist nur der Ausgangsstand. Bei einer Änderung:
  - `default.json` geändert (oder Reset) und der Nutzer hat auf der Seite nichts geändert: Seite neu laden, weil Formular und JSON vom Server kommen.
  - sonst: nur das Bild neu rendern, mit den Daten der Seite. Eingaben des Nutzers gehen nie verloren.
- 404 auf `/events` (kein Watcher) beendet das Polling still; andere Fehler: neuer Versuch nach 3 s.

## Nicht enthalten

- Kein Nachladen von Titel, Animations-Schaltern oder Export-Knopf ohne Seiten-Neuladen (ändert sich `template.yaml` so, zeigt es erst das nächste Laden).
- Keine Änderung an Server-Routen oder Go-Code außer einem Test.
