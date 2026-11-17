// Text layer: measuring, wrapping, fit_items, alignment and glyph drawing.
// Port of renderText, wrapText, layoutFitItems and drawTextLine of
// internal/renderer with the device glyph sets (see devexport doc.go,
// "Text layout on the device").
#pragma once
#include <string>

#include "canvas.h"
#include "package.h"

namespace zza {

// maxTextRunes of internal/renderer.
constexpr size_t kMaxTextRunes = 4096;

// capText: truncates UTF-8 text to kMaxTextRunes runes (invalid bytes count as one).
std::string capText(std::string s);

enum TextAlign { kAlignLeft, kAlignCenter, kAlignRight };
enum TextValign { kValignTop, kValignMiddle, kValignBottom, kValignBaseline };

// The resolved layer fields renderText uses.
struct TextParams {
  int x, y, width, height, maxWidth;
  TextAlign align;
  TextValign valign;
  double lineHeight;  // multiplier; 0 or 1 = font height
  double scaleX;      // as in the template (0 = unset)
  double scaleY;
  bool fitItems;
  std::string fitSplitBy;  // UTF-8
  int fitGap;
};

// Draws a text layer. text is the interpolated, capped UTF-8 value. Returns
// the widest line's on-canvas width (for <id>.width; 0 with fit_items).
int drawTextLayer(Canvas& c, const Font& f, const TextParams& p, const std::string& text, Rgba col);

}  // namespace zza
