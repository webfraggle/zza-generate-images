# Legacy-Dokumentation

Diese Dokumente beschreiben den ursprünglichen 10-Phasen-Plan für den Go-Rewrite (PHP → Go) sowie den damals geltenden Phasen-Workflow und die Security-Audits der inzwischen entfernten Auth-/Edit-/Admin-Routen.

**Aktueller Stand:** Mit dem Dual-Build-Refactor (April 2026) sind Editor, Auth, Admin, SMTP und SQLite aus dem Server-Build entfernt; der Editor läuft nur noch in der Desktop-App. Diese Dokumente sind nicht mehr operativ relevant, werden aber als Historie aufbewahrt.

| Datei | Inhalt |
|---|---|
| `implementation-plan.md` | 10-Phasen-Plan vom Original-Go-Rewrite |
| `requirements-collection.md` | Anforderungen aus der Planungsphase 2026-03 |
| `phase-workflow.md` | Pflicht-Ablauf je Phase (Implementer + Security + Code-Review + manueller Test) |
| `security-findings.md` | Audit der `/create-new` und `/request-token` Flows (alle Routen wurden später entfernt) |

Aktuelle Specs und Pläne: `docs/superpowers/specs/` und `docs/superpowers/plans/`.
