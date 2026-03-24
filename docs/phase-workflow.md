# Phasen-Workflow

Dieser Ablauf ist bei **jeder Phase** einzuhalten. Keine Ausnahmen.

---

## Schritt 1 — Implementierung

- Der **implementer** Agent schreibt den Code gemäß Plan
- Basis ist immer `docs/implementation-plan.md` und `docs/yaml-template-spec.md`
- Bei Phase 8 zusätzlich: `legacy/` als Referenz, Analyse durch **template-porter**
- Code landet auf Branch `develop`

---

## Schritt 2 — Security Review

- Der **security-reviewer** Agent prüft den gesamten neuen Code der Phase
- Fokus auf die in `docs/implementation-plan.md` genannten Risiken je Phase
- Mögliche Ergebnisse:
  - **BLOCKIERT** → Implementierung stoppt, Findings werden behoben, Security Review wiederholt
  - **BEDINGT OK** → Findings werden behoben, dann weiter zu Schritt 3
  - **OK** → direkt weiter zu Schritt 3

---

## Schritt 3 — Code Review

- Der **code-reviewer** Agent prüft Qualität, Go-Idiome, Fehlerbehandlung
- Mögliche Ergebnisse:
  - **BLOCKIERT** → Implementierung stoppt, Findings werden behoben, Code Review wiederholt
  - **BEDINGT OK** → Findings werden behoben, dann weiter zu Schritt 4
  - **OK** → direkt weiter zu Schritt 4

---

## Schritt 4 — Commit

- Alle Änderungen der Phase werden committed
- Commit-Message enthält Phasen-Nummer und kurze Beschreibung
- Push auf `develop`

---

## Schritt 5 — Manuelle Testbeschreibung

- Claude erstellt eine klare, schrittweise Testanleitung für den User
- Format:
  ```
  ## Manueller Test — Phase X

  ### Voraussetzungen
  - Was muss gestartet/installiert sein

  ### Testfälle
  1. Schritt — erwartetes Ergebnis
  2. Schritt — erwartetes Ergebnis
  ...

  ### Bekannte Einschränkungen dieser Phase
  - Was noch nicht funktioniert (kommt in späterer Phase)
  ```
- Claude wartet danach auf Rückmeldung des Users

---

## Schritt 6 — User-Test & Feedback

- User führt die Testfälle durch
- Mögliche Rückmeldungen:
  - **OK** → weiter zu Schritt 7
  - **Bug gefunden** → Bug wird behoben, ab Schritt 2 wiederholen
  - **Verbesserungsidee** → Diskussion, ggf. in `docs/requirements-collection.md` ergänzen, dann entscheiden ob sofort oder später

---

## Schritt 7 — Abschluss der Phase

- Phase wird in `docs/implementation-plan.md` als ✅ markiert
- Erkenntnisse/Änderungen gegenüber dem Plan werden dort dokumentiert
- **Dokumentations-Check:** Sind alle `.md`-Dateien noch aktuell?
  - `CLAUDE.md` — Architektur, Befehle, Struktur noch korrekt?
  - `docs/requirements-collection.md` — neue Erkenntnisse eingearbeitet?
  - `docs/yaml-template-spec.md` — Änderungen an der Spec dokumentiert?
  - `docs/implementation-plan.md` — offene Punkte, Abweichungen notiert?
- **Memory-Check:** Sind die Memory-Dateien unter `~/.claude/projects/.../memory/` noch aktuell?
  - Neue Projektentscheidungen → `project_*.md`
  - Neues User-Feedback → `feedback_*.md`
  - Neue Referenzen → `reference_*.md`
- Commit + Push (inkl. aktualisierter Docs)
- Nächste Phase beginnt mit Schritt 1

---

## Visualisierung

```
┌─────────────────────────────────────────┐
│  Schritt 1: Implementierung             │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 2: Security Review             │
│  BLOCKIERT → zurück zu Schritt 1        │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 3: Code Review                 │
│  BLOCKIERT → zurück zu Schritt 1        │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 4: Commit + Push               │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 5: Manuelle Testbeschreibung   │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 6: User-Test & Feedback        │
│  Bug → zurück zu Schritt 1              │
│  OK → weiter                            │
└────────────────────┬────────────────────┘
                     ↓
┌─────────────────────────────────────────┐
│  Schritt 7: Phasen-Abschluss            │
│  - Docs (.md) aktualisieren             │
│  - Memory aktualisieren                 │
│  - Commit + Push                        │
│  → nächste Phase                        │
└─────────────────────────────────────────┘
```
