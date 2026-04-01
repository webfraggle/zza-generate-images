# Backlog

Ideen und Wünsche für spätere Features — noch nicht priorisiert, noch nicht geplant.

---

## Editor

<!-- Ideen für den Template-Editor -->
- Nodebasierter Editor → Design-Spec: `docs/superpowers/specs/2026-04-01-node-editor-design.md`
- `YAML_FIELD_MAP` und `YAML_TO_DATA_KEY` aus `NODE_TYPES.fields` ableiten statt manuell pflegen (Wartungsfalle: Feld in NODE_TYPES ergänzen ≠ automatisch in YAML_FIELD_MAP) — erfordert `yamlKey`-Property in field-Defs oder Label-Konvention für `copy`-Labels (`src_w` vs `src_width`)
- `edit-editor.html` und `admin-editor.html` konsolidieren — ~96 % Code-Duplikat; Unterschied ist nur Header-Text und API-Basis-URL (`/edit/${TOKEN}/…` vs. `/admin/${TEMPLATE}/…`). Ansätze: gemeinsames JS-Modul (`editor.js`) oder ein einzelnes Go-Template mit `{{if .IsAdmin}}`.

---

## Renderer / Templates

<!-- Ideen für neue Layer-Typen, YAML-Syntax, Filter, etc. -->
- Animierte Gifs
- `and()`/`or()` Funktionen im Evaluator — mehrere Bedingungen in einem `if:` verknüpfen, z.B. `if: "and(not(isEmpty(zug1.via)), isEmpty(zug1.hinweis))"`. Workaround heute: verschachtelte Block-Nodes.

---

## Server / API

<!-- Ideen für neue Endpunkte, Caching, Performance etc. -->

---

## Galerie / UI

<!-- Ideen für die öffentliche Galerie und den Ausprobiermodus -->
- Skalierung der Bilder sauber 1x oder 2x (Detail-Seite zeigt aktuell 1:1 ohne Zoom)
- Preview YAML
- Download des gesamten Themes (als ZIP)

---

## Deployment / Infrastruktur

<!-- Ideen für Docker, CI/CD, Monitoring etc. -->

---

## Sonstiges

<!-- Alles was nicht passt -->

- UI in Exe mit Webview
