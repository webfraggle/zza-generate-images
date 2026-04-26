# Backlog

Ideen und Wünsche für spätere Features — noch nicht priorisiert, noch nicht geplant.

## Editor (Desktop)

- `YAML_FIELD_MAP` und `YAML_TO_DATA_KEY` aus `NODE_TYPES.fields` ableiten statt manuell pflegen (Wartungsfalle: Feld in NODE_TYPES ergänzen ≠ automatisch in YAML_FIELD_MAP). Erfordert `yamlKey`-Property in field-Defs oder Label-Konvention für `copy`-Labels (`src_w` vs. `src_width`).

## Renderer / Templates

- Animierte GIFs
- `and()`/`or()` Funktionen im Evaluator — mehrere Bedingungen in einem `if:` verknüpfen, z.B. `if: "and(not(isEmpty(zug1.via)), isEmpty(zug1.hinweis))"`. Workaround heute: verschachtelte Block-Nodes.

## Galerie / UI

- Skalierung der Bilder sauber 1×/2× (Detail-Seite zeigt aktuell 1:1 ohne Zoom).
- Preview YAML in der Detail-Seite anzeigen.

## Sicherheit / Härtung

- YAML-Alias-Bombe-Schutz im Editor-Save (`gopkg.in/yaml.v3` ohne `SetMaxAliasCount`-Härtung). Body ist auf 1 MiB begrenzt; weitere Mitigation falls jemals Multi-Tenant.
- CORS im Desktop-Build einschränken statt Wildcard `*` auf `/render`.

## Erledigt (Dual-Build, April 2026)

- ✅ ZIP-Download des kompletten Templates (`GET /{template}.zip`)
- ✅ UI in Exe mit WebView (Wails-v2-Bootstrap + Browser-Fallback)
- ✅ Server-Entschlackung: kein Auth, kein SMTP, kein SQLite, kein Admin
- ✅ Editor-Konsolidierung: `admin-editor.html` ist gelöscht, einziger Editor ist `edit-editor.html`
