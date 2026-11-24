package mcp

// serverInstructions is sent to the client in the initialize result.
const serverInstructions = `zza builds images for a model-railway departure display (Zugzielanzeiger).
Before creating or changing a template, call get_template_spec and get_design_guide and read them fully.

Hard rules:
- Only 1.05" displays: canvas 240x240, template names end in "-105-v1", max 24 characters in total.
- The image holds both sides of the display: top half (y 0-119) = side A, bottom half (y 120-239) = side B.
  Elements that belong to one end of the device (platform number, clock) sit on the OUTSIDE:
  left in the top half, right in the bottom half. Build the bottom half with several copy layers
  that swap the blocks; a single copy of the whole top half is only right for symmetric layouts.
- Flat look: solid colour areas, 2-5 strong colours, no gradients, shadows or photos; text >= 10 px.
- Everything that changes per train is a text layer with {{zug1.…}} placeholders, never baked into an image.
- Latin-1 only (U+0000-U+00FF) in all values and test data: no en/em dash (use -), no typographic
  quotes, no ellipsis (use ...), no bullet (use ·), no arrows, no €, no emoji. write_file refuses others.
- Device limits: 1.5 MiB core memory (images: drawn area x 4 bytes; fonts), 15 M operations per image.
  Fix every "device:" warning of write_file, else the display shows its built-in layout.

Workflow: look at 1-2 similar templates (list_templates, read_template, render_preview),
create_template, then after EVERY change call render_preview and compare with the reference.
Before finishing, test with long destination, delay and empty fields as described in the guide.
Tools by task: fonts: list_fonts -> add_font (text layers need a font; the starter has none).
Logos/shapes: svg_to_png (no text). Photo of a real display: import_image (ask the user for the file path)
-> perspective_correct -> sample_colors -> crop_image / color_to_alpha / quantize_colors.
Images referenced in template.yaml must exist before write_file (it test-renders).
`
