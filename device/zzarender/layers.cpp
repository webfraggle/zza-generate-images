#include <algorithm>
#include <climits>
#include <cmath>
#include <cstring>
#include <vector>

#include "numfmt.h"
#include "render_impl.h"
#include "text.h"
#include "utf8.h"

namespace zza {

using rd::coord;
using rd::kMaxBlockDepth;
using rd::kMaxCanvasDimension;
using rd::num;
using rd::plainInt;
using rd::RenderError;
using rd::resolveCond;
using rd::str;

namespace {

// Limits of internal/renderer.
constexpr int kMaxLayers = 256;
constexpr int kDefaultMaxLoopItems = 20;
constexpr int kMaxLoopItemsHardCap = 200;
constexpr int kMaxTextVar = 1 << 24;
constexpr int kMaxAdvance = 240;
// Plain integers from t.json are clamped so coordinate sums cannot overflow.
constexpr long long kMaxPlainInt = 1LL << 30;
// Dynamically scaled images up to this many pixels are cached whole; larger
// ones are scaled only where they are visible, per render.
constexpr long long kCacheMaxPx = 240LL * 240;

int hexNib(char c) {
  if (c >= '0' && c <= '9') return c - '0';
  if (c >= 'a' && c <= 'f') return c - 'a' + 10;
  if (c >= 'A' && c <= 'F') return c - 'A' + 10;
  return -1;
}

}  // namespace

namespace rd {

const char* str(JsonObjectConst o, const char* k) {
  JsonVariantConst v = o[k];
  return v.is<const char*>() ? v.as<const char*>() : "";
}

double num(JsonObjectConst o, const char* k) {
  JsonVariantConst v = o[k];
  return v.is<double>() || v.is<long long>() ? v.as<double>() : 0;
}

int plainInt(JsonVariantConst v) {
  if (!v.is<long long>()) return 0;
  long long n = v.as<long long>();
  return (int)(n > kMaxPlainInt ? kMaxPlainInt : n < -kMaxPlainInt ? -kMaxPlainInt : n);
}

// IntOrExpr.Resolve: an integer, or "{{expr}}" evaluated with the scope's
// integer variables (errors are 0, like Go).
int coord(JsonObjectConst o, const char* k, const Eval& ev) {
  JsonVariantConst v = o[k];
  if (!v.is<const char*>()) return plainInt(v);
  std::string_view s = trimSpace(v.as<const char*>());
  if (s.size() < 4 || s.substr(0, 2) != "{{" || s.substr(s.size() - 2) != "}}") return 0;
  bool ok = false;
  int r = ev.evalInt(std::string(trimSpace(s.substr(2, s.size() - 4))), &ok);
  return ok ? r : 0;
}

// StringOrCond.Resolve: a string, or {"_cond":[[k,v],…]}: the first if/elif
// whose condition holds gives its then-value, else the (last) else-value.
std::string resolveCond(JsonVariantConst v, const Eval& ev) {
  if (v.is<const char*>()) return v.as<const char*>();
  JsonArrayConst pairs = v["_cond"];
  std::string pending, els;
  bool has = false;
  for (JsonArrayConst p : pairs) {
    const char* key = p[0].is<const char*>() ? p[0].as<const char*>() : "";
    const char* val = p[1].is<const char*>() ? p[1].as<const char*>() : "";
    if (!strcmp(key, "if") || !strcmp(key, "elif")) {
      pending = val;
      has = true;
    } else if (!strcmp(key, "then")) {
      if (has && ev.cond(pending)) return val;
      has = false;
    } else if (!strcmp(key, "else")) {
      els = val;
    }
  }
  return els;
}

}  // namespace rd

namespace {

// parseColor: #RRGGBB or #RRGGBBAA after TrimSpace.
Rgba parseColor(const std::string& in) {
  std::string_view s = trimSpace(in);
  if (s.empty()) throw RenderError("parseColor: empty color string");
  if (s[0] != '#') throw RenderError("parseColor: color \"" + std::string(s) + "\" must start with #");
  std::string_view hex = s.substr(1);
  if (hex.size() != 6 && hex.size() != 8)
    throw RenderError("parseColor: color \"" + std::string(s) + "\" has invalid length");
  uint8_t v[4] = {0, 0, 0, 255};
  for (size_t i = 0; i < hex.size(); i += 2) {
    int a = hexNib(hex[i]), b = hexNib(hex[i + 1]);
    if (a < 0 || b < 0) throw RenderError("parseColor: invalid hex color \"" + std::string(s) + "\"");
    v[i / 2] = (uint8_t)(a * 16 + b);
  }
  Rgba c;
  c.r = v[0];
  c.g = v[1];
  c.b = v[2];
  c.a = v[3];
  return c;
}

}  // namespace

Renderer::Impl::Impl(ZzaBudget* b, const Package* p)
    : budget(b), pkg(p), cache{CacheEnt(b), CacheEnt(b), CacheEnt(b), CacheEnt(b)}, tick(0), canvas(),
      layerCount(0), pass(kPassStill), frame(0), flapLayerSeq(0), nFlapRects(0), flapRects(), modCursor(0),
      snapAt(-1) {}

void Renderer::Impl::clearCache() {
  for (CacheEnt& e : cache) {
    e.src = nullptr;
    ZzaBytes(ZzaAlloc<uint8_t>(budget)).swap(e.img.px);
  }
}

const Img& Renderer::Impl::cachedScale(const Img* src, int w, int h) {
  tick++;
  for (CacheEnt& e : cache) {
    if (e.src == src && e.w == w && e.h == h) {
      e.used = tick;
      return e.img;
    }
  }
  CacheEnt* slot = &cache[0];  // a free slot, else the least recently used
  for (CacheEnt& e : cache) {
    if (!e.src) {
      slot = &e;
      break;
    }
    if (e.used < slot->used) slot = &e;
  }
  slot->src = nullptr;  // stays free if scaling fails
  ZzaBytes(ZzaAlloc<uint8_t>(budget)).swap(slot->img.px);
  slot->img.px.resize((size_t)w * h * 4);
  scaleCatmullRom(budget, *src, w, h, 0, 0, w, h, slot->img.px.data());
  slot->img.w = (uint16_t)w;
  slot->img.h = (uint16_t)h;
  slot->src = src;
  slot->w = w;
  slot->h = h;
  slot->used = tick;
  return slot->img;
}

void Renderer::Impl::beginPass(uint8_t* px, Pass p) {
  canvas.w = rd::kCanvasW;
  canvas.h = rd::kCanvasH;
  canvas.px = px;
  layerCount = 0;
  pass = p;
  flapLayerSeq = 0;
  nFlapRects = 0;
  modCursor = 0;
}

int Renderer::Impl::parseData(const char* data, size_t len, std::unique_ptr<BudgetJsonDoc>& doc,
                              JsonVariantConst& root, std::string& err) {
  if (len > kMaxDataBytes) {
    err = "data JSON larger than " + std::to_string(kMaxDataBytes) + " bytes";
    return ZZA_ERR_DATA;
  }
  doc.reset();
  root = JsonVariantConst();
  if (data && len > 0) {
    const uint8_t* d = reinterpret_cast<const uint8_t*>(data);
    doc.reset(new BudgetJsonDoc(jsonCapacity(d, len), JsonAlloc(budget)));
    DeserializationError e = deserializeJson(*doc, data, len, DeserializationOption::NestingLimit(32));
    if (e == DeserializationError::NoMemory) throw std::bad_alloc();
    if (e) {
      err = std::string("invalid data JSON: ") + e.c_str();
      return ZZA_ERR_DATA;
    }
    if (!doc->is<JsonObject>() && !doc->isNull()) {
      err = "data JSON is not an object";
      return ZZA_ERR_DATA;
    }
    root = doc->as<JsonVariantConst>();
  }
  return ZZA_OK;
}

Eval Renderer::Impl::rootEval(JsonVariantConst root, const zza_time& now) const {
  Eval ev(root, now);
  ev.setRegexSource(pkg->regexSource());
  JsonObjectConst an = pkg->tmpl()["meta"]["animation"];
  int frames = plainInt(an["frames"]);
  if (frames > 0) {
    double fd = num(an, "frame_delay");
    if (fd == 0) fd = 1;
    if (!(fd > 0)) fd = 0;  // hostile values (Go rejects them at load): keep the cast defined
    else if (fd > 1e6) fd = 1e6;
    ev.setAnim(0, frames, (int)std::round(fd * 100));
  }
  return ev;
}

void Renderer::Impl::renderLayers(JsonArrayConst layers, Eval& ev, bool inLoop, int depth, bool top, int start) {
  if (depth > kMaxBlockDepth) throw RenderError("block nesting depth exceeds maximum 16");
  bool chainSatisfied = false, inChain = false;
  int i = -1;
  for (JsonVariantConst lv : layers) {
    i++;
    if (top) {
      if (i < start) continue;  // restored from the prefix
      if (i == snapAt) snapshotPrefix(ev);
    }
    if (++layerCount > kMaxLayers) throw RenderError("total layer count exceeds maximum 256");
    JsonObjectConst L = lv.as<JsonObjectConst>();
    const char* elif = str(L, "elif");
    const char* ifE = str(L, "if");
    JsonVariantConst ev0 = L["else"];
    bool isElse = ev0.is<bool>() && ev0.as<bool>();
    bool render = false;
    if (*elif || isElse) {
      if (!inChain) throw RenderError("layer " + std::to_string(i) + ": elif/else without preceding if");
      if (!chainSatisfied && (isElse || ev.cond(elif))) chainSatisfied = render = true;
    } else if (*ifE) {
      inChain = true;
      chainSatisfied = false;
      if (ev.cond(ifE)) chainSatisfied = render = true;
    } else {
      inChain = chainSatisfied = false;
      render = true;
    }
    if (!render) continue;

    std::string type = str(L, "type");
    if (type.empty()) {  // block node: same chain as its siblings
      try {
        renderLayers(L["layers"].as<JsonArrayConst>(), ev, inLoop, depth + 1);
      } catch (const RenderError& e) {
        throw RenderError("block layer " + std::to_string(i) + ": " + e.what());
      }
      continue;
    }
    try {
      if (type == "image") renderImage(L, ev);
      else if (type == "rect") renderRect(L, ev);
      else if (type == "text") renderText(L, ev);
      else if (type == "copy") renderCopy(L, ev);
      else if (type == "loop") {
        if (inLoop) throw RenderError("nested loops are not supported");
        renderLoop(L, ev, depth);
      } else if (type == "splitflap") {
        if (inLoop) throw RenderError("splitflap inside loop is not supported");
        renderSplitflap(L, ev);
      } else {
        throw RenderError("unknown type \"" + type + "\"");
      }
    } catch (const RenderError& e) {
      throw RenderError("layer " + std::to_string(i) + " (" + type + "): " + e.what());
    }
  }
}

void Renderer::Impl::renderImage(JsonObjectConst L, const Eval& ev) {
  const char* fixedId = L["_i"] | (const char*)nullptr;
  const char* origId = L["_io"] | (const char*)nullptr;
  const Img* img = fixedId ? pkg->image(fixedId) : origId ? pkg->image(origId) : nullptr;
  if (!img) throw RenderError("renderImage: image missing from package");

  // Dynamic size (_io): scale at run time like renderImage.
  bool scale = false;
  int w = img->w, h = img->h;
  if (!fixedId) {
    int rw = coord(L, "width", ev), rh = coord(L, "height", ev);
    if (rw > 0 || rh > 0) {
      if (rw > kMaxCanvasDimension || rh > kMaxCanvasDimension)
        throw RenderError("renderImage: scaled dimensions " + std::to_string(rw) + "x" + std::to_string(rh) +
                          " exceed maximum 16384");
      if (rw <= 0) {
        // <= 65535 * 16384, fits int.
        rw = (int)std::round((double)img->w * (double)rh / (double)img->h);
        if (rw < 1) rw = 1;
      }
      if (rh <= 0) {
        rh = (int)std::round((double)img->h * (double)rw / (double)img->w);
        if (rh < 1) rh = 1;
      }
      scale = true;
      w = rw;
      h = rh;
    }
  }

  long long x = coord(L, "x", ev), y = coord(L, "y", ev);

  bool rotate = false;
  RotGeom g = RotGeom();
  long long ox = 0, oy = 0;
  std::string rot(trimSpace(ev.interpolate(resolveCond(L["rotate"], ev))));
  if (!rot.empty()) {
    double deg;
    if (!parseFloat(rot, &deg)) throw RenderError("renderImage: invalid rotate value \"" + rot + "\"");
    if (!std::isfinite(deg))
      throw RenderError("renderImage: invalid rotate value \"" + rot + "\": rotate must be finite");
    deg = std::fmod(deg, 360);
    g = rotGeom(w, h, deg);
    rotate = true;
    ox = x - g.nw / 2;
    oy = y - g.nh / 2;
  }

  ImgView v = fullView(*img);
  ZzaBytes region{ZzaAlloc<uint8_t>(budget)};
  if (scale) {
    if ((long long)w * h <= kCacheMaxPx) {
      v = fullView(cachedScale(img, w, h));
    } else {
      // Only the part that can reach the canvas is scaled (Go's result there is the same).
      int rx, ry, rw, rh;
      if (rotate) {
        if (!rotSourceRect(canvas, w, h, g, ox, oy, &rx, &ry, &rw, &rh)) return;
      } else {
        long long x0 = std::max(0LL, -x), x1 = std::min((long long)w, canvas.w - x);
        long long y0 = std::max(0LL, -y), y1 = std::min((long long)h, canvas.h - y);
        if (x0 >= x1 || y0 >= y1) return;
        rx = (int)x0;
        ry = (int)y0;
        rw = (int)(x1 - x0);
        rh = (int)(y1 - y0);
      }
      region.resize((size_t)rw * rh * 4);
      scaleCatmullRom(budget, *img, w, h, rx, ry, rw, rh, region.data());
      v.px = region.data();
      v.stride = rw;
      v.ox = rx;
      v.oy = ry;
      v.rw = rw;
      v.rh = rh;
      v.w = w;
      v.h = h;
    }
  }
  if (rotate) drawRotated(canvas, v, g, ox, oy);
  else blitImage(canvas, v, x, y);
}

void Renderer::Impl::renderRect(JsonObjectConst L, const Eval& ev) {
  Rgba c;
  try {
    c = parseColor(ev.interpolate(resolveCond(L["color"], ev)));
  } catch (const RenderError& e) {
    throw RenderError(std::string("renderRect: ") + e.what());
  }
  long long x = coord(L, "x", ev), y = coord(L, "y", ev);
  long long w = coord(L, "width", ev), h = coord(L, "height", ev);
  // image.Rect canonicalises: a negative size spans the other way.
  long long x0 = std::min(x, x + w), x1 = std::max(x, x + w);
  long long y0 = std::min(y, y + h), y1 = std::max(y, y + h);
  fillRect(canvas, x0, y0, x1, y1, c);
}

void Renderer::Impl::renderText(JsonObjectConst L, Eval& ev) {
  std::string text = capText(ev.interpolate(resolveCond(L["value"], ev)));
  Rgba col;
  try {
    col = parseColor(ev.interpolate(resolveCond(L["color"], ev)));
  } catch (const RenderError& e) {
    throw RenderError(std::string("renderText: ") + e.what());
  }
  const char* fid = L["_f"] | (const char*)nullptr;
  const Font* f = fid ? pkg->font(fid) : nullptr;
  if (!f) throw RenderError(std::string("renderText: font ID \"") + str(L, "font") + "\" not found in template");

  TextParams p;
  p.x = coord(L, "x", ev);
  p.y = coord(L, "y", ev);
  p.width = coord(L, "width", ev);
  p.height = coord(L, "height", ev);
  p.maxWidth = coord(L, "max_width", ev);
  std::string align = str(L, "align"), valign = str(L, "valign");
  p.align = align == "center" ? kAlignCenter : align == "right" ? kAlignRight : kAlignLeft;
  p.valign = valign == "middle"     ? kValignMiddle
             : valign == "bottom"   ? kValignBottom
             : valign == "baseline" ? kValignBaseline
                                    : kValignTop;
  p.lineHeight = num(L, "line_height");
  p.scaleX = num(L, "scale_x");
  p.scaleY = num(L, "scale_y");
  // Go rejects these when loading the template.
  if (!validScale(p.scaleX)) throw RenderError("renderText: scale_x must be between 0.05 and 8");
  if (!validScale(p.scaleY)) throw RenderError("renderText: scale_y must be between 0.05 and 8");
  int advance = plainInt(L["advance"]);
  if (advance < 0 || advance > kMaxAdvance) throw RenderError("renderText: advance must be between 0 and 240");
  JsonVariantConst fi = L["fit_items"];
  p.fitItems = fi.is<JsonObjectConst>();
  p.fitSplitBy = p.fitItems ? str(fi.as<JsonObjectConst>(), "split_by") : "";
  p.fitGap = p.fitItems ? plainInt(fi["gap"]) : 0;

  int widest = drawTextLayer(budget, canvas, *f, p, text, col);

  std::string id = str(L, "id");
  if (!id.empty() && !p.fitItems) {  // setTextVars
    int width = widest < kMaxTextVar ? widest : kMaxTextVar;
    int x = p.x < -kMaxTextVar ? -kMaxTextVar : p.x > kMaxTextVar ? kMaxTextVar : p.x;
    int end = width > 0 ? x + width + advance : x;
    ev.setIntVar(id + ".width", width);
    ev.setIntVar(id + ".end", end);
  }
}

void Renderer::Impl::renderCopy(JsonObjectConst L, const Eval& ev) {
  int sw = plainInt(L["src_width"]), sh = plainInt(L["src_height"]);
  if (sw <= 0 || sh <= 0)
    throw RenderError("renderCopy: src_width and src_height must be positive (got " + std::to_string(sw) + "x" +
                      std::to_string(sh) + ")");
  copyRegion(canvas, plainInt(L["src_x"]), plainInt(L["src_y"]), sw, sh, coord(L, "x", ev), coord(L, "y", ev));
}

void Renderer::Impl::renderLoop(JsonObjectConst L, const Eval& ev, int depth) {
  JsonArrayConst children = L["layers"];
  if (children.size() == 0) return;
  std::string value = ev.interpolate(resolveCond(L["value"], ev));
  if (value.empty()) return;
  std::string sep = str(L, "split_by");
  if (sep.empty()) sep = "|";
  int maxItems = plainInt(L["max_items"]);
  if (maxItems <= 0) maxItems = kDefaultMaxLoopItems;
  else if (maxItems > kMaxLoopItemsHardCap) maxItems = kMaxLoopItemsHardCap;
  std::string var = str(L, "var");

  int idx = 0;
  size_t st = 0;
  for (bool last = false; !last;) {
    size_t h = value.find(sep, st);
    last = h == std::string::npos;
    std::string_view item = trimSpace(std::string_view(value).substr(st, (last ? value.size() : h) - st));
    st = last ? value.size() : h + sep.size();
    if (item.empty()) continue;
    if (idx >= maxItems) break;
    Eval child = ev;
    if (!var.empty()) child.setStrVar(var, item);
    child.setIntVar("i", idx);
    child.setIntVar("loop.index", idx);
    try {
      renderLayers(children, child, true, depth);
    } catch (const RenderError& e) {
      throw RenderError("renderLoop: item " + std::to_string(idx) + ": " + e.what());
    }
    idx++;
  }
}

Renderer::Renderer(ZzaBudget* budget, const Package* pkg) : d_(new Impl(budget, pkg)) {}
Renderer::~Renderer() {}

int Renderer::render(const char* data, size_t len, const zza_time& now, uint8_t* rgb888, std::string& err) {
  std::unique_ptr<BudgetJsonDoc> doc;
  JsonVariantConst root;
  int rc = d_->parseData(data, len, doc, root, err);
  if (rc != ZZA_OK) return rc;
  Eval ev = d_->rootEval(root, now);
  memset(rgb888, 0, rd::kCanvasBytes);
  d_->beginPass(rgb888, Impl::kPassStill);
  try {
    d_->renderLayers(d_->pkg->tmpl()["layers"].as<JsonArrayConst>(), ev, false, 0, true);
  } catch (const RenderError& e) {
    err = e.what();
    return ZZA_ERR_TEMPLATE;
  } catch (const std::bad_alloc&) {
    d_->clearCache();  // out of budget: give back the scaled images too
    throw;
  }
  return ZZA_OK;
}

}  // namespace zza
