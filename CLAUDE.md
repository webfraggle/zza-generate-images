# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development

Start the local PHP development server:
```bash
node php-server.mjs
```

The app is then accessible at the local URL provided. No build step required.

## Architecture

This is a web-based PNG image generator for railway display signs (Zugzielanzeiger). It renders images that mimic real-world departure boards for various European railway operators.

**Stack:**
- Frontend: `index.html` + `index.js` (jQuery + Bootstrap 5)
- Backend: PHP + GD image library (one `index.php` per theme)
- Dev server: Node.js wrapping PHP's built-in server (`php-server.mjs`)

**Request flow:**
1. `index.js` loads `config.json` to populate the theme selector
2. On theme selection, it loads that theme's `default.json` to pre-fill the form
3. On submit, it POSTs JSON train data to `[theme]/index.php`
4. PHP renders a PNG via GD and returns it directly (with caching via SHA1 hash of input)

## Theme Structure

Each of the 14 themes lives in its own directory (e.g. `oebb-096-v1/`, `sbb-105-v1/`, `umuc-096-v1/`). Every theme directory contains the same files:

| File | Purpose |
|------|---------|
| `index.php` | Receives POST JSON → renders and returns PNG |
| `default.json` | Default train data for the form pre-fill |
| `gfx_functions.inc.php` | GD drawing helpers (text, wrapping, resizing) |
| `cors.inc.php` | CORS headers |
| `fonts/` | TrueType fonts (not in git — must be provided separately) |
| `img/` | Theme background and logo images |
| `cache/` | Auto-generated PNG cache (not in git) |

The directory name encodes: `[operator]-[display-size-inches]-[version]`
Display sizes in use: `096` (0.96"), `105` (1.05"), `114` (1.14").

## Key Data Schema

POST body JSON sent to each `index.php`:
```json
{
  "gleis": "3",
  "mode": 1,
  "zug1": { "vonnach": "Destination", "nr": "S1", "zeit": "15:53", "via": "...", "abw": 0, "hinweis": "" },
  "zug2": { ... },
  "zug3": { ... }
}
```

Theme-specific `default.json` files show all available fields.

## Adding a New Theme

Copy an existing theme directory, update `img/` assets and font references in `index.php`, then register the theme in `config.json`.
