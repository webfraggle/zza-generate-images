# Plan: Detailseite mit Formular

Spec: `docs/superpowers/specs/2026-11-14-detail-form-ui-design.md`
Branch: `feature/detail-form-ui`

## Task 1 — `renderer.ReferencedPaths` (TDD)

- Test: YAML mit `value: "{{zug1.nr | lower}}"`, `if: "not(isEmpty(zug1.hinweis))"`,
  Kommentar `# gleis`, Key `mode:` → enthält `zug1.nr`, `zug1.hinweis`, nicht `gleis`, nicht `mode`.
- Test: alle Templates in `templates/` lassen sich parsen.
- Impl: `internal/renderer/fields.go`, yaml.Node-Walk über Skalar-Werte, Regex-Token.

## Task 2 — `server.buildForm` (TDD)

- Test: Reihenfolge entspricht default.json; nur verwendete Felder; Gruppentitel;
  `Kind` aus JSON-Typ; `Open` bei nicht-leeren Defaults; leere Gruppen entfallen.
- Test: Faltblatt → Bahnsteig(gleis) + Zug 1(zeit, nr, via, vonnach).
- Impl: `internal/server/form.go`.

## Task 3 — Handler

- `detailData` erweitert um `Form []formGroup`, `DefaultData any`.
- `handleDetail` liest template.yaml-Quelle über `renderer.SafeTemplatePath`.
- Test: GET /faltblatt-105-v1 enthält `name="zug1.vonnach"`, nicht `name="zug1.reihung"`.

## Task 4 — Frontend

- `web/templates/detail.html` neu: Gehäuse-Vorschau, Aktionsleiste, Formular, Entwickler-`<details>`.
- `web/static/app.css`: Detail-Abschnitt ersetzen (`.detail-*`).

## Task 5 — Review & Verifikation

- `go test ./...`, Browser-Check Desktop + Mobil.
- security-reviewer + code-reviewer auf Branch-Diff.
