# Dual-Build Manual Test Plan

Work through every checkbox before merging `feature/dual-build` → `develop`. Do not proceed on any failing item.

## Server build (`cmd/zza-server`, Docker)

Start locally (outside Docker for speed):
```
PORT=18080 TEMPLATES_DIR=./templates ./dist/zza-server serve &
```

- [ ] `GET /health` → 200, body "ok"
- [ ] `GET /` lists templates, nav shows only "Galerie" (no "Admin", no "+ Neues Template")
- [ ] `GET /sbb-096-v1` preview page loads, shows:
  - [ ] JSON textarea
  - [ ] PNG preview (live render)
  - [ ] "Als ZIP" button
  - [ ] "PNG herunterladen" button
  - [ ] NO "Bearbeiten" button
  - [ ] NO email modal anywhere on the page
- [ ] `GET /sbb-096-v1.zip` downloads a valid .zip containing `template.yaml`, `default.json`, and all assets/fonts
- [ ] `POST /sbb-096-v1/render` with a valid JSON body returns PNG, `Content-Type: image/png`
- [ ] `GET /edit/sbb-096-v1` → 404
- [ ] `GET /admin` → 404
- [ ] `GET /sbb-096-v1/edit` → 404
- [ ] `POST /sbb-096-v1/request-token` → 404
- [ ] `POST /create-new` → 404
- [ ] HTTPS redirect: set `X-Forwarded-Proto: http` → 301 to `https://…` for `/`, `/sbb-096-v1`, `/sbb-096-v1.zip`
- [ ] HTTPS redirect: `X-Forwarded-Proto: http` on `POST /sbb-096-v1/render` is NOT redirected (stays http, returns PNG)

## Desktop build — GUI (`cmd/zza`)

Double-click `zza.app` (unzipped from `dist/release/zza-macos-arm64.zip` on a fresh folder):
- [ ] First run: Gatekeeper warning appears. Right-click → Öffnen unlocks it.
- [ ] Native window opens. Gallery page is visible.
- [ ] Templates list appears, one card per template.
- [ ] Click a template card → preview page loads, "Bearbeiten" button is visible.
- [ ] Click "Bearbeiten" → editor page loads with `template.yaml` + `default.json` in the editor.
- [ ] Edit YAML (e.g. change a text), click Speichern → no error, preview refreshes with change.
- [ ] Edit YAML to broken syntax (`a: : :`), click Speichern → 400 error shown, file on disk NOT modified.
- [ ] Upload a `.png` (e.g. 8 KB logo) via the file-upload widget → file appears in the editor's file list.
- [ ] Delete the uploaded asset → file removed, no error.
- [ ] Try to delete `template.yaml` → blocked with "forbidden" or equivalent error.
- [ ] Click "Als ZIP" on a preview page → ZIP downloads via Wails native download.
- [ ] Close window → process exits cleanly.

## Desktop build — Browser fallback

Force the fallback path (simulate a missing WebView by running `zza serve --port 0` or via a clean macOS user without Safari trust):
- [ ] Log line appears: `wails unavailable (...) — falling back to default browser` OR `zza editor running at http://127.0.0.1:<port>`.
- [ ] Default browser opens to the editor URL.
- [ ] Same functional tests as GUI mode (edit, save, upload, delete, ZIP).
- [ ] Ctrl+C in terminal stops the server.

## Desktop build — CLI

- [ ] `zza version` → prints version (e.g. `v0.1.13` after build.sh has bumped it).
- [ ] `zza render -t sbb-096-v1 -i /path/to/test.json -o /tmp/out.png --templates-dir /path/to/repo/templates` → PNG generated.
- [ ] `zza serve --port 9000` → server responds on :9000, no window opens.
- [ ] `zza --templates-dir /tmp/custom` (GUI default) → templates folder at `/tmp/custom` is created and seeded with `default/`.

## Templates folder resolution

- [ ] First run with no `templates/` next to binary: folder is created, `default/` sub-dir with `template.yaml` + `default.json` is seeded.
- [ ] Existing `templates/` next to binary: contents preserved, no overwrite.
- [ ] `--templates-dir /absolute/path` override honoured (check the log line `templates: /absolute/path`).
- [ ] `.app` bundle on macOS: templates folder is placed as sibling of `zza.app/`, not inside `zza.app/Contents/MacOS/`.

## Release ZIPs

For each of `zza-macos-arm64.zip`, `zza-macos-intel.zip`, `zza-windows-x64.zip`:
- [ ] Unzips cleanly to a fresh folder.
- [ ] Contains the binary / `.app` bundle.
- [ ] Contains the full `templates/` folder (all ~15 curated themes: sbb-096-v1, oebb-096-v1, nederland-096-v1, etc.).
- [ ] `README.txt` readable, mentions the right Gatekeeper / SmartScreen steps.
- [ ] macOS ZIP binary is Mach-O executable (`file zza.app/Contents/MacOS/zza`).
- [ ] Windows ZIP binary is `PE32+ executable (GUI) x86-64`.

## Docker server image

```
docker run --rm -d --name zza-test -p 18082:8080 \
    -v "$(pwd)/templates:/data/templates:ro" \
    -e TEMPLATES_DIR=/data/templates -e CACHE_DIR=/data/cache \
    "ghcr.io/webfraggle/zza-generate-images:$(cat VERSION)"
```

- [ ] Container starts without error.
- [ ] Logs contain `zza-server listening on :8080 (templates: /data/templates, cache: /data/cache)`.
- [ ] No "opening database" or "SMTP" log lines at startup.
- [ ] `curl -sf http://localhost:18082/health` → `ok`.
- [ ] `curl -sI http://localhost:18082/sbb-096-v1.zip` → 200 + `application/zip`.
- [ ] `curl -sI http://localhost:18082/edit/sbb-096-v1` → 404.
- [ ] Old stale `zza.db` on the host is ignored (no DB mount, no crash).
- [ ] `docker stop zza-test` succeeds cleanly.

## Regression checks

- [ ] `go test ./...` passes on `feature/dual-build`.
- [ ] Both `CGO_ENABLED=1 go build ./cmd/zza` and `CGO_ENABLED=0 go build ./cmd/zza-server` succeed.
- [ ] `go list -deps ./cmd/zza-server | grep -i wails` → empty.
- [ ] No SQLite dependency anywhere: `go list -deps ./... | grep -i sqlite` → empty.

## Spec coverage sanity check

- [ ] Server routes match the spec: `/`, `/{template}`, `/{template}/preview`, `/{template}/render`, `/{template}.zip`, `/health`.
- [ ] Desktop routes additionally include: `/edit/{template}`, `/edit/{template}/files`, `/edit/{template}/file/{filename}`, `/edit/{template}/save`, `/edit/{template}/upload`, `/edit/{template}/file/{filename}` (DELETE).
- [ ] No admin routes, no request-token routes, no create-new routes in either build.
