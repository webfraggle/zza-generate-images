# ZZA-Paket online herunterladen – Plan

Spec: `docs/superpowers/specs/2026-11-22-zza-online-export-design.md`. Branch `feature/zza-online-export`.

Jede Aufgabe: Test zuerst (rot), dann Code (grün). Tests in `internal/server/zzaexport_test.go`.

1. **Route und Paket** – `GET /{template}/zza.zip` liefert das Paket des Exporters, bytegleich
   (`TestZZAExport_ServesThePackageOfTheExporter`); Fehlerfälle 400/404/422, kein Export per POST
   (`TestZZAExport_Errors`); Source-ZIP unverändert (`TestZZAExport_SourceZipStillWorks`); http → https
   (`TestZZAExport_RedirectsHTTPToHTTPS`).
2. **Cache** – zweiter Abruf aus dem Cache, Paket im eigenen Ordner
   (`TestZZAExport_SecondRequestComesFromTheCache`); geändertes Template wird neu exportiert
   und ersetzt das alte Paket (`TestZZAExport_ChangedTemplateIsExportedAgain`); inkompatibles Ergebnis wird
   gemerkt (`TestZZAExport_IncompatibleResultIsRemembered`); `/zza.zip` bleibt Source-ZIP
   (`TestZZAExport_TemplateNamedZZAKeepsItsSourceZip`); eigene Grenze (`TestZZACacheHonoursItsOwnLimit`);
   Bild-Cache lässt den Unterordner in Ruhe (`TestImageCacheIgnoresZZASubfolder`).
3. **Gleichzeitigkeit** – belegte Plätze ergeben 503 mit `Retry-After` (`TestZZAExport_BusySlotsAnswer503`).
4. **Detailseite** – Knopf in beiden Builds, ausgegraut mit Gründen bei inkompatiblem Template
   (`TestDetailPage_ZZAButton`, `TestServer_Detail`).
5. **Konfiguration und Doku** – `ZZA_CACHE_MAX_SIZE_MB` (`internal/config`, `.env.example`,
   `docker-compose.ionos.yml`), README, CLAUDE.md.
6. **Prüfung** – `go test ./...`, Server-Abhängigkeiten (Befehl in CLAUDE.md), Lauf im Container mit
   `read_only`, Vergleich mit `zza export`; danach manueller Testplan.

Vor dem Merge: security-reviewer und code-reviewer auf den Branch-Diff.
