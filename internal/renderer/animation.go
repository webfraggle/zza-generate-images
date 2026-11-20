package renderer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"math"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Animation limits and defaults (template meta.animation, splitflap timing fields).
const (
	defaultAnimFPS      = 20
	minAnimFPS          = 5
	maxAnimFPS          = 25
	defaultAnimDuration = 2.5 // seconds
	minAnimDuration     = 0.5
	maxAnimDuration     = 5.0
	maxAnimFrames       = 125 // frames per animation, including the final still

	// Frame mode (meta.animation.frames).
	minAnimFrames     = 2
	defaultFrameDelay = 1.0 // seconds
	minFrameDelay     = 0.02
	maxFrameDelay     = 10.0
	maxAnimSeconds    = 120.0 // frames × frame_delay

	// maxAnimPixels bounds frames × canvas width × canvas height of one
	// animation (≈ 128 MB of NRGBA), checked before any frame is rendered.
	maxAnimPixels = 32_000_000

	defaultFlipFrames = 2 // frames per flip
	maxFlipFrames     = 6
	defaultMinFlips   = 4
)

// Look of a flip in progress (spec §6). The falling flap is darkened by these
// fractions; its squash per phase is given by flipPhase.
const (
	flapShadeUpper = 0.25 // upper half of the old value, falling towards the hinge
	flapShadeLower = 0.15 // lower half of the new value, unfolding below the hinge
)

// Animation is a rendered frame sequence.
type Animation struct {
	Frames []*image.NRGBA
	FPS    int  // splitflap mode: every frame lasts round(100/FPS) cs, the last frame gifFinalDelay
	Delay  int  // frame mode: every frame lasts Delay cs (1–1000); when > 0, FPS is ignored and the last frame is not held
	Loop   bool // GIF repeats forever (NETSCAPE loop count 0); false = plays once
}

// resolve returns the effective animation settings, applying defaults for
// zero values, or an error if a value is out of range.
func (a AnimationMeta) resolve() (fps int, duration float64, err error) {
	fps, duration = a.FPS, a.Duration
	if fps == 0 {
		fps = defaultAnimFPS
	} else if fps < minAnimFPS || fps > maxAnimFPS {
		return 0, 0, fmt.Errorf("animation.fps must be between %d and %d (got %d)", minAnimFPS, maxAnimFPS, fps)
	}
	if duration == 0 {
		duration = defaultAnimDuration
	} else if !(duration >= minAnimDuration && duration <= maxAnimDuration) {
		return 0, 0, fmt.Errorf("animation.duration must be between %g and %g seconds (got %g)", minAnimDuration, maxAnimDuration, duration)
	}
	return fps, duration, nil
}

// frameDelayCS returns the frame-mode delay per frame in hundredths of a
// second (0 = default of 1 s).
func (a AnimationMeta) frameDelayCS() int {
	if a.FrameDelay == 0 {
		return int(math.Round(defaultFrameDelay * 100))
	}
	return int(math.Round(a.FrameDelay * 100))
}

// validateAnimation checks meta.animation: ranges and the rules of frame mode,
// splitflap mode and non-animated templates.
func (t *Template) validateAnimation() error {
	a := t.Meta.Animation
	if _, _, err := a.resolve(); err != nil {
		return err
	}
	if a.Frames == 0 {
		if a.FrameDelay != 0 {
			return fmt.Errorf("animation.frame_delay requires animation.frames")
		}
		return nil
	}
	if a.Frames < minAnimFrames || a.Frames > maxAnimFrames {
		return fmt.Errorf("animation.frames must be between %d and %d (got %d)", minAnimFrames, maxAnimFrames, a.Frames)
	}
	if a.FPS != 0 || a.Duration != 0 {
		return fmt.Errorf("animation.frames cannot be combined with animation.fps or animation.duration")
	}
	if t.HasSplitflap() {
		return fmt.Errorf("animation.frames cannot be used in a template with splitflap layers")
	}
	d := a.FrameDelay
	if d == 0 {
		d = defaultFrameDelay
	}
	if !(d >= minFrameDelay && d <= maxFrameDelay) {
		return fmt.Errorf("animation.frame_delay must be between %g and %g seconds in steps of 0.01 (got %g)", minFrameDelay, maxFrameDelay, a.FrameDelay)
	}
	if math.Abs(d*100-math.Round(d*100)) >= 1e-9 {
		return fmt.Errorf("animation.frame_delay must be between %g and %g seconds in steps of 0.01 (got %g)", minFrameDelay, maxFrameDelay, a.FrameDelay)
	}
	if total := float64(a.Frames) * d; total > maxAnimSeconds {
		return fmt.Errorf("animation.frames × animation.frame_delay must not exceed %g seconds (got %g)", maxAnimSeconds, total)
	}
	return nil
}

// AnimationSeed derives the seed of an animation from the template name and
// the request body: sha256(templateName ‖ 0x00 ‖ body).
func AnimationSeed(templateName string, body []byte) []byte {
	h := sha256.New()
	h.Write([]byte(templateName))
	h.Write([]byte{0})
	h.Write(body)
	return h.Sum(nil)
}

// RenderAnimation renders the animation of a template. In frame mode
// (meta.animation.frames) it renders N frames with anim.frame = 0 … N−1 and
// a fixed Delay; frame 0 equals Render's output. Otherwise it renders the
// split-flap animation: every
// module starts flipping in frame 0 and comes to rest on its target value
// after a seeded number of flips. The last frame equals Render's output.
// A template without splitflap layers yields exactly one frame.
// Equal inputs give identical frames; {{now.*}} advances per frame in frame
// mode and is frozen at the start in splitflap mode.
func (r *Renderer) RenderAnimation(tmpl *Template, data map[string]interface{}, seed []byte) (*Animation, error) {
	fps, duration, err := tmpl.Meta.Animation.resolve()
	if err != nil {
		return nil, fmt.Errorf("renderer: RenderAnimation: %w", err)
	}
	if err := checkCanvas(tmpl.Meta.Canvas); err != nil {
		return nil, fmt.Errorf("renderer: RenderAnimation: %w", err)
	}

	if tmpl.Meta.Animation.Frames > 0 {
		return r.renderFrameMode(tmpl, data)
	}

	// Reject oversized animations up front, using the upper bound on frames.
	if tmpl.HasSplitflap() {
		maxFrames := min(int(math.Floor(duration*float64(fps)+1e-9)), maxAnimFrames-1) + 1
		if px := int64(maxFrames) * int64(tmpl.Meta.Canvas.Width) * int64(tmpl.Meta.Canvas.Height); px > maxAnimPixels {
			return nil, fmt.Errorf("renderer: RenderAnimation: animation too large: up to %d frames of %dx%d exceed %d pixels",
				maxFrames, tmpl.Meta.Canvas.Width, tmpl.Meta.Canvas.Height, maxAnimPixels)
		}
	}

	eval := r.newEvaluator(tmpl, data) // one evaluator, one `now`, for all frames
	imgCache := make(map[string]image.Image)
	newCtx := func(anim *animState) *renderCtx {
		rc := newRenderCtx(tmpl, eval) // fresh maxLayers counter per frame
		rc.imgCache = imgCache
		rc.anim = anim
		return rc
	}

	// The static pass applies exactly Render's checks and yields the final still.
	final, err := r.renderFrame(newCtx(nil))
	if err != nil {
		return nil, fmt.Errorf("renderer: RenderAnimation: %w", err)
	}
	if !tmpl.HasSplitflap() {
		return &Animation{Frames: []*image.NRGBA{final}, FPS: fps, Loop: false}, nil
	}

	budget := int(math.Floor(duration*float64(fps) + 1e-9))
	anim := &animState{
		seed:      seed,
		budget:    min(budget, maxAnimFrames-1),
		schedules: make(map[moduleKey]*flapSchedule),
		leaves:    make(map[leafKey]*image.NRGBA),
	}
	var frames []*image.NRGBA
	// Frame 0 schedules every module and so fixes anim.end.
	for anim.frame = 0; anim.frame == 0 || anim.frame < anim.end; anim.frame++ {
		img, err := r.renderFrame(newCtx(anim))
		if err != nil {
			return nil, fmt.Errorf("renderer: RenderAnimation: frame %d: %w", anim.frame, err)
		}
		if anim.end == 0 { // no module was drawn (splitflap only in untaken branches)
			break
		}
		frames = append(frames, img)
	}
	frames = append(frames, final)
	return &Animation{Frames: frames, FPS: fps, Loop: tmpl.Meta.Animation.Loop}, nil
}

// renderFrameMode renders the frames of a frame-mode template. {{now.*}}
// runs with the frames: the start is the renderer clock in the template's
// time zone, truncated to the second; frame f shows
// start + f × frame_delay (f·delay centiseconds). Every frame runs the whole
// layer list.
func (r *Renderer) renderFrameMode(tmpl *Template, data map[string]interface{}) (*Animation, error) {
	a := tmpl.Meta.Animation
	c := tmpl.Meta.Canvas
	if px := int64(a.Frames) * int64(c.Width) * int64(c.Height); px > maxAnimPixels {
		return nil, fmt.Errorf("renderer: RenderAnimation: animation too large: %d frames of %dx%d exceed %d pixels",
			a.Frames, c.Width, c.Height, maxAnimPixels)
	}
	delay := tmpl.animDelayCS()
	start := r.clock().In(tmpl.location()).Truncate(time.Second) // clock read once
	root := newEvaluatorAt(data, start)
	imgCache := make(map[string]image.Image)
	frames := make([]*image.NRGBA, 0, a.Frames)
	for f := 0; f < a.Frames; f++ {
		now := start.Add(time.Duration(f*delay) * 10 * time.Millisecond)
		rc := newRenderCtx(tmpl, root.withNow(now).withAnim(f, a.Frames, delay)) // fresh maxLayers counter
		rc.imgCache = imgCache
		img, err := r.renderFrame(rc)
		if err != nil {
			return nil, fmt.Errorf("renderer: RenderAnimation: frame %d: %w", f, err)
		}
		frames = append(frames, img)
	}
	return &Animation{Frames: frames, Delay: delay, Loop: a.Loop}, nil
}

// animState is the animation state shared by the frames of one
// RenderAnimation call; nil in renderCtx means static image.
type animState struct {
	frame     int    // index of the frame being rendered
	seed      []byte // seeds the flip counts
	budget    int    // frames available for flipping (duration·fps, capped)
	end       int    // first frame in which all modules are at rest (max k·flipFrames)
	schedules map[moduleKey]*flapSchedule
	leaves    map[leafKey]*image.NRGBA // rendered flaps; each rendered at most once
}

type moduleKey struct{ layerIdx, cellIdx int }

type leafKey struct {
	layerIdx, cellIdx int
	value             string
}

// flapSchedule is the motion of one module: it flips from seq[i] to seq[i+1]
// during frames i·flipFrames … (i+1)·flipFrames−1 and rests on the last entry
// (the target) from frame (len(seq)−1)·flipFrames on.
type flapSchedule struct {
	flipFrames int
	seq        []string
}

// schedule returns the schedule of m, creating it in frame 0.
func (a *animState) schedule(m flapModule) (*flapSchedule, error) {
	key := moduleKey{m.layerIdx, m.cellIdx}
	if s, ok := a.schedules[key]; ok {
		return s, nil
	}
	if a.frame != 0 {
		return nil, fmt.Errorf("module first appeared in frame %d, not in frame 0", a.frame)
	}

	f := m.layer.FlipFrames
	if f == 0 {
		f = defaultFlipFrames
	}
	maxK := max(1, a.budget/f)
	minFlips, maxFlips := m.layer.MinFlips, m.layer.MaxFlips
	if minFlips == 0 {
		minFlips = defaultMinFlips
	}
	if maxFlips == 0 {
		maxFlips = maxK
	}
	minFlips = min(max(minFlips, 1), maxK)
	maxFlips = min(max(maxFlips, 1), maxK)
	minFlips = min(minFlips, maxFlips)

	k := flipCount(a.seed, m.layerIdx, m.cellIdx, minFlips, maxFlips)
	s := &flapSchedule{flipFrames: f, seq: flipSequence(m.layer.Reel, m.target, k)}
	a.schedules[key] = s
	a.end = max(a.end, k*f)
	return s, nil
}

// drawAnimatedModule draws module m in its state in the current frame.
func (r *Renderer) drawAnimatedModule(rc *renderCtx, dst *image.NRGBA, m flapModule) error {
	s, err := rc.anim.schedule(m)
	if err != nil {
		return err
	}
	fr := rc.anim.frame
	k := len(s.seq) - 1
	if fr >= k*s.flipFrames {
		leaf, err := r.cachedLeaf(rc, dst, m, m.target)
		if err != nil {
			return err
		}
		draw.Draw(dst, m.rect, leaf, image.Point{}, draw.Src)
		return nil
	}
	step, phase := fr/s.flipFrames, fr%s.flipFrames
	from, err := r.cachedLeaf(rc, dst, m, s.seq[step])
	if err != nil {
		return err
	}
	to, err := r.cachedLeaf(rc, dst, m, s.seq[step+1])
	if err != nil {
		return err
	}
	composeFlip(dst, m.rect.Min, from, to, m.splitY, phase, s.flipFrames)
	return nil
}

// cachedLeaf returns the flap of m showing value, rendering it on first use.
// The background is taken from dst under the module; modules do not overlap
// and the canvas under a module is the same in every frame, so the leaf is
// valid for all frames.
func (r *Renderer) cachedLeaf(rc *renderCtx, dst *image.NRGBA, m flapModule, value string) (*image.NRGBA, error) {
	key := leafKey{m.layerIdx, m.cellIdx, value}
	if leaf, ok := rc.anim.leaves[key]; ok {
		return leaf, nil
	}
	leaf, err := r.renderLeaf(rc, m, snapshot(dst, m.rect), value)
	if err != nil {
		return nil, err
	}
	rc.anim.leaves[key] = leaf
	return leaf, nil
}

// flipCount returns the seeded number of flips of a module, in
// [minFlips, maxFlips]. It requires minFlips <= maxFlips.
func flipCount(seed []byte, layerIdx, cellIdx, minFlips, maxFlips int) int {
	if maxFlips <= minFlips {
		return minFlips
	}
	var idx [16]byte
	binary.BigEndian.PutUint64(idx[:8], uint64(layerIdx))
	binary.BigEndian.PutUint64(idx[8:], uint64(cellIdx))
	h := sha256.New()
	h.Write(seed)
	h.Write(idx[:])
	sum := h.Sum(nil)
	return minFlips + int(binary.BigEndian.Uint64(sum[:8])%uint64(maxFlips-minFlips+1))
}

// flipSequence returns the k+1 values a module shows during k flips, ending
// with target. If target is in the reel (first occurrence), the run walks the
// reel cyclically and arrives there; otherwise k reel entries in cyclic order
// precede target, starting at an offset derived from k (which is seeded).
// Without a reel the module flips through empty flaps ("").
func flipSequence(reel []string, target string, k int) []string {
	k = max(k, 0)
	seq := make([]string, k+1)
	seq[k] = target
	n := len(reel)
	if n == 0 {
		return seq
	}
	start := k
	for i, v := range reel {
		if v == target {
			start = i - k
			break
		}
	}
	for i := 0; i < k; i++ {
		seq[i] = reel[((start+i)%n+n)%n]
	}
	return seq
}

// flipPhase maps a frame of a flip lasting phases frames to the half that is
// falling and its progress p in (0, 1). The first ceil(phases/2) frames show
// the upper half falling, the rest the lower half; progress is spaced evenly
// within each half (p = 0.5 for a single frame).
func flipPhase(phase, phases int) (upper bool, p float64) {
	nUpper := (phases + 1) / 2
	if phase < nUpper {
		return true, float64(phase+1) / float64(nUpper+1)
	}
	nLower := phases - nUpper
	return false, float64(phase-nUpper+1) / float64(nLower+1)
}

// composeFlip draws, at position at on dst, a module flipping from leaf from
// to leaf to (same size, origin (0,0)) in frame phase ∈ [0, phases) of the flip.
// splitY is the hinge line relative to the module.
//
// Upper half falling: the upper half shows to, overlaid by the upper half of
// from, squashed to height upper·(1−p) and anchored at the hinge, darkened;
// the lower half still shows from. Lower half falling: the upper half shows
// to; below the hinge, the lower half of to, squashed to height lower·p,
// lies over the lower half of from, slightly darkened.
func composeFlip(dst *image.NRGBA, at image.Point, from, to *image.NRGBA, splitY int, phase, phases int) {
	w, h := to.Bounds().Dx(), to.Bounds().Dy()
	upperR := image.Rect(0, 0, w, splitY)
	lowerR := image.Rect(0, splitY, w, h)
	draw.Draw(dst, upperR.Add(at), to, upperR.Min, draw.Src)
	draw.Draw(dst, lowerR.Add(at), from, lowerR.Min, draw.Src)

	upper, p := flipPhase(phase, phases)
	if upper {
		hh := squashedHeight(splitY, 1-p)
		r := image.Rect(0, splitY-hh, w, splitY).Add(at)
		xdraw.ApproxBiLinear.Scale(dst, r, from, upperR, xdraw.Src, nil)
		shade(dst, r, flapShadeUpper)
		return
	}
	hh := squashedHeight(h-splitY, p)
	r := image.Rect(0, splitY, w, splitY+hh).Add(at)
	xdraw.ApproxBiLinear.Scale(dst, r, to, lowerR, xdraw.Src, nil)
	shade(dst, r, flapShadeLower)
}

// squashedHeight returns full·frac rounded, at least 1 and at most full.
func squashedHeight(full int, frac float64) int {
	return min(max(int(math.Round(float64(full)*frac)), 1), full)
}

// shade darkens the colour channels of img inside r by the fraction amount.
// Integer arithmetic keeps the result identical on every platform.
func shade(img *image.NRGBA, r image.Rectangle, amount float64) {
	r = r.Intersect(img.Bounds())
	keep := uint32(math.Round((1 - amount) * 256))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := img.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x, i = x+1, i+4 {
			for c := 0; c < 3; c++ {
				img.Pix[i+c] = uint8((uint32(img.Pix[i+c])*keep + 128) >> 8)
			}
		}
	}
}
