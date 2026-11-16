#include "regex.h"

#include <cstring>
#include <new>

#include "utf8.h"

// Port of Go's regexp NFA matcher (regexp/exec.go: machine.match/step/add) for
// FindStringSubmatch. Threads live in two sparse-set queues ordered by
// priority; each queue slot owns its capture array, so no per-thread
// allocation happens. add() uses an explicit stack instead of recursion.

namespace zza {

namespace {

// regexp/syntax.InstOp
enum : uint8_t {
  kAlt = 0,
  kAltMatch = 1,
  kCapture = 2,
  kEmptyWidth = 3,
  kMatch = 4,
  kFail = 5,
  kNop = 6,
  kRune = 7,
  kRune1 = 8,
  kRuneAny = 9,
  kRuneAnyNotNL = 10,
};

// regexp/syntax.EmptyOp
enum : uint32_t {
  kEmptyBeginLine = 1,
  kEmptyEndLine = 2,
  kEmptyBeginText = 4,
  kEmptyEndText = 8,
  kEmptyWordBoundary = 16,
  kEmptyNoWordBoundary = 32,
};

constexpr int32_t kEndOfText = -1;
constexpr uint32_t kMaxRune = 0x10FFFF;
// Structural limits for a loaded program (far above what a 100-rune pattern
// compiles to; the budget is the real cap).
constexpr uint32_t kMaxInsts = 1u << 20;
constexpr uint32_t kMaxCap = 1u << 16;

bool isWordChar(int32_t r) {
  return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_';
}

// syntax.EmptyOpContext: the empty-width assertions satisfied between r1 and r2.
uint32_t emptyOpContext(int32_t r1, int32_t r2) {
  uint32_t op = kEmptyNoWordBoundary;
  int boundary = 0;
  if (isWordChar(r1)) boundary = 1;
  else if (r1 == '\n') op |= kEmptyBeginLine;
  else if (r1 < 0) op |= kEmptyBeginText | kEmptyBeginLine;
  if (isWordChar(r2)) boundary ^= 1;
  else if (r2 == '\n') op |= kEmptyEndLine;
  else if (r2 < 0) op |= kEmptyEndText | kEmptyEndLine;
  if (boundary) op ^= (kEmptyWordBoundary | kEmptyNoWordBoundary);
  return op;
}

// inputString.step: rune at pos and its width (kEndOfText, 0 at the end).
int32_t stepInput(std::string_view s, size_t pos, int* width) {
  if (pos >= s.size()) {
    *width = 0;
    return kEndOfText;
  }
  uint32_t r;
  *width = decodeRune(s.data() + pos, s.size() - pos, &r);
  return static_cast<int32_t>(r);
}

bool readU32(JsonVariantConst v, uint32_t* out) {
  if (!v.is<uint32_t>()) return false;
  *out = v.as<uint32_t>();
  return true;
}

}  // namespace

// ---- RegexProg ----

RegexProg::RegexProg(ZzaBudget* budget)
    : budget_(budget),
      start_(0),
      ncap_(0),
      inst_(ZzaAlloc<Inst>(budget)),
      runes_(ZzaAlloc<int32_t>(budget)) {}

void RegexProg::clear() {
  start_ = 0;
  ncap_ = 0;
  ZzaVec<Inst>(ZzaAlloc<Inst>(budget_)).swap(inst_);
  ZzaVec<int32_t>(ZzaAlloc<int32_t>(budget_)).swap(runes_);
}

bool RegexProg::load(JsonArrayConst prog) {
  clear();
  if (prog.isNull() || prog.size() != 3) return false;
  JsonArrayConst::iterator it = prog.begin();
  uint32_t start, ncap;
  if (!readU32(*it, &start)) return false;
  ++it;
  if (!readU32(*it, &ncap) || ncap < 2 || ncap % 2 != 0 || ncap > kMaxCap) return false;
  ++it;
  JsonArrayConst insts = (*it).as<JsonArrayConst>();
  if (insts.isNull()) return false;
  start_ = start;
  ncap_ = ncap;
  bool ok = false;
  try {
    ok = loadInsts(insts);
  } catch (...) {
    clear();
    throw;
  }
  if (!ok || start_ >= inst_.size()) {
    clear();
    return false;
  }
  return true;
}

bool RegexProg::loadInsts(JsonArrayConst insts) {
  // First pass: shape and sizes, so both arrays are allocated once.
  size_t n = 0, nr = 0;
  for (JsonVariantConst v : insts) {
    JsonArrayConst a = v.as<JsonArrayConst>();
    if (a.isNull() || a.size() != 4) return false;
    JsonArrayConst r = a[3].as<JsonArrayConst>();
    if (r.isNull()) return false;
    n++;
    nr += r.size();
    if (n > kMaxInsts || nr > kMaxInsts * 2) return false;
  }
  if (n == 0) return false;
  inst_.reserve(n);
  runes_.reserve(nr);

  for (JsonVariantConst v : insts) {
    JsonArrayConst a = v.as<JsonArrayConst>();
    JsonArrayConst::iterator it = a.begin();
    uint32_t op, out, arg;
    if (!readU32(*it, &op) || op > kRuneAnyNotNL) return false;
    ++it;
    if (!readU32(*it, &out) || out >= n) return false;
    ++it;
    if (!readU32(*it, &arg)) return false;
    ++it;
    if ((op == kAlt || op == kAltMatch) && arg >= n) return false;
    Inst in;
    in.op = static_cast<uint8_t>(op);
    in.out = out;
    in.arg = arg;
    in.runeOff = static_cast<uint32_t>(runes_.size());
    uint32_t prevHi = 0;
    size_t count = 0;
    for (JsonVariantConst rv : (*it).as<JsonArrayConst>()) {
      uint32_t r;
      if (!readU32(rv, &r) || r > kMaxRune) return false;
      if (count % 2 == 1 && r < static_cast<uint32_t>(runes_.back())) return false;  // lo > hi
      if (count % 2 == 0 && count > 0 && r <= prevHi) return false;               // unsorted/overlap
      if (count % 2 == 1) prevHi = r;
      runes_.push_back(static_cast<int32_t>(r));
      count++;
    }
    in.runeLen = static_cast<uint32_t>(count);
    if (op == kRune && count % 2 != 0) return false;
    if (op == kRune1 && count != 1) return false;
    if (op != kRune && op != kRune1 && count != 0) return false;
    inst_.push_back(in);
  }
  return true;
}

bool RegexProg::matchRune(const Inst& in, int32_t r) const {
  if (in.op == kRune1) return r == runes_[in.runeOff];
  // Binary search over the sorted [lo,hi] pairs.
  const int32_t* p = runes_.data() + in.runeOff;
  size_t lo = 0, hi = in.runeLen / 2;
  while (lo < hi) {
    size_t m = lo + (hi - lo) / 2;
    if (r < p[2 * m]) hi = m;
    else if (r > p[2 * m + 1]) lo = m + 1;
    else return true;
  }
  return false;
}

// ---- Pike VM ----

class RegexVM {
 public:
  RegexVM(const RegexProg& prog, int32_t* mem);
  // Size of the scratch memory in int32_t units (throws bad_alloc on overflow).
  static size_t memWords(const RegexProg& prog);
  bool match(std::string_view s);
  const int32_t* matchcap() const { return matchcap_; }

 private:
  struct Queue {
    uint32_t* sparse;  // pc -> dense index
    uint32_t* dense;   // dense index -> pc
    int32_t* caps;     // dense index * ncap
    uint32_t size;
    bool contains(uint32_t pc) const { return sparse[pc] < size && dense[sparse[pc]] == pc; }
  };
  struct StackEntry {
    uint32_t pc;
    int32_t capIdx;  // >= 0: restore cap[capIdx] = old
    int32_t old;
  };

  void add(Queue& q, uint32_t pc, int32_t pos, int32_t* cap, uint32_t ctx);
  void step(Queue& runq, Queue& nextq, int32_t pos, int32_t nextPos, int32_t c, uint32_t nextCtx);

  const RegexProg& p_;
  uint32_t n_, ncap_;
  Queue q0_, q1_;
  StackEntry* stack_;
  int32_t* matchcap_;
  bool matched_;
};

size_t RegexVM::memWords(const RegexProg& prog) {
  // Per queue: sparse + dense + caps; stack (3 words per entry, <= 2n+1
  // entries: each instruction pushes at most two); matchcap.
  uint64_t n = prog.inst_.size(), ncap = prog.ncap_;
  uint64_t words = 2 * (2 * n + n * ncap) + 3 * (2 * n + 1) + ncap;
  if (words > SIZE_MAX / sizeof(int32_t)) throw std::bad_alloc();
  return static_cast<size_t>(words);
}

RegexVM::RegexVM(const RegexProg& prog, int32_t* mem)
    : p_(prog), n_(static_cast<uint32_t>(prog.inst_.size())), ncap_(prog.ncap_), matched_(false) {
  static_assert(sizeof(StackEntry) == 3 * sizeof(int32_t), "stack entry layout");
  Queue* qs[2] = {&q0_, &q1_};
  for (Queue* q : qs) {
    q->sparse = reinterpret_cast<uint32_t*>(mem);
    mem += n_;
    q->dense = reinterpret_cast<uint32_t*>(mem);
    mem += n_;
    q->caps = mem;
    mem += static_cast<size_t>(n_) * ncap_;
    q->size = 0;
    // sparse may hold anything (membership is cross-checked with dense), but
    // keep it deterministic.
    memset(q->sparse, 0, sizeof(uint32_t) * n_);
  }
  stack_ = reinterpret_cast<StackEntry*>(mem);
  mem += 3 * (2 * static_cast<size_t>(n_) + 1);
  matchcap_ = mem;
}

// machine.add: follows empty transitions from pc in priority order and records
// every thread-bearing instruction (Match, Rune*) with a copy of cap.
void RegexVM::add(Queue& q, uint32_t pc0, int32_t pos, int32_t* cap, uint32_t ctx) {
  size_t sp = 0;
  stack_[sp++] = StackEntry{pc0, -1, 0};
  while (sp > 0) {
    StackEntry e = stack_[--sp];
    if (e.capIdx >= 0) {
      cap[e.capIdx] = e.old;
      continue;
    }
    uint32_t pc = e.pc;
    if (pc == 0 || q.contains(pc)) continue;  // pc 0 is always fail (as in Go)
    uint32_t j = q.size++;
    q.dense[j] = pc;
    q.sparse[pc] = j;
    const RegexProg::Inst& in = p_.inst_[pc];
    switch (in.op) {
      case kFail:
        break;
      case kAlt:
      case kAltMatch:
        stack_[sp++] = StackEntry{in.arg, -1, 0};  // explored after out
        stack_[sp++] = StackEntry{in.out, -1, 0};
        break;
      case kEmptyWidth:
        if ((in.arg & ~ctx) == 0) stack_[sp++] = StackEntry{in.out, -1, 0};
        break;
      case kNop:
        stack_[sp++] = StackEntry{in.out, -1, 0};
        break;
      case kCapture:
        if (in.arg < ncap_) {
          stack_[sp++] = StackEntry{0, static_cast<int32_t>(in.arg), cap[in.arg]};
          cap[in.arg] = pos;
        }
        stack_[sp++] = StackEntry{in.out, -1, 0};
        break;
      default:  // kMatch, kRune, kRune1, kRuneAny, kRuneAnyNotNL
        memcpy(q.caps + static_cast<size_t>(j) * ncap_, cap, sizeof(int32_t) * ncap_);
        break;
    }
  }
}

// machine.step (first-match mode): advances every thread over rune c.
void RegexVM::step(Queue& runq, Queue& nextq, int32_t pos, int32_t nextPos, int32_t c, uint32_t nextCtx) {
  for (uint32_t j = 0; j < runq.size; j++) {
    const RegexProg::Inst& in = p_.inst_[runq.dense[j]];
    int32_t* cap = runq.caps + static_cast<size_t>(j) * ncap_;
    bool adv = false;
    switch (in.op) {
      case kMatch:
        cap[1] = pos;
        memcpy(matchcap_, cap, sizeof(int32_t) * ncap_);
        matched_ = true;
        runq.size = 0;  // cut off all lower-priority threads
        return;
      case kRune:
      case kRune1:
        adv = p_.matchRune(in, c);
        break;
      case kRuneAny:
        adv = true;
        break;
      case kRuneAnyNotNL:
        adv = c != '\n';
        break;
      default:  // no thread in this slot
        continue;
    }
    // runq's slot is not read again, so its captures serve as add's scratch.
    if (adv) add(nextq, in.out, nextPos, cap, nextCtx);
  }
  runq.size = 0;
}

// machine.match from position 0.
bool RegexVM::match(std::string_view s) {
  matched_ = false;
  for (uint32_t i = 0; i < ncap_; i++) matchcap_[i] = -1;
  Queue* runq = &q0_;
  Queue* nextq = &q1_;
  int32_t pos = 0;
  int width = 0, width1 = 0;
  int32_t r = stepInput(s, 0, &width);
  int32_t r1 = kEndOfText;
  if (r != kEndOfText) r1 = stepInput(s, static_cast<size_t>(width), &width1);
  uint32_t ctx = emptyOpContext(-1, r);
  for (;;) {
    if (runq->size == 0 && matched_) break;
    if (!matched_) {
      matchcap_[0] = pos;
      add(*runq, p_.start_, pos, matchcap_, ctx);
    }
    ctx = emptyOpContext(r, r1);
    step(*runq, *nextq, pos, pos + width, r, ctx);
    if (width == 0) break;
    pos += width;
    r = r1;
    width = width1;
    if (r != kEndOfText) r1 = stepInput(s, static_cast<size_t>(pos + width), &width1);
    Queue* t = runq;
    runq = nextq;
    nextq = t;
  }
  return matched_;
}

std::string matchGroup(const RegexProg& prog, std::string_view utf8Input, int group) {
  if (group < 0 || utf8Input.size() > kMaxFilterValueLen || prog.empty()) return std::string();
  ZzaVec<int32_t> mem{ZzaAlloc<int32_t>(prog.budget_)};
  mem.resize(RegexVM::memWords(prog));
  RegexVM vm(prog, mem.data());
  if (!vm.match(utf8Input)) return std::string();
  size_t g = static_cast<size_t>(group);
  if (2 * g + 1 >= prog.ncap_) return std::string();
  const int32_t* mc = vm.matchcap();
  int32_t a = mc[2 * g], b = mc[2 * g + 1];
  if (a < 0 || b < a) return std::string();
  return std::string(utf8Input.substr(static_cast<size_t>(a), static_cast<size_t>(b - a)));
}

// ---- RegexTable ----

RegexTable::RegexTable(ZzaBudget* budget)
    : budget_(budget),
      names_(ZzaAlloc<char>(budget)),
      nameOff_(ZzaAlloc<uint32_t>(budget)),
      progs_(ZzaAlloc<RegexProg>(budget)) {}

bool RegexTable::load(JsonObjectConst table) {
  names_.clear();
  nameOff_.clear();
  progs_.clear();
  if (table.isNull()) return false;
  size_t n = 0, chars = 0;
  for (JsonPairConst kv : table) {
    n++;
    chars += kv.key().size();
  }
  names_.reserve(chars);
  nameOff_.reserve(n + 1);
  progs_.reserve(n);
  nameOff_.push_back(0);
  for (JsonPairConst kv : table) {
    JsonString k = kv.key();  // sized: patterns may contain NUL
    progs_.emplace_back(budget_);
    if (!progs_.back().load(kv.value().as<JsonArrayConst>())) {
      names_.clear();
      nameOff_.clear();
      progs_.clear();
      return false;
    }
    names_.insert(names_.end(), k.c_str(), k.c_str() + k.size());
    nameOff_.push_back(static_cast<uint32_t>(names_.size()));
  }
  return true;
}

const RegexProg* RegexTable::find(std::string_view pattern) const {
  for (size_t i = 0; i < progs_.size(); i++) {
    std::string_view name(names_.data() + nameOff_[i], nameOff_[i + 1] - nameOff_[i]);
    if (name == pattern) return &progs_[i];
  }
  return nullptr;
}

}  // namespace zza
