# ÖBB/RhB Legacy-Parität + animierte Galerie — Manueller Testplan

Start: `go run ./cmd/zza-server serve` (oder Desktop-Build), Browser auf die Galerie.

## Galerie

- [ ] Faltblatt, Nederland, U-Muc und ÖBB zeigen ein laufendes GIF; RhB und SBB ein Standbild.
- [ ] Animierte Karten haben oben rechts einen ↻-Button, nicht animierte keinen.
- [ ] Klick auf ↻ startet die Animation neu (Faltblatt blättert erneut) und öffnet **nicht** die Detailseite.
- [ ] Klick auf die Karte außerhalb des Buttons öffnet die Detailseite.
- [ ] Galerie mehrfach schnell neu laden: jede Karte zeigt ein Bild (GIF oder PNG-Fallback), keine kaputten Bilder.

## ÖBB (Detailseite, JSON ändern)

- [ ] Standard-JSON sieht aus wie das PHP-Referenzbild (S-Logo + 1, `15:53 16:03 Neulengbach`, `Wien Penzing~Wien Hüttendorf`, gelber Balken mit schwarzem Text).
- [ ] „Animation abspielen": Uhr rechts oben zählt Sekunden hoch, Minutenwechsel stimmt.
- [ ] `abw: 0` → keine gelbe Zeit, Ziel rückt direkt hinter die Abfahrtszeit.
- [ ] `hinweis` ohne `*` → weißer Text, kein Balken. `hinweis` leer → nichts.
- [ ] `nr: "REX1234"` → `REX` fett, `1234` leicht. `nr: "RJ"` → `RJ` fett. `nr: "s3"` → S-Logo + 3.
- [ ] `zeit: ""` → Ziel beginnt am linken Rand. `zeit: "sofort"` → Text wird unverändert gezeigt.
- [ ] `zeit: "23:55"`, `abw: 10` → gelbe Zeit `00:05`.

## RhB

- [ ] Standard-JSON sieht aus wie das PHP-Referenzbild (Punktmatrix-Feld mit `RE`, Vias dahinter, große Zeile `11:30 St. Moritz`).
- [ ] `nr: "R"` / `"IR"` → Punktmatrix-Feld schmaler/breiter, Vias rücken mit.
- [ ] `via: ""` → keine Vias, kein Fehler. Fünf Vias → laufen rechts aus dem Bild, kein Umbruch.
- [ ] `vonnach: "Zürich HB"` → Umlaut korrekt.

## Regression

- [ ] Faltblatt, Nederland, SBB, U-Muc sehen aus wie vorher.
- [ ] `POST /{template}/render` (PNG und `?format=gif`) liefert für ÖBB und RhB 200.
