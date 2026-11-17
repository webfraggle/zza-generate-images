#include "package.h"

#include <cstring>
#include <vector>

#include "sha256.h"

namespace zza {

void* JsonAlloc::allocate(size_t n) {
  if (n > SIZE_MAX - 16) throw std::bad_alloc();
  uint8_t* p = static_cast<uint8_t*>(b->alloc(n + 16));
  memcpy(p, &n, sizeof n);
  return p + 16;
}

void JsonAlloc::deallocate(void* ptr) {
  uint8_t* p = static_cast<uint8_t*>(ptr) - 16;
  size_t n;
  memcpy(&n, p, sizeof n);
  b->free(p, n + 16);
}

// Every value takes one slot and is preceded by '[', '{' or ','; strings
// (including keys) are copied into the pool.
size_t jsonCapacity(const uint8_t* d, size_t n) {
  size_t slots = 1, quotes = 0;
  bool inStr = false, esc = false;
  for (size_t i = 0; i < n; i++) {
    uint8_t c = d[i];
    if (inStr) {
      if (esc) esc = false;
      else if (c == '\\') esc = true;
      else if (c == '"') { inStr = false; quotes++; }
      continue;
    }
    if (c == '"') { inStr = true; quotes++; }
    else if (c == ',' || c == '[' || c == '{') slots++;
  }
  return slots * sizeof(detail::VariantSlot) + n + quotes / 2 + 64;
}

namespace {

// A format violation; msg already names the file.
struct LoadError {
  std::string msg;
};

[[noreturn]] void fail(const std::string& file, const std::string& what) {
  throw LoadError{file + ": " + what};
}

// JSON nesting allowed in package files. A valid template reaches about 70
// levels in t.json: every block node adds an object and its "layers" array
// (2 levels) up to maxBlockDepth 16, a splitflap's children start at depth 0
// again (another 16 blocks), plus root, loop and splitflap levels.
constexpr uint8_t kMaxJsonNesting = 80;

std::unique_ptr<BudgetJsonDoc> parseJson(ZzaBudget* b, const uint8_t* d, size_t n, const char* file) {
  std::unique_ptr<BudgetJsonDoc> doc(new BudgetJsonDoc(jsonCapacity(d, n), JsonAlloc(b)));
  DeserializationError e = deserializeJson(*doc, reinterpret_cast<const char*>(d), n,
                                           DeserializationOption::NestingLimit(kMaxJsonNesting));
  if (e == DeserializationError::NoMemory) throw std::bad_alloc();
  if (e) fail(file, std::string("invalid JSON: ") + e.c_str());
  if (!doc->is<JsonObject>()) fail(file, "JSON root is not an object");
  return doc;
}

uint16_t le16(const uint8_t* p) { return (uint16_t)(p[0] | p[1] << 8); }
uint32_t le32(const uint8_t* p) {
  return (uint32_t)p[0] | (uint32_t)p[1] << 8 | (uint32_t)p[2] << 16 | (uint32_t)p[3] << 24;
}

// Expands RLE (control c < 128: c+1 literal pixels; else the next pixel
// repeats c-126 times) to exactly npix pixels of bpp bytes. *used = input bytes consumed.
bool rleDecode(const uint8_t* in, size_t inLen, size_t bpp, size_t npix, uint8_t* out, size_t* used) {
  size_t p = 0, o = 0, total = npix * bpp;
  while (o < total) {
    if (p >= inLen) return false;
    size_t c = in[p++];
    if (c < 128) {
      size_t l = (c + 1) * bpp;
      if (l > inLen - p || l > total - o) return false;
      memcpy(out + o, in + p, l);
      p += l;
      o += l;
    } else {
      size_t cnt = c - 126;
      if (bpp > inLen - p || cnt * bpp > total - o) return false;
      for (size_t k = 0; k < cnt; k++) memcpy(out + o + k * bpp, in + p, bpp);
      o += cnt * bpp;
      p += bpp;
    }
  }
  *used = p;
  return true;
}

// The most pixels that len bytes of RLE (bpp bytes per pixel) can expand to:
// the best case is a run, (1 + bpp) bytes for 129 pixels.
uint64_t rleMaxPixels(uint64_t len, uint64_t bpp) { return (len / (1 + bpp)) * 129; }

void loadImage(ZzaBudget* b, const std::string& name, const uint8_t* d, size_t n, Img& img) {
  if (n < 5) fail(name, "truncated image header");
  uint32_t w = le16(d), h = le16(d + 2);
  uint8_t enc = d[4];
  if (w == 0 || h == 0) fail(name, "empty image");
  uint64_t npix = (uint64_t)w * h;
  const uint8_t* body = d + 5;
  size_t bl = n - 5;
  if (enc == 0) {
    if ((uint64_t)bl != npix * 4) fail(name, "raw pixel data does not match w*h*4");
    img.px.resize((size_t)(npix * 4));
    memcpy(img.px.data(), body, bl);
  } else if (enc == 1) {
    if (npix > rleMaxPixels(bl, 4)) fail(name, "RLE data too short for w*h pixels");
    img.px.resize((size_t)(npix * 4));
    size_t used = 0;
    if (!rleDecode(body, bl, 4, (size_t)npix, img.px.data(), &used)) fail(name, "invalid RLE stream");
    if (used != bl) fail(name, "trailing data after RLE stream");
  } else {
    fail(name, "unknown image encoding");
  }
  img.w = (uint16_t)w;
  img.h = (uint16_t)h;
  (void)b;
}

const size_t kFntHeader = 16, kFntEntry = 17, kFntKern = 6;

void loadFont(const std::string& name, const uint8_t* d, size_t n, Font& f) {
  if (n < kFntHeader || memcmp(d, "ZF1", 3) != 0) fail(name, "not a ZF1 font");
  uint8_t flags = d[3];
  if (flags & ~1u) fail(name, "unknown font flags");
  bool rle = flags & 1;
  f.ascent = (int16_t)le16(d + 4);
  f.descent = (int16_t)le16(d + 6);
  f.height = (int16_t)le16(d + 8);
  uint32_t ng = le16(d + 10), nk = le32(d + 12);
  if (ng == 0) fail(name, "no glyphs");
  uint64_t tabEnd = kFntHeader + (uint64_t)ng * kFntEntry + (uint64_t)nk * kFntKern;
  if (tabEnd > n) fail(name, "glyph or kerning table exceeds file");
  const uint8_t* ent = d + kFntHeader;
  const uint8_t* kp = ent + (size_t)ng * kFntEntry;
  const uint8_t* area = d + (size_t)tabEnd;
  size_t areaLen = n - (size_t)tabEnd;

  // Pass 1: validate the tables and size the decoded mask area.
  uint64_t total = 0;
  int prev = -1;
  bool hasQ = false;
  for (uint32_t i = 0; i < ng; i++) {
    const uint8_t* e = ent + (size_t)i * kFntEntry;
    if ((int)e[0] <= prev) fail(name, "glyph code points not strictly ascending");
    prev = e[0];
    if (e[0] == '?') hasQ = true;
    uint64_t px = (uint64_t)le16(e + 9) * le16(e + 11);
    uint64_t off = le32(e + 13);
    if (px == 0) {
      if (off > areaLen) fail(name, "glyph offset beyond mask data");
    } else if (!rle) {
      if (off > areaLen || px > areaLen - off) fail(name, "glyph mask exceeds mask data");
    } else {
      if (off >= areaLen) fail(name, "glyph offset beyond mask data");
      if (px > rleMaxPixels(areaLen - off, 1)) fail(name, "glyph RLE data too short for w*h");
    }
    total += px;
    if (total > UINT32_MAX) fail(name, "masks too large");
  }
  if (!hasQ) fail(name, "glyph '?' missing");
  uint32_t pk = 0;
  for (uint32_t i = 0; i < nk; i++) {
    uint32_t key = le16(kp + (size_t)i * kFntKern);
    if (i > 0 && key <= pk) fail(name, "kerning keys not strictly ascending");
    pk = key;
  }

  // Pass 2: build.
  f.glyphs.reserve(ng);
  f.kern.reserve(nk);
  f.masks.resize((size_t)total);
  uint32_t out = 0;
  for (uint32_t i = 0; i < ng; i++) {
    const uint8_t* e = ent + (size_t)i * kFntEntry;
    Glyph g;
    g.cp = e[0];
    g.adv = (int32_t)le32(e + 1);
    g.bx = (int16_t)le16(e + 5);
    g.by = (int16_t)le16(e + 7);
    g.w = le16(e + 9);
    g.h = le16(e + 11);
    uint32_t off = le32(e + 13);
    size_t px = (size_t)g.w * g.h;
    g.off = out;
    if (px > 0) {
      if (!rle) {
        memcpy(f.masks.data() + out, area + off, px);
      } else {
        size_t used = 0;
        if (!rleDecode(area + off, areaLen - off, 1, px, f.masks.data() + out, &used))
          fail(name, "invalid glyph RLE stream");
      }
      out += (uint32_t)px;
    }
    f.glyphs.push_back(g);
  }
  for (uint32_t i = 0; i < nk; i++) {
    const uint8_t* k = kp + (size_t)i * kFntKern;
    KernEnt ke;
    ke.key = le16(k);
    ke.kern = (int32_t)le32(k + 2);
    f.kern.push_back(ke);
  }
}

bool hexEq(const char* hex, const uint8_t digest[32]) {
  static const char* kHex = "0123456789abcdef";
  if (strlen(hex) != 64) return false;
  for (int i = 0; i < 32; i++) {
    char a = hex[2 * i], c = hex[2 * i + 1];
    if (a >= 'A' && a <= 'F') a = (char)(a - 'A' + 'a');
    if (c >= 'A' && c <= 'F') c = (char)(c - 'A' + 'a');
    if (a != kHex[digest[i] >> 4] || c != kHex[digest[i] & 15]) return false;
  }
  return true;
}

// "i<digits>.img" or "f<digits>.fnt": returns 'i' / 'f' (and the id) or 0.
char classify(const std::string& name, std::string& id) {
  if (name.size() < 6) return 0;
  char kind = name[0];
  const char* ext = kind == 'i' ? ".img" : kind == 'f' ? ".fnt" : nullptr;
  if (!ext || name.compare(name.size() - 4, 4, ext) != 0) return 0;
  for (size_t i = 1; i + 4 < name.size(); i++)
    if (name[i] < '0' || name[i] > '9') return 0;
  id = name.substr(0, name.size() - 4);
  return kind;
}

bool validName(const char* s) {
  size_t n = strlen(s);
  if (n == 0 || n > 12) return false;
  for (size_t i = 0; i < n; i++) {
    char c = s[i];
    bool ok = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' ||
              c == '_' || c == '-';
    if (!ok) return false;
  }
  return strcmp(s, ".") != 0 && strcmp(s, "..") != 0;
}

}  // namespace

struct Package::Impl {
  struct ImgEnt {
    std::string id;
    Img img;
    ImgEnt(std::string i, ZzaBudget* b) : id(std::move(i)), img(b) {}
  };
  struct FontEnt {
    std::string id;
    Font font;
    FontEnt(std::string i, ZzaBudget* b) : id(std::move(i)), font(b) {}
  };

  ZzaBudget* budget;
  std::vector<std::unique_ptr<ImgEnt>> imgs;
  std::vector<std::unique_ptr<FontEnt>> fonts;
  std::unique_ptr<BudgetJsonDoc> tj;
  RegexTable re;

  explicit Impl(ZzaBudget* b) : budget(b), re(b) {}
};

Package::Package(ZzaBudget* budget) : d_(new Impl(budget)) {}
Package::~Package() {}

int Package::load(const zza_file* files, int n, std::string& err) {
  ZzaBudget* b = d_->budget;
  try {
    if (n < 0 || (n > 0 && !files)) fail("package", "no files");
    std::vector<const zza_file*> fl;
    for (int i = 0; i < n; i++) {
      if (!files[i].name || (!files[i].data && files[i].len > 0)) fail("package", "invalid file entry");
      for (const zza_file* o : fl)
        if (strcmp(o->name, files[i].name) == 0) fail(files[i].name, "duplicate file");
      fl.push_back(&files[i]);
    }
    auto find = [&](const char* name) -> const zza_file* {
      for (const zza_file* f : fl)
        if (strcmp(f->name, name) == 0) return f;
      return nullptr;
    };

    const zza_file* mf = find("manifest.json");
    if (!mf) fail("manifest.json", "missing");
    std::unique_ptr<BudgetJsonDoc> man = parseJson(b, mf->data, mf->len, "manifest.json");
    JsonObjectConst m = man->as<JsonObjectConst>();
    JsonVariantConst fmt = m["format"];
    if (!fmt.is<long>() || fmt.as<long>() != 1) fail("manifest.json", "unsupported format (want 1)");
    JsonArrayConst canvas = m["canvas"];
    if (canvas.size() != 2 || !canvas[0].is<int>() || !canvas[1].is<int>() || canvas[0].as<int>() != 240 ||
        canvas[1].as<int>() != 240)
      fail("manifest.json", "canvas must be [240,240]");
    JsonArrayConst list = m["files"];
    if (list.isNull()) fail("manifest.json", "files missing");

    std::vector<std::string> seen;
    for (JsonObjectConst e : list) {
      const char* name = e["n"] | (const char*)nullptr;
      const char* hash = e["h"] | (const char*)nullptr;
      JsonVariantConst sv = e["s"];
      if (!name || !hash || !sv.is<unsigned long>()) fail("manifest.json", "malformed file entry");
      if (!validName(name) || strcmp(name, "manifest.json") == 0) fail("manifest.json", "invalid file name");
      for (const std::string& s : seen)
        if (s == name) fail("manifest.json", std::string("duplicate entry ") + name);
      seen.push_back(name);

      const zza_file* f = find(name);
      if (!f) fail(name, "listed in manifest but missing");
      if ((unsigned long)f->len != sv.as<unsigned long>()) fail(name, "size differs from manifest");
      ZzaSha256 sha;
      sha.update(f->data, f->len);
      uint8_t dig[32];
      sha.final(dig);
      if (!hexEq(hash, dig)) fail(name, "SHA-256 differs from manifest");

      std::string nm = name, id;
      char kind = classify(nm, id);
      if (strcmp(name, "t.json") == 0) {
        d_->tj = parseJson(b, f->data, f->len, name);
        JsonVariantConst re = d_->tj->as<JsonObjectConst>()["_re"];
        if (!re.isNull()) {
          if (!re.is<JsonObjectConst>() || !d_->re.load(re.as<JsonObjectConst>()))
            fail(name, "invalid _re table");
        }
      } else if (kind == 'i') {
        d_->imgs.emplace_back(new Impl::ImgEnt(id, b));
        loadImage(b, nm, f->data, f->len, d_->imgs.back()->img);
      } else if (kind == 'f') {
        d_->fonts.emplace_back(new Impl::FontEnt(id, b));
        loadFont(nm, f->data, f->len, d_->fonts.back()->font);
      }
    }
    if (!d_->tj) fail("t.json", "not listed in manifest");
    return ZZA_OK;
  } catch (const LoadError& e) {
    err = e.msg;
    return ZZA_ERR_FORMAT;
  }
}

const Img* Package::image(const char* id) const {
  for (const auto& e : d_->imgs)
    if (e->id == id) return &e->img;
  return nullptr;
}

const Font* Package::font(const char* id) const {
  for (const auto& e : d_->fonts)
    if (e->id == id) return &e->font;
  return nullptr;
}

const RegexSource* Package::regexSource() const { return &d_->re; }

const RegexProg* Package::regex(const std::string& pattern) const { return d_->re.find(pattern); }

JsonVariantConst Package::tmpl() const {
  if (!d_->tj) return JsonVariantConst();
  return d_->tj->as<JsonVariantConst>();
}

const Glyph* Font::find(unsigned cp) const {
  size_t lo = 0, hi = glyphs.size();
  while (lo < hi) {
    size_t mid = (lo + hi) / 2;
    if (glyphs[mid].cp < cp) lo = mid + 1;
    else hi = mid;
  }
  return lo < glyphs.size() && glyphs[lo].cp == cp ? &glyphs[lo] : nullptr;
}

int32_t Font::kernFor(unsigned left, unsigned right) const {
  unsigned key = (left & 0xFF) << 8 | (right & 0xFF);
  size_t lo = 0, hi = kern.size();
  while (lo < hi) {
    size_t mid = (lo + hi) / 2;
    if (kern[mid].key < key) lo = mid + 1;
    else hi = mid;
  }
  return lo < kern.size() && kern[lo].key == key ? kern[lo].kern : 0;
}

}  // namespace zza
