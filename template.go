package glyph

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unsafe"

	"github.com/kungfusheep/riffkey"
)

// Component is the extension interface for custom components.
// External packages can implement this to create custom components
// that expand to built-in primitives at compile time.
type Component interface {
	Build() Component
}

// Renderer is the extension interface for components that render directly.
// Unlike Component (which expands to primitives), Renderer draws to the
// buffer itself. This is useful for custom widgets like charts, sparklines, etc.
type Renderer interface {
	Component

	// MinSize returns the minimum dimensions needed by this component.
	// Called during layout phase.
	MinSize() (width, height int)

	// Render draws the component to the buffer at the given position.
	// w and h are the allocated dimensions (may be larger than MinSize).
	Render(buf *Buffer, x, y, w, h int)
}

// forEachCompiler is implemented by generic ForEach types to compile themselves
type forEachCompiler interface {
	compileTo(t *Template, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16
}

// listCompiler is implemented by generic List types to compile themselves
type listCompiler interface {
	toSelectionList() *selectionList
}

// bindable is implemented by components that declare key bindings as data.
type bindable interface {
	bindings() []binding
}

// routeBindable is implemented by zero-size routing nodes that declare
// handlers directly in the view tree.
type routeBindable interface {
	routeBindings() []binding
}

// textInputBindable is implemented by InputC for text input routing.
type textInputBindable interface {
	textBinding() *textInputBinding
}

// templateTree is implemented by compound components that compose existing
// building blocks into a template subtree.
type templateTree interface {
	toTemplate() Component
}

type valueBranchNode interface {
	getMatchIndex() int
	getCaseNodes() []any
	getDefaultNode() any
}

// LayoutFunc positions children given their sizes and available space.
type LayoutFunc func(children []ChildSize, availW, availH int) []Rect

// ChildSize represents a child's computed minimum dimensions.
type ChildSize struct {
	MinW, MinH int
}

// Rect represents a positioned rectangle.
type Rect struct {
	X, Y, W, H int
	// Opacity is the effective rendered opacity for this node after inherited
	// container/overlay opacity has been applied. Effects that target a NodeRef
	// can use it to fade with the node they are attached to.
	Opacity    float64
	opacitySet bool
}

// NodeRef holds a node's rendered screen bounds, populated each frame after layout.
// Declare one, attach it to a node with .NodeRef(), then read it in effects or
// anywhere that needs to know where something actually rendered.
type NodeRef = Rect

// Box is a container with a custom layout function.
// Use this when HBox/VBox don't fit your needs.
type Box struct {
	Layout   LayoutFunc
	Children []Component
}

// Template is a compiled UI template.
// Compile does all reflection. Execute is pure pointer arithmetic.
type Template struct {
	ops  []Op
	geom []Geom // parallel to ops, filled at runtime

	// For bottom-up layout traversal
	maxDepth int
	byDepth  [][]int16 // ops grouped by tree depth

	// Current element base for ForEach context (set during layout/render)
	elemBase unsafe.Pointer
	// Compile/runtime element contexts for nested ForEach captures. elemBase is
	// kept for the current item fast path; elemBases keeps outer item bases
	// addressable when an inner template captures an outer item field.
	compileElemContexts []elemCompileContext
	elemBases           []unsafe.Pointer

	// App reference for jump mode coordination
	app *App

	jumpOffsetX int
	jumpOffsetY int
	jumpMinY    int
	jumpMaxY    int

	// per-item index for ForEach/selectionList (reset per iteration, used by per-item tweens)
	itemIndex int

	// row styling for selectionList selected rows (merged with cell styles)
	rowBG   Color
	rowFG   Color
	rowAttr Attribute

	// Style inheritance - current inherited style during render
	inheritedStyle *Style
	inheritedFill  Color // cascades through nested containers
	refOpacity     float64
	refOpacitySet  bool

	// vertical clip: maximum Y coordinate for rendering (exclusive, 0 = no clip)
	clipMaxY int16

	// exit is used by conditional branch selectors to retain a branch until its
	// Animate.Out tweens have completed.
	exit exitScope

	// compile-time: tracks the outermost property pointer and collects
	// nested tween items maps so the outermost condition can record
	// per-item displayed values for transition detection
	compilePropertyPtr    *Color
	compileTweenItemsMaps []map[unsafe.Pointer]*perItemColorState

	// Pending overlays to render after main content (cleared each frame)
	pendingOverlays []pendingOverlay

	// Pending screen effects collected from tree (cleared each frame)
	pendingScreenEffects []Effect

	// scratch buffers for per-frame reuse (avoid nil-slice allocs in hot paths)
	flexScratchIdx  []int16   // flex child indices (shared by VBox + HBox phases)
	flexScratchGrow []float32 // flex grow values (shared by VBox + HBox phases)
	flexScratchImpl []int16   // implicit flex children (HBox only)
	treeScratchPfx  []bool    // tree node line prefix

	// ext pools — contiguous allocations for cache-friendly render access.

	// Declarative bindings collected during compile, wired during setup
	pendingBindings           []binding
	pendingRouteBindings      []binding
	pendingModalRouteBindings []binding
	pendingTIB                *textInputBinding
	pendingLogs               []*LogC       // Logs that need app.RequestRender wiring
	pendingFocusManager       *FocusManager // Focus manager for multi-input routing
	routeRouter               *riffkey.Router
	routeAttached             bool
	routeModalRouter          *riffkey.Router
	routeModalPushed          bool
	routeFMActive             bool // FM pushed for this visibility cycle; a user blur stays blurred until the branch toggles

	// per-frame evaluators — conditions, animations, etc. run at start of Execute
	evals []func()

	// exit evaluators — non-item Animate.Out evaluators owned by this template.
	// These can be primed when a retained branch first renders as exiting, so
	// NodeRefs see the correct opacity before effects sample them.
	exitEvals []func()

	// per-item evaluators — run once per ForEach item with elemBase set
	itemEvals []func()

	// frame timing — single timestamp per frame, shared by all animations
	frameTime time.Time
	animating bool
	oscEpoch  time.Time        // first-frame stamp; oscillators derive from frameTime-oscEpoch
	nowFn     func() time.Time // test injection point; nil means time.Now

	// completions defers tween OnComplete callbacks to the end of Execute:
	// they fire on the render thread after the frame's reads finish, so a
	// callback may safely mutate bound state — including the slice a ForEach
	// is iterating — for the NEXT frame
	completions []func()

	// animation ticker — runs at ~60fps only while animations are active
	animTicker    *time.Ticker
	requestRender func()

	// root points to the outermost template so sub-templates (If branches,
	// Overlays, ForEach) register evaluators where Execute actually runs them.
	root *Template
}

type elemCompileContext struct {
	base unsafe.Pointer
	size uintptr
}

// deferComplete queues a tween completion callback to run after this frame's
// Execute finishes. See the completions field for the safety contract.
func (t *Template) deferComplete(fn func()) {
	root := t.evalRoot()
	root.completions = append(root.completions, fn)
}

// evalRoot returns the root template where evaluators should be registered.
// for top-level templates root is nil and we return self.
func (t *Template) evalRoot() *Template {
	if t.root != nil {
		return t.root
	}
	return t
}

type exitScope struct {
	tweenCount     int
	activeLeases   int
	rendering      bool
	renderingItems map[unsafe.Pointer]bool
	parent         *Template
}

func (t *Template) registerExitTween(tw tweenNode) {
	if tw != nil && tw.getTweenOut() != nil {
		t.exit.tweenCount++
		if t.exit.parent != nil {
			t.exit.parent.registerExitTween(tw)
		}
	}
}

func (t *Template) hasExitTweens() bool {
	return t != nil && t.exit.tweenCount > 0
}

func (t *Template) hasActiveExitLeases() bool {
	return t != nil && t.exit.activeLeases > 0
}

func (t *Template) setExitLease(slot *bool, active bool) {
	if *slot == active {
		return
	}
	*slot = active
	if active {
		t.exit.activeLeases++
		if t.exit.parent != nil {
			t.exit.parent.setExitLease(new(bool), true)
		}
		return
	}
	if t.exit.activeLeases > 0 {
		t.exit.activeLeases--
	}
	if t.exit.parent != nil && t.exit.parent.exit.activeLeases > 0 {
		t.exit.parent.exit.activeLeases--
	}
}

func (t *Template) setExitRendering(active bool) {
	if t == nil {
		return
	}
	t.exit.rendering = active
	for i := range t.ops {
		switch ext := t.ops[i].Ext.(type) {
		case *opOverlay:
			if ext.childTmpl != nil {
				ext.childTmpl.setExitRendering(active)
			}
		case *opIf:
			ext.thenTmpl.setExitRendering(active)
			ext.elseTmpl.setExitRendering(active)
		case *opSwitch:
			for _, tmpl := range ext.cases {
				tmpl.setExitRendering(active)
			}
			ext.def.setExitRendering(active)
		case *opMatch:
			for _, tmpl := range ext.cases {
				tmpl.setExitRendering(active)
			}
			ext.def.setExitRendering(active)
		}
	}
}

func (t *Template) setExitRenderingFor(elemBase unsafe.Pointer, active bool) {
	if t == nil {
		return
	}
	if elemBase == nil {
		t.setExitRendering(active)
		return
	}
	if t.exit.renderingItems == nil {
		t.exit.renderingItems = make(map[unsafe.Pointer]bool)
	}
	if active {
		t.exit.renderingItems[elemBase] = true
	} else {
		delete(t.exit.renderingItems, elemBase)
	}
	for i := range t.ops {
		switch ext := t.ops[i].Ext.(type) {
		case *opOverlay:
			if ext.childTmpl != nil {
				ext.childTmpl.setExitRenderingFor(elemBase, active)
			}
		case *opIf:
			ext.thenTmpl.setExitRenderingFor(elemBase, active)
			ext.elseTmpl.setExitRenderingFor(elemBase, active)
		case *opSwitch:
			for _, tmpl := range ext.cases {
				tmpl.setExitRenderingFor(elemBase, active)
			}
			ext.def.setExitRenderingFor(elemBase, active)
		case *opMatch:
			for _, tmpl := range ext.cases {
				tmpl.setExitRenderingFor(elemBase, active)
			}
			ext.def.setExitRenderingFor(elemBase, active)
		}
	}
}

func (t *Template) isExitRenderingFor(elemBase unsafe.Pointer) bool {
	if t == nil {
		return false
	}
	if elemBase != nil && t.exit.renderingItems != nil && t.exit.renderingItems[elemBase] {
		return true
	}
	return t.exit.rendering
}

func (t *Template) runItemEvals(elemBase unsafe.Pointer) {
	t.runItemEvalsFrom(nil, elemBase)
}

func (t *Template) runItemEvalsFrom(parent *Template, elemBase unsafe.Pointer) {
	t.bindItemContext(parent, elemBase)
	for _, eval := range t.itemEvals {
		eval()
	}
}

func (t *Template) bindItemContext(parent *Template, elemBase unsafe.Pointer) {
	t.elemBase = elemBase
	contexts := len(t.compileElemContexts)
	if contexts == 0 {
		t.elemBases = nil
		return
	}
	if cap(t.elemBases) < contexts {
		t.elemBases = make([]unsafe.Pointer, contexts)
	}
	t.elemBases = t.elemBases[:contexts]
	clear(t.elemBases)
	parentContexts := 0
	if parent != nil {
		parentContexts = len(parent.compileElemContexts)
		copy(t.elemBases, parent.elemBases)
	}
	if elemBase != nil && contexts > parentContexts {
		t.elemBases[contexts-1] = elemBase
	} else if elemBase != nil && contexts > 0 && t.elemBases[contexts-1] == nil {
		t.elemBases[contexts-1] = elemBase
	}
}

func (t *Template) runtimeElemBase(idx int) unsafe.Pointer {
	if idx >= 0 && idx < len(t.elemBases) {
		return t.elemBases[idx]
	}
	return t.elemBase
}

func (t *Template) runExitEvals() {
	if t == nil {
		return
	}
	for _, eval := range t.exitEvals {
		eval()
	}
	for i := range t.ops {
		switch ext := t.ops[i].Ext.(type) {
		case *opOverlay:
			ext.childTmpl.runExitEvals()
		case *opIf:
			ext.thenTmpl.runExitEvals()
			ext.elseTmpl.runExitEvals()
		case *opSwitch:
			for _, tmpl := range ext.cases {
				tmpl.runExitEvals()
			}
			ext.def.runExitEvals()
		case *opMatch:
			for _, tmpl := range ext.cases {
				tmpl.runExitEvals()
			}
			ext.def.runExitEvals()
		}
	}
}

type branchSelector struct {
	selected     int
	initialized  bool
	exiting      bool
	exitRendered bool
}

func (b *branchSelector) selectBranch(requested int, branches []*Template) (int, bool) {
	if !b.initialized {
		b.initialized = true
		b.selected = -1
	}
	if b.selected == requested {
		b.exiting = false
		b.exitRendered = false
		if current := branchAt(branches, requested); current != nil {
			current.setExitRendering(false)
		}
		return requested, false
	}

	current := branchAt(branches, b.selected)
	if current == nil || !current.hasExitTweens() {
		if current != nil {
			current.setExitRendering(false)
		}
		b.selected = requested
		b.exiting = false
		b.exitRendered = false
		return requested, false
	}

	if b.exiting && b.exitRendered && !current.hasActiveExitLeases() {
		current.setExitRendering(false)
		b.selected = requested
		b.exiting = false
		b.exitRendered = false
		return requested, false
	}

	b.exiting = true
	if !b.exitRendered {
		current.evalRoot().animating = true
	}
	return b.selected, true
}

func (b *branchSelector) markExitRendered() {
	if b.exiting {
		b.exitRendered = true
	}
}

func branchAt(branches []*Template, idx int) *Template {
	if idx < 0 || idx >= len(branches) {
		return nil
	}
	return branches[idx]
}

func routeChildTemplates(t *Template) []*Template {
	if t == nil {
		return nil
	}
	var children []*Template
	for i := range t.ops {
		switch ext := t.ops[i].Ext.(type) {
		case *opIf:
			if ext.thenTmpl != nil {
				children = append(children, ext.thenTmpl)
			}
			if ext.elseTmpl != nil {
				children = append(children, ext.elseTmpl)
			}
		case *opSwitch:
			children = append(children, ext.cases...)
			if ext.def != nil {
				children = append(children, ext.def)
			}
		case *opMatch:
			children = append(children, ext.cases...)
			if ext.def != nil {
				children = append(children, ext.def)
			}
		case *opOverlay:
			if ext.childTmpl != nil {
				children = append(children, ext.childTmpl)
			}
		}
	}
	return children
}

func setRouteBranchActive(branches []*Template, activeIdx int) {
	for idx, tmpl := range branches {
		if tmpl == nil {
			continue
		}
		if idx != activeIdx {
			tmpl.setRouteActive(false)
		}
	}
	if activeIdx < 0 || activeIdx >= len(branches) || branches[activeIdx] == nil {
		return
	}
	branches[activeIdx].setRouteActive(true)
}

func (t *Template) setRouteActive(active bool) {
	if t == nil {
		return
	}
	if !active {
		for _, child := range routeChildTemplates(t) {
			child.setRouteActive(false)
		}
	}
	if t.routeRouter != nil {
		if active {
			t.routeRouter.Enable()
		} else {
			t.routeRouter.Disable()
		}
	}
	// the focus manager rides the same visibility edges as the modal router.
	// Stack discipline: on show the modal router pushes first, the focused
	// field's sub-router above it (the input gets first crack); on hide they
	// pop in reverse — FM first, modal second — including the retained
	// exit/fade path, so nothing orphans. The FM pushes on the SHOW EDGE
	// only (routeFMActive latch): a user blur (Escape) stays blurred, so the
	// next Escape reaches the modal router and can dismiss the overlay.
	fm := t.pendingFocusManager
	if !active && fm != nil && t.routeFMActive {
		if fm.pushed {
			if fm.pop != nil {
				fm.pop()
			}
			fm.pushed = false
		}
		t.routeFMActive = false
	}
	if t.routeModalRouter != nil {
		app := t.evalRoot().app
		if active && !t.routeModalPushed {
			t.routeModalRouter.Enable()
			if app != nil && app.input != nil {
				app.input.Push(t.routeModalRouter)
				t.routeModalPushed = true
			}
		} else if !active && t.routeModalPushed {
			if app != nil && app.input != nil {
				app.input.Pop()
			}
			t.routeModalRouter.Disable()
			t.routeModalPushed = false
		}
	}
	if active && fm != nil && !t.routeFMActive {
		fm.initialPush()
		t.routeFMActive = true
	}
}

func ifBranches(ifExt *opIf, elemBase unsafe.Pointer) ([]*Template, int) {
	branches := []*Template{ifExt.thenTmpl, ifExt.elseTmpl}
	if ifExt.eval(elemBase) {
		if ifExt.thenTmpl == nil {
			return branches, -1
		}
		return branches, 0
	}
	if ifExt.elseTmpl == nil {
		return branches, -1
	}
	return branches, 1
}

func switchBranches(swExt *opSwitch, elemBase unsafe.Pointer) ([]*Template, int) {
	branches := make([]*Template, 0, len(swExt.cases)+1)
	branches = append(branches, swExt.cases...)
	defIdx := -1
	if swExt.def != nil {
		defIdx = len(branches)
		branches = append(branches, swExt.def)
	}
	matchIdx := swExt.node.getMatchIndexWithBase(elemBase)
	if matchIdx >= 0 && matchIdx < len(swExt.cases) && swExt.cases[matchIdx] != nil {
		return branches, matchIdx
	}
	return branches, defIdx
}

func matchBranches(mExt *opMatch, elemBase unsafe.Pointer) ([]*Template, int) {
	branches := make([]*Template, 0, len(mExt.cases)+1)
	branches = append(branches, mExt.cases...)
	defIdx := -1
	if mExt.def != nil {
		defIdx = len(branches)
		branches = append(branches, mExt.def)
	}
	matchIdx := mExt.node.getMatchIndexWithBase(elemBase)
	if matchIdx >= 0 && matchIdx < len(mExt.cases) && mExt.cases[matchIdx] != nil {
		return branches, matchIdx
	}
	return branches, defIdx
}

// pendingOverlay stores info needed to render an overlay after main content
type pendingOverlay struct {
	op      *Op  // pointer to the overlay op
	exiting bool // the overlay's branch is animating out (condition went false) — its modal
	// router must be released, not kept alive, so it can't orphan on the input stack.
}

// SetApp links this template to an App for jump mode support.
func (t *Template) SetApp(a *App) {
	t.app = a
	// wire the animation render-scheduler so this template's tweens can drive their
	// own frames. Without it, t.animating starts no ticker (template.go: the ticker
	// only spins when requestRender != nil) and animations freeze. SetView set this
	// explicitly; named views (View/UpdateView) and scrollview children only get it
	// here — so wiring it in SetApp keeps every template's animations alive.
	if a != nil {
		t.requestRender = a.RequestRender
	}
}

func (t *Template) setJumpViewport(offsetX, offsetY, minY, maxY int) {
	t.jumpOffsetX = offsetX
	t.jumpOffsetY = offsetY
	t.jumpMinY = minY
	t.jumpMaxY = maxY
}

func (t *Template) collectBindings(node any) {
	if b, ok := node.(bindable); ok {
		t.pendingBindings = append(t.pendingBindings, b.bindings()...)
	}
}

func (t *Template) collectRouteBindings(node any) {
	switch b := node.(type) {
	case OnC:
		if b.modal {
			t.pendingModalRouteBindings = append(t.pendingModalRouteBindings, b.routeBindings()...)
		} else {
			t.pendingRouteBindings = append(t.pendingRouteBindings, b.routeBindings()...)
		}
	case routeBindable:
		t.pendingRouteBindings = append(t.pendingRouteBindings, b.routeBindings()...)
	}
}

func (t *Template) collectTextInputBinding(node any) {
	if tib, ok := node.(textInputBindable); ok {
		t.pendingTIB = tib.textBinding()
	}
}

func (t *Template) collectFocusManager(node any) {
	// check if InputC or FilterLogC has a manager
	switch v := node.(type) {
	case *InputC:
		if v.manager != nil && t.pendingFocusManager == nil {
			t.pendingFocusManager = v.manager
		}
	case *FilterLogC:
		if v.manager != nil && t.pendingFocusManager == nil {
			t.pendingFocusManager = v.manager
		}
	}
}

// Geom holds runtime geometry for an op.
// Filled during execute, parallel array to ops.
type Geom struct {
	W, H           int16 // dimensions
	LocalX, LocalY int16 // position relative to parent
	ContentH       int16 // natural content height (before flex distribution)
}

// Op represents a single compiled template instruction.
// The template compiler produces a flat array of these; Execute walks them to render.
type Op struct {
	Kind   OpKind
	Depth  int8  // tree depth (root children = 0)
	Parent int16 // parent op index, -1 for root children

	// Layout hints
	Width        int16   // explicit width
	Height       int16   // explicit height
	PercentWidth float32 // 0.0-1.0
	FlexGrow     float32 // share of remaining space
	Gap          int8    // gap between children
	ContentSized bool    // has fixed-width children (don't implicit flex)
	FitContent   bool    // size to content instead of filling available space
	MaxWidth     int16   // >0: size to content but never exceed this (wrapping content wraps at it)
	MaxWidthPct  float32 // >0: like MaxWidth but as a fraction of available width

	// Container
	IsRow        bool        // true=HBox, false=VBox
	Border       BorderStyle // border style
	BorderFG     *Color      // border foreground color
	BorderBG     *Color      // border background color
	Title        string      // border title
	ChildStart   int16       // first child op index
	ChildEnd     int16       // last child op index (exclusive)
	CascadeStyle *Style      // style inherited by children (pointer for dynamic themes)
	LocalStyle   *Style      // style for this container only (not inherited)
	Fill         Color       // container fill color (fills entire area)
	Margin       [4]int16    // outer margin: top, right, bottom, left
	Padding      [4]int16    // inner padding: top, right, bottom, left
	NodeRef      *NodeRef    // if set, populated with rendered screen bounds each frame
	OpacityMode  OpacityMode // rune handoff strategy when OpDyn.Opacity is set

	// kind-specific data — type-assert based on Kind.
	// we use a Kind switch + type assertion instead of interface dispatch because
	// concrete method calls after assertion are inlinable. interface calls are not,
	// and cause parameters to escape to heap. verified via go build -gcflags='-m -m'.
	Ext any

	// dynamic layout property overrides — nil for static ops
	Dyn *OpDyn
}

// OpDyn holds pointer overrides for shared layout properties.
// only allocated for ops that use dynamic values (e.g. Height(&h)).
type OpDyn struct {
	Height       *int16
	Width        *int16
	FlexGrow     *float32
	PercentWidth *float32
	Gap          *int8
	Fill         *Color
	FillOff      uintptr
	FillIsOff    bool
	Opacity      *float64
	OpacityOff   uintptr
	OpacityIsOff bool
	OpacityArmed *bool // set true by render to signal From tween activation
}

// resolver methods — inlinable nil-check + deref, zero cost when Dyn is nil

func (op *Op) height() int16 {
	if op.Dyn != nil {
		if p := op.Dyn.Height; p != nil {
			return *p
		}
	}
	return op.Height
}

func (op *Op) width() int16 {
	if op.Dyn != nil {
		if p := op.Dyn.Width; p != nil {
			return *p
		}
	}
	return op.Width
}

// hasDynWidth reports whether a dynamic width binding is present; a present
// binding evaluating to 0 means "explicitly sized, currently zero", not unset.
func (op *Op) hasDynWidth() bool {
	return op.Dyn != nil && op.Dyn.Width != nil
}

func (op *Op) flexGrow() float32 {
	if op.Dyn != nil {
		if p := op.Dyn.FlexGrow; p != nil {
			return *p
		}
	}
	return op.FlexGrow
}

func (op *Op) percentWidth() float32 {
	if op.Dyn != nil {
		if p := op.Dyn.PercentWidth; p != nil {
			return *p
		}
	}
	return op.PercentWidth
}

func (op *Op) gap() int8 {
	if op.Dyn != nil {
		if p := op.Dyn.Gap; p != nil {
			return *p
		}
	}
	return op.Gap
}

func (op *Op) fill() Color {
	return op.fillFor(nil)
}

// fillFor resolves the container fill, rebasing item-field pointers onto the
// element currently being rendered (a raw pointer into the ForEach template
// item would read the compile-time placeholder forever).
func (op *Op) fillFor(elemBase unsafe.Pointer) Color {
	if op.Dyn != nil {
		if op.Dyn.FillIsOff {
			if elemBase != nil {
				return *(*Color)(unsafe.Pointer(uintptr(elemBase) + op.Dyn.FillOff))
			}
			return op.Fill
		}
		if p := op.Dyn.Fill; p != nil {
			return *p
		}
	}
	return op.Fill
}

// compileCond registers a conditional evaluator and returns a pointer to its storage.
// the evaluator runs each frame, resolving the condition and writing the active value.

func (t *Template) compileCondInt16(cond conditionNode) *int16 {
	root := t.evalRoot()
	storage := new(int16)
	thenVal := cond.getThen()
	elseVal := cond.getElse()
	eval := func() {
		if cond.evaluate() {
			*storage = anyToInt16(thenVal)
		} else {
			*storage = anyToInt16(elseVal)
		}
	}
	eval() // set initial value
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileCondFloat32(cond conditionNode) *float32 {
	root := t.evalRoot()
	storage := new(float32)
	thenVal := cond.getThen()
	elseVal := cond.getElse()
	eval := func() {
		if cond.evaluate() {
			*storage = anyToFloat32(thenVal)
		} else {
			*storage = anyToFloat32(elseVal)
		}
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileCondFloat64(cond conditionNode) *float64 {
	root := t.evalRoot()
	storage := new(float64)
	thenVal := cond.getThen()
	elseVal := cond.getElse()
	eval := func() {
		if cond.evaluate() {
			*storage = anyToFloat64(thenVal)
		} else {
			*storage = anyToFloat64(elseVal)
		}
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileCondInt8(cond conditionNode) *int8 {
	root := t.evalRoot()
	storage := new(int8)
	thenVal := cond.getThen()
	elseVal := cond.getElse()
	eval := func() {
		if cond.evaluate() {
			*storage = anyToInt8(thenVal)
		} else {
			*storage = anyToInt8(elseVal)
		}
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileBranchInt16(branch valueBranchNode) *int16 {
	root := t.evalRoot()
	storage := new(int16)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	eval := func() {
		idx := branch.getMatchIndex()
		if idx >= 0 && idx < len(cases) {
			*storage = anyToInt16(cases[idx])
			return
		}
		*storage = anyToInt16(def)
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileBranchFloat32(branch valueBranchNode) *float32 {
	root := t.evalRoot()
	storage := new(float32)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	eval := func() {
		idx := branch.getMatchIndex()
		if idx >= 0 && idx < len(cases) {
			*storage = anyToFloat32(cases[idx])
			return
		}
		*storage = anyToFloat32(def)
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileBranchFloat64(branch valueBranchNode) *float64 {
	root := t.evalRoot()
	storage := new(float64)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	eval := func() {
		idx := branch.getMatchIndex()
		if idx >= 0 && idx < len(cases) {
			*storage = anyToFloat64(cases[idx])
			return
		}
		*storage = anyToFloat64(def)
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileBranchInt8(branch valueBranchNode) *int8 {
	root := t.evalRoot()
	storage := new(int8)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	eval := func() {
		idx := branch.getMatchIndex()
		if idx >= 0 && idx < len(cases) {
			*storage = anyToInt8(cases[idx])
			return
		}
		*storage = anyToInt8(def)
	}
	eval()
	root.evals = append(root.evals, eval)
	return storage
}

func anyToInt16(v any) int16 {
	switch val := v.(type) {
	case int16:
		return val
	case int:
		return int16(val)
	case *int16:
		return *val
	}
	return 0
}

func anyToFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case *float64:
		return *val
	}
	return 0
}

func anyToFloat32(v any) float32 {
	switch val := v.(type) {
	case float32:
		return val
	case float64:
		return float32(val)
	case int:
		return float32(val)
	case *float32:
		return *val
	}
	return 0
}

func anyToInt8(v any) int8 {
	switch val := v.(type) {
	case int8:
		return val
	case int:
		return int8(val)
	case *int8:
		return *val
	}
	return 0
}

// compileDyn resolves a dynamic property value (conditionNode or tweenNode) to a pointer.
// used by compile sites where Cond fields can hold either type.

func (t *Template) compileDynInt16(v any, elemBase unsafe.Pointer, elemSize uintptr) *int16 {
	switch c := v.(type) {
	case *int16:
		// a raw pointer into a ForEach element rebinds per item: read from the
		// element being rendered each frame, not the compile-time placeholder.
		if contextIdx, ptrOffset, inForEach := t.elemContextForPtr(uintptr(unsafe.Pointer(c)), elemBase, elemSize); inForEach {
			storage := new(int16)
			*storage = *c
			t.itemEvals = append(t.itemEvals, func() {
				if base := t.runtimeElemBase(contextIdx); base != nil {
					*storage = *(*int16)(unsafe.Pointer(uintptr(base) + ptrOffset))
				}
			})
			return storage
		}
		return c
	case conditionNode:
		return t.compileCondInt16(c)
	case valueBranchNode:
		return t.compileBranchInt16(c)
	case tweenNode:
		return t.compileTweenInt16(c, elemBase, elemSize)
	case OscC:
		return t.compileOscInt16(c)
	}
	return nil
}

func (t *Template) compileDynFloat32(v any, elemBase unsafe.Pointer, elemSize uintptr) *float32 {
	switch c := v.(type) {
	case *float32:
		// a raw pointer into a ForEach element rebinds per item: read from the
		// element being rendered each frame, not the compile-time placeholder.
		if contextIdx, ptrOffset, inForEach := t.elemContextForPtr(uintptr(unsafe.Pointer(c)), elemBase, elemSize); inForEach {
			storage := new(float32)
			*storage = *c
			t.itemEvals = append(t.itemEvals, func() {
				if base := t.runtimeElemBase(contextIdx); base != nil {
					*storage = *(*float32)(unsafe.Pointer(uintptr(base) + ptrOffset))
				}
			})
			return storage
		}
		return c
	case conditionNode:
		return t.compileCondFloat32(c)
	case valueBranchNode:
		return t.compileBranchFloat32(c)
	case tweenNode:
		return t.compileTweenFloat32(c, elemBase, elemSize)
	case OscC:
		return t.compileOscFloat32(c)
	}
	return nil
}

func (t *Template) compileDynFloat64(v any, elemBase unsafe.Pointer, elemSize uintptr) *float64 {
	switch c := v.(type) {
	case *float64:
		return c
	case conditionNode:
		return t.compileCondFloat64(c)
	case valueBranchNode:
		return t.compileBranchFloat64(c)
	case tweenNode:
		return t.compileTweenFloat64(c, nil, elemBase, elemSize)
	case OscC:
		return t.compileOscFloat64(c)
	}
	return nil
}

func (t *Template) compileDynInt8(v any, elemBase unsafe.Pointer, elemSize uintptr) *int8 {
	switch c := v.(type) {
	case conditionNode:
		return t.compileCondInt8(c)
	case valueBranchNode:
		return t.compileBranchInt8(c)
	case tweenNode:
		return t.compileTweenInt8(c, elemBase, elemSize)
	}
	return nil
}

func (t *Template) compileDynColor(v any, elemBase unsafe.Pointer, elemSize uintptr) *Color {
	switch c := v.(type) {
	case *Color:
		if contextIdx, ptrOffset, inForEach := t.elemContextForPtr(uintptr(unsafe.Pointer(c)), elemBase, elemSize); inForEach {
			storage := new(Color)
			*storage = *c
			eval := func() {
				base := t.runtimeElemBase(contextIdx)
				if base != nil {
					*storage = *(*Color)(unsafe.Pointer(uintptr(base) + ptrOffset))
				}
			}
			t.itemEvals = append(t.itemEvals, eval)
			return storage
		}
		return c
	case conditionNode:
		return t.compileCondColor(c, elemBase, elemSize)
	case valueBranchNode:
		return t.compileBranchColor(c, elemBase, elemSize)
	case tweenNode:
		return t.compileTweenColor(c, elemBase, elemSize)
	case OscC:
		return t.compileOscColor(c)
	}
	return nil
}

func (t *Template) elemContextForPtr(ptrAddr uintptr, elemBase unsafe.Pointer, elemSize uintptr) (int, uintptr, bool) {
	for i := len(t.compileElemContexts) - 1; i >= 0; i-- {
		ctx := t.compileElemContexts[i]
		if ctx.base == nil || ctx.size == 0 {
			continue
		}
		baseAddr := uintptr(ctx.base)
		if ptrAddr >= baseAddr && ptrAddr < baseAddr+ctx.size {
			return i, ptrAddr - baseAddr, true
		}
	}
	if elemBase != nil && elemSize > 0 {
		baseAddr := uintptr(elemBase)
		if ptrAddr >= baseAddr && ptrAddr < baseAddr+elemSize {
			return len(t.compileElemContexts) - 1, ptrAddr - baseAddr, true
		}
	}
	return -1, 0, false
}

func (t *Template) compileDynStyle(v any, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	switch c := v.(type) {
	case *Style:
		if contextIdx, ptrOffset, inForEach := t.elemContextForPtr(uintptr(unsafe.Pointer(c)), elemBase, elemSize); inForEach {
			storage := new(Style)
			*storage = *c
			eval := func() {
				base := t.runtimeElemBase(contextIdx)
				if base != nil {
					*storage = *(*Style)(unsafe.Pointer(uintptr(base) + ptrOffset))
				}
			}
			t.itemEvals = append(t.itemEvals, eval)
			return storage
		}
		return c
	case conditionNode:
		return t.compileCondStyle(c, elemBase, elemSize)
	case valueBranchNode:
		return t.compileBranchStyle(c, elemBase, elemSize)
	case tweenNode:
		return t.compileTweenStyle(c, elemBase, elemSize)
	}
	return nil
}

// compileStyleDyn wires styleDyn/fgDyn/bgDyn into a *Style for any leaf component.
// returns nil if no dynamic styling is needed.
func (t *Template) compileStyleDyn(baseStyle Style, styleDyn, fgDyn, bgDyn any, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	if styleDyn != nil {
		return t.compileDynStyle(styleDyn, elemBase, elemSize)
	}
	if fgDyn == nil && bgDyn == nil {
		return nil
	}
	storage := new(Style)
	*storage = baseStyle
	var fgPtr *Color
	var bgPtr *Color
	if fgDyn != nil {
		fgPtr = t.compileDynColor(fgDyn, elemBase, elemSize)
	}
	if bgDyn != nil {
		bgPtr = t.compileDynColor(bgDyn, elemBase, elemSize)
	}
	base := baseStyle
	eval := func() {
		s := base
		if fgPtr != nil {
			s.FG = *fgPtr
		}
		if bgPtr != nil {
			s.BG = *bgPtr
		}
		*storage = s
	}
	// prime the storage so the very first frame renders live values instead
	// of the zero base style (branch evals only run once a branch is active)
	eval()
	if elemBase != nil && elemSize > 0 {
		t.itemEvals = append(t.itemEvals, eval)
	} else {
		root := t.evalRoot()
		root.evals = append(root.evals, eval)
	}
	return storage
}

func (t *Template) compileCondColor(cond conditionNode, elemBase unsafe.Pointer, elemSize uintptr) *Color {
	storage := new(Color)

	isOutermost := t.compilePropertyPtr == nil
	if isOutermost {
		t.compilePropertyPtr = storage
		t.compileTweenItemsMaps = nil
		defer func() {
			t.compilePropertyPtr = nil
			t.compileTweenItemsMaps = nil
		}()
	}

	thenVal := cond.getThen()
	elseVal := cond.getElse()

	// recursively compile nested conditions, tweens, and reactive pointers
	resolveColor := func(v any) func() Color {
		switch nested := v.(type) {
		case conditionNode:
			ptr := t.compileCondColor(nested, elemBase, elemSize)
			return func() Color { return *ptr }
		case tweenNode:
			ptr, items := t.compileTweenColorItems(nested, elemBase, elemSize)
			if items != nil {
				t.compileTweenItemsMaps = append(t.compileTweenItemsMaps, items)
			}
			return func() Color { return *ptr }
		case *Color:
			return func() Color { return *nested }
		default:
			c := anyToColor(v)
			return func() Color { return c }
		}
	}
	thenFn := resolveColor(thenVal)
	elseFn := resolveColor(elseVal)

	contextIdx, ptrOffset, inForEach := t.elemContextForPtr(cond.getPtrAddr(), elemBase, elemSize)
	if inForEach {
		cond.setOffset(ptrOffset)
	}

	if inForEach {
		if cond.evaluate() {
			*storage = thenFn()
		} else {
			*storage = elseFn()
		}
		eval := func() {
			if cond.evaluateWithBase(t.runtimeElemBase(contextIdx)) {
				*storage = thenFn()
			} else {
				*storage = elseFn()
			}
		}
		t.itemEvals = append(t.itemEvals, eval)

		// outermost condition records per-item displayed value into nested tweens
		if isOutermost && len(t.compileTweenItemsMaps) > 0 {
			propPtr := storage
			tweenMaps := t.compileTweenItemsMaps
			t.itemEvals = append(t.itemEvals, func() {
				displayed := *propPtr
				key := t.elemBase
				for _, items := range tweenMaps {
					if state, ok := items[key]; ok {
						state.lastDisplayed = displayed
					}
				}
			})
		}
	} else {
		root := t.evalRoot()
		eval := func() {
			if cond.evaluate() {
				*storage = thenFn()
			} else {
				*storage = elseFn()
			}
		}
		eval()
		root.evals = append(root.evals, eval)
	}
	return storage
}

func (t *Template) compileCondStyle(cond conditionNode, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	storage := new(Style)
	thenVal := cond.getThen()
	elseVal := cond.getElse()

	// recursively compile nested conditions, tweens, and reactive pointers
	resolveStyle := func(v any) func() Style {
		switch nested := v.(type) {
		case conditionNode:
			ptr := t.compileCondStyle(nested, elemBase, elemSize)
			return func() Style { return *ptr }
		case tweenNode:
			ptr := t.compileTweenStyle(nested, elemBase, elemSize)
			return func() Style { return *ptr }
		case *Style:
			return func() Style { return *nested }
		default:
			s := anyToStyle(v)
			return func() Style { return s }
		}
	}
	thenFn := resolveStyle(thenVal)
	elseFn := resolveStyle(elseVal)

	// check if the condition pointer is within a ForEach element
	contextIdx, ptrOffset, inForEach := t.elemContextForPtr(cond.getPtrAddr(), elemBase, elemSize)
	if inForEach {
		cond.setOffset(ptrOffset)
	}

	if inForEach {
		if cond.evaluate() {
			*storage = thenFn()
		} else {
			*storage = elseFn()
		}
		eval := func() {
			if cond.evaluateWithBase(t.runtimeElemBase(contextIdx)) {
				*storage = thenFn()
			} else {
				*storage = elseFn()
			}
		}
		t.itemEvals = append(t.itemEvals, eval)
	} else {
		// global eval — runs once per frame
		root := t.evalRoot()
		eval := func() {
			if cond.evaluate() {
				*storage = thenFn()
			} else {
				*storage = elseFn()
			}
		}
		eval()
		root.evals = append(root.evals, eval)
	}
	return storage
}

func (t *Template) compileBranchColor(branch valueBranchNode, elemBase unsafe.Pointer, elemSize uintptr) *Color {
	storage := new(Color)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	contextIdx := t.prepareValueBranchForBase(branch, elemBase, elemSize)
	inForEach := contextIdx >= 0
	eval := func() {
		idx := t.valueBranchIndex(branch, contextIdx)
		if idx >= 0 && idx < len(cases) {
			*storage = anyToColor(cases[idx])
			return
		}
		*storage = anyToColor(def)
	}
	eval()
	if inForEach {
		t.itemEvals = append(t.itemEvals, eval)
	} else {
		root := t.evalRoot()
		root.evals = append(root.evals, eval)
	}
	return storage
}

func (t *Template) compileBranchStyle(branch valueBranchNode, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	storage := new(Style)
	cases := branch.getCaseNodes()
	def := branch.getDefaultNode()
	contextIdx := t.prepareValueBranchForBase(branch, elemBase, elemSize)
	inForEach := contextIdx >= 0
	eval := func() {
		idx := t.valueBranchIndex(branch, contextIdx)
		if idx >= 0 && idx < len(cases) {
			*storage = anyToStyle(cases[idx])
			return
		}
		*storage = anyToStyle(def)
	}
	eval()
	if inForEach {
		t.itemEvals = append(t.itemEvals, eval)
	} else {
		root := t.evalRoot()
		root.evals = append(root.evals, eval)
	}
	return storage
}

func (t *Template) prepareValueBranchForBase(branch valueBranchNode, elemBase unsafe.Pointer, elemSize uintptr) int {
	if elemBase == nil || elemSize == 0 {
		return -1
	}
	base, ok := branch.(interface {
		getPtrAddr() uintptr
		setPtrOffset(uintptr)
	})
	if !ok {
		return -1
	}
	contextIdx, offset, ok := t.elemContextForPtr(base.getPtrAddr(), elemBase, elemSize)
	if ok {
		base.setPtrOffset(offset)
		return contextIdx
	}
	return -1
}

func (t *Template) valueBranchIndex(branch valueBranchNode, contextIdx int) int {
	if contextIdx >= 0 {
		if base, ok := branch.(interface{ getMatchIndexWithBase(unsafe.Pointer) int }); ok {
			return base.getMatchIndexWithBase(t.runtimeElemBase(contextIdx))
		}
	}
	return branch.getMatchIndex()
}

func anyToColor(v any) Color {
	switch val := v.(type) {
	case Color:
		return val
	case *Color:
		return *val
	}
	return Color{}
}

func anyToStyle(v any) Style {
	switch val := v.(type) {
	case Style:
		return val
	case *Style:
		return *val
	}
	return Style{}
}

// Animating returns true if any tween is currently in progress.
// check this after Execute to determine if another frame is needed.
func (t *Template) Animating() bool { return t.animating }

// compileTween resolves a tweenNode's target to a typed pointer, allocates
// interpolation storage, and registers a per-frame evaluator that watches the
// target and lerps toward it. all tweens in a frame share t.frameTime.

func (t *Template) compileTweenScalar(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr, target func() float64, assign func(float64), current func() float64, convert func(any) float64, resolveOut func(tweenNode) func() float64) {
	root := t.evalRoot()
	durVal := tw.getTweenDuration()
	durPtr := tw.getTweenDurationPtr()
	onComplete := tw.getTweenOnComplete()
	ease := tw.getTweenEasing()
	outTw := tw.getTweenOut()
	var outTarget func() float64
	var outDurVal time.Duration
	var outDurPtr *time.Duration
	var outEase func(float64) float64
	var outOnComplete func()
	if outTw != nil {
		t.registerExitTween(tw)
		outTarget = resolveOut(outTw)
		outDurVal = outTw.getTweenDuration()
		outDurPtr = outTw.getTweenDurationPtr()
		outEase = outTw.getTweenEasing()
		outOnComplete = outTw.getTweenOnComplete()
	}

	fromVal := 0.0
	hasFrom := false
	if from := tw.getTweenFrom(); from != nil {
		hasFrom = true
		fromVal = convert(from)
		assign(fromVal)
	}

	run := func(state *perItemFloat64State, exiting bool, key unsafe.Pointer) {
		dur := durVal
		if durPtr != nil {
			dur = *durPtr
		}
		targetVal := target()
		now := root.frameTime
		if outTw != nil && exiting {
			outDur := outDurVal
			if outDurPtr != nil {
				outDur = *outDurPtr
			}
			outVal := outTarget()
			if state.exitComplete {
				state.current = outVal
				assign(state.current)
				t.setExitLease(&state.exitLeaseActive, false)
				return
			}
			if !state.exitActive {
				state.exitActive = true
				state.startVal = state.current
				if from := outTw.getTweenFrom(); from != nil {
					state.startVal = convert(from)
					state.current = state.startVal
				}
				state.lastTarget = outVal
				state.startTime = now
			} else if outVal != state.lastTarget {
				state.startVal = state.current
				state.lastTarget = outVal
				state.startTime = now
			}
			t.setExitLease(&state.exitLeaseActive, true)
			if state.startTime.IsZero() {
				assign(state.current)
				return
			}
			elapsed := now.Sub(state.startTime)
			if elapsed >= outDur {
				state.current = outVal
				assign(state.current)
				state.startTime = time.Time{}
				state.exitActive = false
				state.exitComplete = true
				t.setExitLease(&state.exitLeaseActive, false)
				if outOnComplete != nil {
					t.deferComplete(outOnComplete)
				}
				return
			}
			progress := float64(elapsed) / float64(outDur)
			if outEase != nil {
				progress = outEase(progress)
			}
			state.current = state.startVal + progress*(outVal-state.startVal)
			assign(state.current)
			root.animating = true
			return
		}
		if outTw != nil {
			state.exitActive = false
			state.exitComplete = false
			t.setExitLease(&state.exitLeaseActive, false)
		}
		if state.needsFirstFrame {
			state.startVal = state.current
			state.lastTarget = targetVal
			state.startTime = now
			state.needsFirstFrame = false
		} else if targetVal != state.lastTarget {
			state.startVal = state.current
			state.lastTarget = targetVal
			state.startTime = now
		}
		if state.startTime.IsZero() {
			state.current = targetVal
			assign(state.current)
			return
		}
		elapsed := now.Sub(state.startTime)
		if elapsed >= dur {
			state.current = targetVal
			assign(state.current)
			state.startTime = time.Time{}
			if onComplete != nil {
				t.deferComplete(onComplete)
			}
			return
		}
		progress := float64(elapsed) / float64(dur)
		if ease != nil {
			progress = ease(progress)
		}
		state.current = state.startVal + progress*(targetVal-state.startVal)
		assign(state.current)
		root.animating = true
	}

	if elemBase != nil && elemSize > 0 {
		items := make(map[unsafe.Pointer]*perItemFloat64State)
		t.itemEvals = append(t.itemEvals, func() {
			key := t.elemBase
			if key == nil {
				return
			}
			state, ok := items[key]
			if !ok {
				initial := target()
				state = &perItemFloat64State{
					lastTarget:      initial,
					startVal:        initial,
					current:         initial,
					needsFirstFrame: hasFrom,
				}
				if hasFrom {
					state.startVal = fromVal
					state.current = fromVal
				}
				items[key] = state
			}
			run(state, t.isExitRenderingFor(key), key)
		})
		return
	}

	state := &perItemFloat64State{
		lastTarget:      target(),
		startVal:        current(),
		current:         current(),
		needsFirstFrame: hasFrom,
	}
	if hasFrom {
		state.startVal = fromVal
		state.current = fromVal
	}
	eval := func() {
		run(state, t.exit.rendering, nil)
	}
	root.evals = append(root.evals, eval)
	if outTw != nil {
		t.exitEvals = append(t.exitEvals, eval)
	}
}

func (t *Template) compileTweenInt16(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) *int16 {
	watchPtr := t.resolveTweenTargetInt16(tw.getTarget())
	storage := new(int16)
	*storage = *watchPtr
	t.compileTweenScalar(
		tw,
		elemBase,
		elemSize,
		func() float64 { return float64(*watchPtr) },
		func(v float64) { *storage = int16(v) },
		func() float64 { return float64(*storage) },
		func(v any) float64 { return float64(anyToInt16(v)) },
		func(out tweenNode) func() float64 {
			outPtr := t.resolveTweenTargetInt16(out.getTarget())
			return func() float64 { return float64(*outPtr) }
		},
	)
	return storage
}

func (t *Template) compileTweenFloat32(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) *float32 {
	watchPtr := t.resolveTweenTargetFloat32(tw.getTarget())
	storage := new(float32)
	*storage = *watchPtr
	t.compileTweenScalar(
		tw,
		elemBase,
		elemSize,
		func() float64 { return float64(*watchPtr) },
		func(v float64) { *storage = float32(v) },
		func() float64 { return float64(*storage) },
		anyToFloat64,
		func(out tweenNode) func() float64 {
			outPtr := t.resolveTweenTargetFloat32(out.getTarget())
			return func() float64 { return float64(*outPtr) }
		},
	)
	return storage
}

// compileOscFloat64 registers a frame evaluator deriving the oscillator's
// value from the shared frame clock. Resolving marks the template animating,
// so the gated ticker runs exactly while an oscillator is reachable.
func (t *Template) compileOscInt16(o OscC) *int16 {
	root := t.evalRoot()
	storage := new(int16)
	var acc oscAccum
	osc := o
	eval := func() {
		root.animating = true
		*storage = int16(osc.resolve(root.frameTime.Sub(root.oscEpoch), &acc) + 0.5)
	}
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileOscFloat64(o OscC) *float64 {
	root := t.evalRoot()
	storage := new(float64)
	var acc oscAccum
	osc := o
	eval := func() {
		root.animating = true
		*storage = osc.resolve(root.frameTime.Sub(root.oscEpoch), &acc)
	}
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileOscFloat32(o OscC) *float32 {
	root := t.evalRoot()
	storage := new(float32)
	var acc oscAccum
	osc := o
	eval := func() {
		root.animating = true
		*storage = float32(osc.resolve(root.frameTime.Sub(root.oscEpoch), &acc))
	}
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileOscColor(o OscC) *Color {
	root := t.evalRoot()
	storage := new(Color)
	var acc oscAccum
	osc := o
	eval := func() {
		root.animating = true
		v := osc.resolve(root.frameTime.Sub(root.oscEpoch), &acc)
		*storage = Lerp(osc.colA, osc.colB, v)
	}
	root.evals = append(root.evals, eval)
	return storage
}

func (t *Template) compileTweenFloat64(tw tweenNode, armed *bool, elemBase unsafe.Pointer, elemSize uintptr) *float64 {
	root := t.evalRoot()
	watchPtr := t.resolveTweenTargetFloat64(tw.getTarget())
	storage := new(float64)
	*storage = *watchPtr
	durVal := tw.getTweenDuration()
	durPtr := tw.getTweenDurationPtr()
	onComplete := tw.getTweenOnComplete()
	ease := tw.getTweenEasing()
	outTw := tw.getTweenOut()
	var outWatchPtr *float64
	var outDurVal time.Duration
	var outDurPtr *time.Duration
	var outEase func(float64) float64
	var outOnComplete func()
	if outTw != nil {
		t.registerExitTween(tw)
		outWatchPtr = t.resolveTweenTargetFloat64(outTw.getTarget())
		outDurVal = outTw.getTweenDuration()
		outDurPtr = outTw.getTweenDurationPtr()
		outEase = outTw.getTweenEasing()
		outOnComplete = outTw.getTweenOnComplete()
	}

	var fromVal float64
	hasFrom := false
	if from := tw.getTweenFrom(); from != nil {
		hasFrom = true
		fromVal = anyToFloat64(from)
		*storage = fromVal
	}

	inForEach := elemBase != nil && elemSize > 0
	if inForEach {
		// item-field targets compiled against the dummy element must rebase
		// per rendered element, like every other per-item binding; a raw
		// pointer would read the compile-time placeholder forever
		watchOff, watchInItem := uintptr(0), false
		if isWithinRange(unsafe.Pointer(watchPtr), elemBase, elemSize) {
			watchOff = uintptr(unsafe.Pointer(watchPtr)) - uintptr(elemBase)
			watchInItem = true
		}
		readWatch := func(key unsafe.Pointer) float64 {
			if watchInItem {
				return *(*float64)(unsafe.Pointer(uintptr(key) + watchOff))
			}
			return *watchPtr
		}
		outOff, outInItem := uintptr(0), false
		if outWatchPtr != nil && isWithinRange(unsafe.Pointer(outWatchPtr), elemBase, elemSize) {
			outOff = uintptr(unsafe.Pointer(outWatchPtr)) - uintptr(elemBase)
			outInItem = true
		}
		readOut := func(key unsafe.Pointer) float64 {
			if outInItem {
				return *(*float64)(unsafe.Pointer(uintptr(key) + outOff))
			}
			return *outWatchPtr
		}
		items := make(map[unsafe.Pointer]*perItemFloat64State)
		t.itemEvals = append(t.itemEvals, func() {
			key := t.elemBase
			if key == nil {
				return
			}
			state, ok := items[key]
			if !ok {
				initial := readWatch(key)
				state = &perItemFloat64State{
					lastTarget:      initial,
					startVal:        initial,
					current:         initial,
					needsFirstFrame: hasFrom,
					wasActive:       armed == nil,
				}
				if hasFrom {
					state.startVal = fromVal
					state.current = fromVal
				}
				items[key] = state
			}

			dur := durVal
			if durPtr != nil {
				dur = *durPtr
			}
			target := readWatch(key)
			now := root.frameTime

			if outTw != nil && t.isExitRenderingFor(key) {
				outDur := outDurVal
				if outDurPtr != nil {
					outDur = *outDurPtr
				}
				outTarget := readOut(key)
				if state.exitComplete {
					state.current = outTarget
					*storage = state.current
					t.setExitLease(&state.exitLeaseActive, false)
					return
				}
				if !state.exitActive {
					state.exitActive = true
					state.startVal = state.current
					if from := outTw.getTweenFrom(); from != nil {
						state.startVal = anyToFloat64(from)
						state.current = state.startVal
					}
					state.lastTarget = outTarget
					state.startTime = now
				} else if outTarget != state.lastTarget {
					state.startVal = state.current
					state.lastTarget = outTarget
					state.startTime = now
				}
				t.setExitLease(&state.exitLeaseActive, true)
				if state.startTime.IsZero() {
					*storage = state.current
					return
				}
				elapsed := now.Sub(state.startTime)
				if elapsed >= outDur {
					state.current = outTarget
					*storage = state.current
					state.startTime = time.Time{}
					state.exitActive = false
					state.exitComplete = true
					t.setExitLease(&state.exitLeaseActive, false)
					if outOnComplete != nil {
						t.deferComplete(outOnComplete)
					}
					return
				}
				progress := float64(elapsed) / float64(outDur)
				if outEase != nil {
					progress = outEase(progress)
				}
				state.current = state.startVal + progress*(outTarget-state.startVal)
				*storage = state.current
				root.animating = true
				return
			}

			if outTw != nil {
				state.exitActive = false
				state.exitComplete = false
				t.setExitLease(&state.exitLeaseActive, false)
			}

			if armed != nil && !state.wasActive {
				state.wasActive = true
				if hasFrom {
					state.current = fromVal
					state.startVal = fromVal
				} else {
					state.startVal = state.current
				}
				state.lastTarget = target
				state.startTime = now
				state.needsFirstFrame = false
			} else if state.needsFirstFrame {
				state.startVal = state.current
				state.lastTarget = target
				state.startTime = now
				state.needsFirstFrame = false
			} else if target != state.lastTarget {
				state.startVal = state.current
				state.lastTarget = target
				state.startTime = now
			}

			if state.startTime.IsZero() {
				state.current = target
				*storage = state.current
				return
			}
			elapsed := now.Sub(state.startTime)
			if elapsed >= dur {
				state.current = target
				*storage = state.current
				state.startTime = time.Time{}
				if onComplete != nil {
					t.deferComplete(onComplete)
				}
				return
			}
			progress := float64(elapsed) / float64(dur)
			if ease != nil {
				progress = ease(progress)
			}
			state.current = state.startVal + progress*(target-state.startVal)
			*storage = state.current
			root.animating = true
		})
		return storage
	}

	lastTarget := *watchPtr
	startVal := *watchPtr
	var startTime time.Time
	needsFirstFrame := false
	exitActive := false
	exitComplete := false
	exitLeaseActive := false

	if hasFrom {
		*storage = fromVal
		startVal = fromVal
		needsFirstFrame = true
	}

	// tracks whether resolve() was called last frame (effect was active)
	wasActive := armed == nil // nil armed = always active (non-effect tweens)

	eval := func() {
		dur := durVal
		if durPtr != nil {
			dur = *durPtr
		}
		target := *watchPtr
		now := root.frameTime
		if outTw != nil && t.exit.rendering {
			outDur := outDurVal
			if outDurPtr != nil {
				outDur = *outDurPtr
			}
			outTarget := *outWatchPtr
			if exitComplete {
				*storage = outTarget
				t.setExitLease(&exitLeaseActive, false)
				return
			}
			if !exitActive {
				exitActive = true
				startVal = *storage
				if from := outTw.getTweenFrom(); from != nil {
					startVal = anyToFloat64(from)
					*storage = startVal
				}
				lastTarget = outTarget
				startTime = now
			} else if outTarget != lastTarget {
				startVal = *storage
				lastTarget = outTarget
				startTime = now
			}
			t.setExitLease(&exitLeaseActive, true)
			if startTime.IsZero() {
				return
			}
			elapsed := now.Sub(startTime)
			if elapsed >= outDur {
				*storage = outTarget
				startTime = time.Time{}
				exitActive = false
				exitComplete = true
				t.setExitLease(&exitLeaseActive, false)
				if outOnComplete != nil {
					t.deferComplete(outOnComplete)
				}
				return
			}
			progress := float64(elapsed) / float64(outDur)
			if outEase != nil {
				progress = outEase(progress)
			}
			*storage = startVal + progress*(outTarget-startVal)
			root.animating = true
			return
		}
		if outTw != nil {
			exitActive = false
			exitComplete = false
			t.setExitLease(&exitLeaseActive, false)
		}

		// activation gating: From tweens in screen effects wait for resolve()
		if armed != nil {
			active := *armed
			*armed = false // reset each frame; resolve() re-sets if still active

			if !active {
				wasActive = false
				if hasFrom {
					*storage = fromVal // reset so stale target doesn't flash on re-open
				}
				return
			}

			if !wasActive {
				// inactive → active transition: (re)start From animation when
				// requested; otherwise animate from the last displayed value.
				wasActive = true
				if hasFrom {
					*storage = fromVal
					startVal = fromVal
				} else {
					startVal = *storage
				}
				lastTarget = target
				startTime = now
				needsFirstFrame = false
				goto interpolate
			}
		}

		if needsFirstFrame {
			startVal = *storage
			lastTarget = target
			startTime = now
			needsFirstFrame = false
		} else if target != lastTarget {
			startVal = *storage
			lastTarget = target
			startTime = now
		}

	interpolate:
		if startTime.IsZero() {
			return
		}
		elapsed := now.Sub(startTime)
		if elapsed >= dur {
			*storage = target
			startTime = time.Time{}
			if onComplete != nil {
				t.deferComplete(onComplete)
			}
			return
		}
		progress := float64(elapsed) / float64(dur)
		if ease != nil {
			progress = ease(progress)
		}
		*storage = startVal + progress*(target-startVal)
		root.animating = true
	}
	root.evals = append(root.evals, eval)
	if outTw != nil {
		t.exitEvals = append(t.exitEvals, eval)
	}
	return storage
}

func (t *Template) compileTweenInt8(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) *int8 {
	watchPtr := t.resolveTweenTargetInt8(tw.getTarget())
	storage := new(int8)
	*storage = *watchPtr
	t.compileTweenScalar(
		tw,
		elemBase,
		elemSize,
		func() float64 { return float64(*watchPtr) },
		func(v float64) { *storage = int8(v) },
		func() float64 { return float64(*storage) },
		func(v any) float64 { return float64(anyToInt8(v)) },
		func(out tweenNode) func() float64 {
			outPtr := t.resolveTweenTargetInt8(out.getTarget())
			return func() float64 { return float64(*outPtr) }
		},
	)
	return storage
}

// resolve tween targets — unwrap conditionNode or pointer, same as properties
func (t *Template) resolveTweenTargetInt16(target any) *int16 {
	switch v := target.(type) {
	case *int16:
		return v
	case *int:
		// bridge *int to *int16 via an eval that syncs each frame
		storage := new(int16)
		*storage = int16(*v)
		root := t.evalRoot()
		root.evals = append(root.evals, func() {
			*storage = int16(*v)
		})
		return storage
	case conditionNode:
		return t.compileCondInt16(v)
	}
	// static fallback: allocate storage with the value
	storage := new(int16)
	*storage = anyToInt16(target)
	return storage
}

func (t *Template) resolveTweenTargetFloat32(target any) *float32 {
	switch v := target.(type) {
	case *float32:
		return v
	case conditionNode:
		return t.compileCondFloat32(v)
	}
	storage := new(float32)
	*storage = anyToFloat32(target)
	return storage
}

func (t *Template) resolveTweenTargetFloat64(target any) *float64 {
	switch v := target.(type) {
	case *float64:
		return v
	case conditionNode:
		return t.compileCondFloat64(v)
	}
	storage := new(float64)
	*storage = anyToFloat64(target)
	return storage
}

func (t *Template) resolveTweenTargetInt8(target any) *int8 {
	switch v := target.(type) {
	case *int8:
		return v
	case conditionNode:
		return t.compileCondInt8(v)
	}
	storage := new(int8)
	*storage = anyToInt8(target)
	return storage
}

type perItemColorState struct {
	lastTarget      Color
	startVal        Color
	current         Color
	startTime       time.Time
	lastDisplayed   Color // what the property actually showed for this item last frame
	needsFirstFrame bool
	exitActive      bool
	exitComplete    bool
	exitLeaseActive bool
}

type perItemFloat64State struct {
	lastTarget      float64
	startVal        float64
	current         float64
	startTime       time.Time
	needsFirstFrame bool
	exitActive      bool
	exitComplete    bool
	exitLeaseActive bool
	wasActive       bool
}

type perItemStyleState struct {
	lastTarget      Style
	startVal        Style
	current         Style
	startTime       time.Time
	needsFirstFrame bool
	exitActive      bool
	exitComplete    bool
	exitLeaseActive bool
}

func (t *Template) compileTweenColorItems(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) (*Color, map[unsafe.Pointer]*perItemColorState) {
	items := make(map[unsafe.Pointer]*perItemColorState)
	ptr := t.compileTweenColorInner(tw, elemBase, elemSize, items)
	if elemBase == nil || elemSize == 0 {
		return ptr, nil // not ForEach, no per-item tracking
	}
	return ptr, items
}

func (t *Template) compileTweenColor(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) *Color {
	return t.compileTweenColorInner(tw, elemBase, elemSize, nil)
}

func (t *Template) compileTweenColorInner(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr, sharedItems map[unsafe.Pointer]*perItemColorState) *Color {
	root := t.evalRoot()
	watchPtr := t.resolveTweenTargetColor(tw.getTarget(), elemBase, elemSize)
	storage := new(Color)
	*storage = *watchPtr
	durVal := tw.getTweenDuration()
	durPtr := tw.getTweenDurationPtr()
	onComplete := tw.getTweenOnComplete()
	ease := tw.getTweenEasing()
	outTw := tw.getTweenOut()
	var outWatchPtr *Color
	var outDurVal time.Duration
	var outDurPtr *time.Duration
	var outEase func(float64) float64
	var outOnComplete func()
	if outTw != nil {
		t.registerExitTween(tw)
		outWatchPtr = t.resolveTweenTargetColor(outTw.getTarget(), elemBase, elemSize)
		outDurVal = outTw.getTweenDuration()
		outDurPtr = outTw.getTweenDurationPtr()
		outEase = outTw.getTweenEasing()
		outOnComplete = outTw.getTweenOnComplete()
	}

	fromVal := Color{}
	hasFrom := false
	if from := tw.getTweenFrom(); from != nil {
		hasFrom = true
		fromVal = anyToColor(from)
		*storage = fromVal
	}

	// detect ForEach context
	inForEach := elemBase != nil && elemSize > 0

	// item-field targets compiled against the dummy element rebase per
	// rendered element (t.elemBase at run time); raw pointers would read
	// the compile-time placeholder forever
	watchOff, watchInItem := uintptr(0), false
	if inForEach && isWithinRange(unsafe.Pointer(watchPtr), elemBase, elemSize) {
		watchOff = uintptr(unsafe.Pointer(watchPtr)) - uintptr(elemBase)
		watchInItem = true
	}
	readWatch := func() Color {
		if watchInItem {
			if key := t.elemBase; key != nil {
				return *(*Color)(unsafe.Pointer(uintptr(key) + watchOff))
			}
		}
		return *watchPtr
	}
	outOff, outInItem := uintptr(0), false
	if inForEach && outWatchPtr != nil && isWithinRange(unsafe.Pointer(outWatchPtr), elemBase, elemSize) {
		outOff = uintptr(unsafe.Pointer(outWatchPtr)) - uintptr(elemBase)
		outInItem = true
	}
	readOut := func() Color {
		if outInItem {
			if key := t.elemBase; key != nil {
				return *(*Color)(unsafe.Pointer(uintptr(key) + outOff))
			}
		}
		return *outWatchPtr
	}

	run := func(state *perItemColorState, exiting bool) {
		dur := durVal
		if durPtr != nil {
			dur = *durPtr
		}
		target := readWatch()
		now := root.frameTime
		if outTw != nil && exiting {
			outDur := outDurVal
			if outDurPtr != nil {
				outDur = *outDurPtr
			}
			outTarget := readOut()
			if state.exitComplete {
				state.current = outTarget
				*storage = state.current
				t.setExitLease(&state.exitLeaseActive, false)
				return
			}
			if !state.exitActive {
				state.exitActive = true
				state.startVal = state.current
				if from := outTw.getTweenFrom(); from != nil {
					state.startVal = anyToColor(from)
					state.current = state.startVal
				}
				state.lastTarget = outTarget
				state.startTime = now
			} else if outTarget != state.lastTarget {
				state.startVal = state.current
				state.lastTarget = outTarget
				state.startTime = now
			}
			t.setExitLease(&state.exitLeaseActive, true)
			if state.startTime.IsZero() {
				*storage = state.current
				return
			}
			elapsed := now.Sub(state.startTime)
			if elapsed >= outDur {
				state.current = outTarget
				*storage = state.current
				state.startTime = time.Time{}
				state.exitActive = false
				state.exitComplete = true
				t.setExitLease(&state.exitLeaseActive, false)
				if outOnComplete != nil {
					t.deferComplete(outOnComplete)
				}
				return
			}
			progress := float64(elapsed) / float64(outDur)
			if outEase != nil {
				progress = outEase(progress)
			}
			state.current = lerpColor(state.startVal, outTarget, progress)
			*storage = state.current
			root.animating = true
			return
		}
		if outTw != nil {
			state.exitActive = false
			state.exitComplete = false
			t.setExitLease(&state.exitLeaseActive, false)
		}
		if state.lastDisplayed != (Color{}) && state.lastDisplayed != state.current {
			state.startVal = state.lastDisplayed
			state.current = state.lastDisplayed
			state.startTime = now
		}
		if state.needsFirstFrame {
			state.startVal = state.current
			state.lastTarget = target
			state.startTime = now
			state.needsFirstFrame = false
		} else if target != state.lastTarget {
			state.startVal = state.current
			state.lastTarget = target
			state.startTime = now
		}
		if state.startTime.IsZero() {
			state.current = target
			*storage = target
			return
		}
		elapsed := now.Sub(state.startTime)
		if elapsed >= dur {
			state.current = target
			*storage = target
			state.startTime = time.Time{}
			if onComplete != nil {
				t.deferComplete(onComplete)
			}
			return
		}
		progress := float64(elapsed) / float64(dur)
		if ease != nil {
			progress = ease(progress)
		}
		state.current = lerpColor(state.startVal, target, progress)
		*storage = state.current
		root.animating = true
	}

	if inForEach {
		items := sharedItems
		if items == nil {
			items = make(map[unsafe.Pointer]*perItemColorState)
		}
		t.itemEvals = append(t.itemEvals, func() {
			key := t.elemBase
			target := readWatch()
			state, ok := items[key]
			if !ok {
				state = &perItemColorState{lastTarget: target, startVal: target, current: target, needsFirstFrame: hasFrom}
				if hasFrom {
					state.startVal = fromVal
					state.current = fromVal
				}
				items[key] = state
			}
			run(state, t.isExitRenderingFor(key))
		})
	} else {
		state := &perItemColorState{lastTarget: *watchPtr, startVal: *storage, current: *storage, needsFirstFrame: hasFrom}
		if hasFrom {
			state.startVal = fromVal
			state.current = fromVal
		}
		eval := func() {
			run(state, t.exit.rendering)
		}
		root.evals = append(root.evals, eval)
		if outTw != nil {
			t.exitEvals = append(t.exitEvals, eval)
		}
	}
	return storage
}

func (t *Template) compileTweenStyle(tw tweenNode, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	root := t.evalRoot()
	watchPtr := t.resolveTweenTargetStyle(tw.getTarget(), elemBase, elemSize)
	storage := new(Style)
	*storage = *watchPtr
	durVal := tw.getTweenDuration()
	durPtr := tw.getTweenDurationPtr()
	onComplete := tw.getTweenOnComplete()
	ease := tw.getTweenEasing()
	outTw := tw.getTweenOut()
	var outWatchPtr *Style
	var outDurVal time.Duration
	var outDurPtr *time.Duration
	var outEase func(float64) float64
	var outOnComplete func()
	if outTw != nil {
		t.registerExitTween(tw)
		outWatchPtr = t.resolveTweenTargetStyle(outTw.getTarget(), elemBase, elemSize)
		outDurVal = outTw.getTweenDuration()
		outDurPtr = outTw.getTweenDurationPtr()
		outEase = outTw.getTweenEasing()
		outOnComplete = outTw.getTweenOnComplete()
	}

	fromVal := Style{}
	hasFrom := false
	if from := tw.getTweenFrom(); from != nil {
		hasFrom = true
		fromVal = anyToStyle(from)
		*storage = fromVal
	}

	// detect ForEach context
	inForEach := elemBase != nil && elemSize > 0

	// item-field targets compiled against the dummy element rebase per
	// rendered element (t.elemBase at run time); raw pointers would read
	// the compile-time placeholder forever
	styleWatchOff, styleWatchInItem := uintptr(0), false
	if inForEach && isWithinRange(unsafe.Pointer(watchPtr), elemBase, elemSize) {
		styleWatchOff = uintptr(unsafe.Pointer(watchPtr)) - uintptr(elemBase)
		styleWatchInItem = true
	}
	readWatch := func() Style {
		if styleWatchInItem {
			if key := t.elemBase; key != nil {
				return *(*Style)(unsafe.Pointer(uintptr(key) + styleWatchOff))
			}
		}
		return *watchPtr
	}
	styleOutOff, styleOutInItem := uintptr(0), false
	if inForEach && outWatchPtr != nil && isWithinRange(unsafe.Pointer(outWatchPtr), elemBase, elemSize) {
		styleOutOff = uintptr(unsafe.Pointer(outWatchPtr)) - uintptr(elemBase)
		styleOutInItem = true
	}
	readOut := func() Style {
		if styleOutInItem {
			if key := t.elemBase; key != nil {
				return *(*Style)(unsafe.Pointer(uintptr(key) + styleOutOff))
			}
		}
		return *outWatchPtr
	}

	run := func(state *perItemStyleState, exiting bool) {
		dur := durVal
		if durPtr != nil {
			dur = *durPtr
		}
		target := readWatch()
		now := root.frameTime
		if outTw != nil && exiting {
			outDur := outDurVal
			if outDurPtr != nil {
				outDur = *outDurPtr
			}
			outTarget := readOut()
			if state.exitComplete {
				state.current = outTarget
				*storage = state.current
				t.setExitLease(&state.exitLeaseActive, false)
				return
			}
			if !state.exitActive {
				state.exitActive = true
				state.startVal = state.current
				if from := outTw.getTweenFrom(); from != nil {
					state.startVal = anyToStyle(from)
					state.current = state.startVal
				}
				state.lastTarget = outTarget
				state.startTime = now
			} else if outTarget != state.lastTarget {
				state.startVal = state.current
				state.lastTarget = outTarget
				state.startTime = now
			}
			t.setExitLease(&state.exitLeaseActive, true)
			if state.startTime.IsZero() {
				*storage = state.current
				return
			}
			elapsed := now.Sub(state.startTime)
			if elapsed >= outDur {
				state.current = outTarget
				*storage = state.current
				state.startTime = time.Time{}
				state.exitActive = false
				state.exitComplete = true
				t.setExitLease(&state.exitLeaseActive, false)
				if outOnComplete != nil {
					t.deferComplete(outOnComplete)
				}
				return
			}
			progress := float64(elapsed) / float64(outDur)
			if outEase != nil {
				progress = outEase(progress)
			}
			state.current = lerpStyle(state.startVal, outTarget, progress)
			*storage = state.current
			root.animating = true
			return
		}
		if outTw != nil {
			state.exitActive = false
			state.exitComplete = false
			t.setExitLease(&state.exitLeaseActive, false)
		}
		if state.needsFirstFrame {
			state.startVal = state.current
			state.lastTarget = target
			state.startTime = now
			state.needsFirstFrame = false
		} else if target != state.lastTarget {
			state.startVal = state.current
			state.lastTarget = target
			state.startTime = now
		}
		if state.startTime.IsZero() {
			state.current = target
			*storage = target
			return
		}
		elapsed := now.Sub(state.startTime)
		if elapsed >= dur {
			state.current = target
			*storage = target
			state.startTime = time.Time{}
			if onComplete != nil {
				t.deferComplete(onComplete)
			}
			return
		}
		progress := float64(elapsed) / float64(dur)
		if ease != nil {
			progress = ease(progress)
		}
		state.current = lerpStyle(state.startVal, target, progress)
		*storage = state.current
		root.animating = true
	}

	if inForEach {
		items := make(map[unsafe.Pointer]*perItemStyleState)
		t.itemEvals = append(t.itemEvals, func() {
			key := t.elemBase
			target := readWatch()
			state, ok := items[key]
			if !ok {
				state = &perItemStyleState{lastTarget: target, startVal: target, current: target, needsFirstFrame: hasFrom}
				if hasFrom {
					state.startVal = fromVal
					state.current = fromVal
				}
				items[key] = state
			}
			run(state, t.isExitRenderingFor(key))
		})
	} else {
		state := &perItemStyleState{lastTarget: readWatch(), startVal: *storage, current: *storage, needsFirstFrame: hasFrom}
		if hasFrom {
			state.startVal = fromVal
			state.current = fromVal
		}
		eval := func() {
			run(state, t.exit.rendering)
		}
		root.evals = append(root.evals, eval)
		if outTw != nil {
			t.exitEvals = append(t.exitEvals, eval)
		}
	}

	return storage
}

func (t *Template) resolveTweenTargetColor(target any, elemBase unsafe.Pointer, elemSize uintptr) *Color {
	switch v := target.(type) {
	case *Color:
		return v
	case conditionNode:
		return t.compileCondColor(v, elemBase, elemSize)
	}
	storage := new(Color)
	*storage = anyToColor(target)
	return storage
}

func (t *Template) resolveTweenTargetStyle(target any, elemBase unsafe.Pointer, elemSize uintptr) *Style {
	switch v := target.(type) {
	case *Style:
		return v
	case conditionNode:
		return t.compileCondStyle(v, elemBase, elemSize)
	}
	storage := new(Style)
	*storage = anyToStyle(target)
	return storage
}

// opSparkline holds sparkline-specific data.
type opSparkline struct {
	values    []float64
	valuesPtr sliceBinding // bound *[]float64; offset-resolved per ForEach element
	min       float64
	max       float64
	style     Style
	stylePtr  *Style
}

func (s *opSparkline) resolveValues(elemBase unsafe.Pointer) []float64 {
	if p := s.valuesPtr.ptrFor(elemBase); p != nil {
		return *(*[]float64)(p)
	}
	return s.values
}

func (s *opSparkline) render(t *Template, buf *Buffer, x, y, w, h int16) {
	baseStyle := s.style
	if s.stylePtr != nil {
		baseStyle = *s.stylePtr
	}
	style := t.effectiveStyle(baseStyle)
	vals := s.resolveValues(t.elemBase)
	if len(vals) == 0 {
		return
	}
	if h <= 1 {
		buf.WriteSparkline(int(x), int(y), vals, int(w), s.min, s.max, style)
	} else {
		buf.WriteSparklineMulti(int(x), int(y), vals, int(w), int(h), s.min, s.max, style)
	}
}

func (s *opSparkline) dataLen(elemBase unsafe.Pointer) int {
	if p := s.valuesPtr.ptrFor(elemBase); p != nil {
		return len(*(*[]float64)(p))
	}
	return len(s.values)
}

// text variant modes
const (
	textStatic uint8 = iota
	textPtr
	textOff
	textFn
	textIntPtr
	textIntOff
	textFloat64Ptr
	textFloat64Off
)

type opIf struct {
	condPtr      *bool
	condNode     conditionNode
	thenTmpl     *Template
	elseTmpl     *Template
	branch       branchSelector
	itemBranches map[unsafe.Pointer]*branchSelector
}

func (c *opIf) eval(elemBase unsafe.Pointer) bool {
	return (c.condPtr != nil && *c.condPtr) ||
		(c.condNode != nil && c.condNode.evaluateWithBase(elemBase))
}

func (c *opIf) evalStatic() bool {
	return (c.condPtr != nil && *c.condPtr) ||
		(c.condNode != nil && c.condNode.evaluate())
}

func (c *opIf) selector(elemBase unsafe.Pointer) *branchSelector {
	if elemBase == nil {
		return &c.branch
	}
	if c.itemBranches == nil {
		c.itemBranches = make(map[unsafe.Pointer]*branchSelector)
	}
	selector := c.itemBranches[elemBase]
	if selector == nil {
		selector = &branchSelector{}
		c.itemBranches[elemBase] = selector
	}
	return selector
}

type opForEach struct {
	iterTmpl  *Template
	slice     sliceBinding
	elemSize  uintptr
	elemIsPtr bool   // true when slice elements are pointers (e.g. []*T)
	geoms     []Geom // per-item geometry, reused across frames

	// optional render cap: hasLimit selects between limitStatic and limitPtr
	// (limitPtr supports in-item *int via the same offset trick as slice).
	hasLimit    bool
	limitStatic int
	limitPtr    sliceBinding // points at an int when set
	remaining   sliceBinding // *int writeback: items excluded by the limit
}

// visibleLen clamps the iteration count by the configured limit and writes
// the excluded count to the remaining binding. All iteration sites must use
// this so geometry and render agree. 0 = render none, negative = unlimited.
func (f *opForEach) visibleLen(elemBase unsafe.Pointer, sliceLen int) int {
	n := sliceLen
	if f.hasLimit {
		lim := f.limitStatic
		if p := f.limitPtr.ptrFor(elemBase); p != nil {
			lim = *(*int)(p)
		}
		if lim >= 0 && lim < n {
			n = lim
		}
	}
	if p := f.remaining.ptrFor(elemBase); p != nil {
		*(*int)(p) = sliceLen - n
	}
	return n
}

type sliceBinding struct {
	ptr    unsafe.Pointer
	off    uintptr
	inItem bool
}

func newSliceBinding(ptr, elemBase unsafe.Pointer, elemSize uintptr) sliceBinding {
	b := sliceBinding{ptr: ptr}
	if ptr == nil || elemBase == nil || elemSize == 0 {
		return b
	}
	ptrAddr := uintptr(ptr)
	baseAddr := uintptr(elemBase)
	if ptrAddr >= baseAddr && ptrAddr < baseAddr+elemSize {
		b.off = ptrAddr - baseAddr
		b.inItem = true
	}
	return b
}

func (b sliceBinding) ptrFor(elemBase unsafe.Pointer) unsafe.Pointer {
	if b.inItem && elemBase != nil {
		return unsafe.Pointer(uintptr(elemBase) + b.off)
	}
	return b.ptr
}

func (f *opForEach) sliceHeaderFor(elemBase unsafe.Pointer) (sliceHeader, bool) {
	ptr := f.slice.ptrFor(elemBase)
	if ptr == nil {
		return sliceHeader{}, false
	}
	return *(*sliceHeader)(ptr), true
}

type opSwitch struct {
	node         switchNodeInterface
	cases        []*Template
	def          *Template
	branch       branchSelector
	itemBranches map[unsafe.Pointer]*branchSelector
}

type opMatch struct {
	node         matchNodeInterface
	cases        []*Template
	def          *Template
	branch       branchSelector
	itemBranches map[unsafe.Pointer]*branchSelector
}

func (s *opSwitch) selector(elemBase unsafe.Pointer) *branchSelector {
	if elemBase == nil {
		return &s.branch
	}
	if s.itemBranches == nil {
		s.itemBranches = make(map[unsafe.Pointer]*branchSelector)
	}
	selector := s.itemBranches[elemBase]
	if selector == nil {
		selector = &branchSelector{}
		s.itemBranches[elemBase] = selector
	}
	return selector
}

func (m *opMatch) selector(elemBase unsafe.Pointer) *branchSelector {
	if elemBase == nil {
		return &m.branch
	}
	if m.itemBranches == nil {
		m.itemBranches = make(map[unsafe.Pointer]*branchSelector)
	}
	selector := m.itemBranches[elemBase]
	if selector == nil {
		selector = &branchSelector{}
		m.itemBranches[elemBase] = selector
	}
	return selector
}

type opCustomRenderer struct {
	renderer Renderer
}

type opCustomLayout struct {
	layout LayoutFunc
}

type opText struct {
	mode       uint8
	static     string
	ptr        *string
	intPtr     *int
	float64Ptr *float64
	off        uintptr
	fn         func() string
	fnCached   string // cached result from fn(), set during width measurement
	style      Style
	stylePtr   *Style        // dynamic style override (nil = use static)
	styleCond  conditionNode // conditional style for ForEach (nil = not conditional)
	charWrap   bool          // true = character-wrap, false = word-wrap (TextBlock only)
}

func (tx *opText) resolve(elemBase unsafe.Pointer) string {
	switch tx.mode {
	case textPtr:
		return *tx.ptr
	case textOff:
		return *(*string)(unsafe.Pointer(uintptr(elemBase) + tx.off))
	case textFn:
		return tx.fnCached
	case textIntPtr:
		return strconv.Itoa(*tx.intPtr)
	case textIntOff:
		return strconv.Itoa(*(*int)(unsafe.Pointer(uintptr(elemBase) + tx.off)))
	case textFloat64Ptr:
		return strconv.FormatFloat(*tx.float64Ptr, 'f', -1, 64)
	case textFloat64Off:
		return strconv.FormatFloat(*(*float64)(unsafe.Pointer(uintptr(elemBase) + tx.off)), 'f', -1, 64)
	default:
		return tx.static
	}
}

func (tx *opText) textWidth(elemBase unsafe.Pointer) int16 {
	// use display-cell width so wide runes (emoji, CJK) reserve the right
	// amount of space in layout. rune count was the historical behaviour but
	// underestimates by 1 for each wide rune, which cascades into row overflow.
	switch tx.mode {
	case textPtr:
		return int16(StringWidth(*tx.ptr))
	case textOff:
		if elemBase != nil {
			return int16(StringWidth(*(*string)(unsafe.Pointer(uintptr(elemBase) + tx.off))))
		}
		return 10
	case textFn:
		if tx.fn != nil {
			tx.fnCached = tx.fn()
			return int16(StringWidth(tx.fnCached))
		}
		return 0
	case textIntPtr:
		return int16(len(strconv.Itoa(*tx.intPtr)))
	case textIntOff:
		if elemBase != nil {
			return int16(len(strconv.Itoa(*(*int)(unsafe.Pointer(uintptr(elemBase) + tx.off)))))
		}
		return 1
	case textFloat64Ptr:
		return int16(len(strconv.FormatFloat(*tx.float64Ptr, 'f', -1, 64)))
	case textFloat64Off:
		if elemBase != nil {
			return int16(len(strconv.FormatFloat(*(*float64)(unsafe.Pointer(uintptr(elemBase) + tx.off)), 'f', -1, 64)))
		}
		return 1
	default:
		return int16(StringWidth(tx.static))
	}
}

// progress variant modes
const (
	progStatic uint8 = iota
	progPtr
	progOff
	progInt16Ptr
)

type opProgress struct {
	mode     uint8
	static   int
	ptr      *int
	int16Ptr *int16
	off      uintptr
	style    Style
	stylePtr *Style
}

func (p *opProgress) resolve(elemBase unsafe.Pointer) int {
	switch p.mode {
	case progPtr:
		return *p.ptr
	case progOff:
		return *(*int)(unsafe.Pointer(uintptr(elemBase) + p.off))
	case progInt16Ptr:
		return int(*p.int16Ptr)
	default:
		return p.static
	}
}

// richtext variant modes
const (
	richStatic uint8 = iota
	richPtr
	richOff
)

type opRichText struct {
	mode        uint8
	staticSpans []Span
	spansPtr    *[]Span
	off         uintptr
	spanStrOffs []uintptr
	spanStrPtrs []*string // global *string spans; typed so the GC pins them
	charWrap    bool
	preserveBG  bool
}

func (rt *opRichText) resolve(elemBase unsafe.Pointer) []Span {
	var spans []Span
	switch rt.mode {
	case richPtr:
		spans = *rt.spansPtr
	case richOff:
		if elemBase == nil {
			return nil
		}
		spans = *(*[]Span)(unsafe.Pointer(uintptr(elemBase) + rt.off))
	default:
		spans = rt.staticSpans
	}
	if rt.spanStrOffs != nil {
		return resolveSpanStrs(spans, rt.spanStrOffs, rt.spanStrPtrs, elemBase)
	}
	return spans
}

func styleSpans(spans []Span, styleFor func(Style) Style) []Span {
	if len(spans) == 0 {
		return spans
	}
	styled := make([]Span, len(spans))
	for i, span := range spans {
		styled[i] = Span{Text: span.Text, Style: styleFor(span.Style), OnSelect: span.OnSelect, OnSelectRef: span.OnSelectRef}
	}
	return styled
}

// leader variant modes
const (
	leaderStatic uint8 = iota
	leaderPtr
	leaderIntPtr
	leaderFloatPtr
)

type opLeader struct {
	mode     uint8
	label    string
	value    string
	valuePtr sliceBinding // *string, offset-resolved per ForEach element
	intPtr   sliceBinding // *int
	floatPtr sliceBinding // *float64
	fill     rune
	style    Style
	stylePtr *Style
}

type opCounter struct {
	currentPtr   sliceBinding // *int, offset-resolved per ForEach element
	totalPtr     sliceBinding // *int
	prefix       string
	streamingPtr *bool
	framePtr     *int32
	style        Style
}

type opSpinner struct {
	framePtr sliceBinding // manual *int frame index, offset-resolved per element
	frames   []string
	selfFps  float64 // >0: self-animating from the frame clock
	style    Style
	stylePtr *Style
}

// frameIndex resolves the spinner's current frame. Self-animating spinners
// derive it from the frame clock and mark the template animating — computed
// at render time, so visible means animating and hidden costs nothing.
func (s *opSpinner) frameIndex(t *Template) (int, bool) {
	n := len(s.frames)
	if n == 0 {
		return 0, false
	}
	if s.selfFps > 0 {
		root := t.evalRoot()
		root.animating = true
		return oscStepIndex(root.frameTime.Sub(root.oscEpoch), s.selfFps, n), true
	}
	if p := s.framePtr.ptrFor(t.elemBase); p != nil {
		return *(*int)(p) % n, true
	}
	return 0, false
}

type opRule struct {
	char        rune
	style       Style
	stylePtr    *Style
	extend      bool
	vruleX      int16
	vruleX2     int16
	extendTop   bool
	extendBot   bool
	extendLeft  int16
	extendRight int16
}

type opScrollbar struct {
	contentSize   int
	viewSize      int
	contentPtr    sliceBinding // dynamic content size *int (overrides contentSize when set)
	viewPtr       sliceBinding // dynamic viewport size *int (overrides viewSize when set)
	posPtr        sliceBinding // *int
	layer         *Layer
	horizontal    bool
	trackChar     rune
	thumbChar     rune
	trackStyle    Style
	thumbStyle    Style
	trackStylePtr *Style
	thumbStylePtr *Style
}

type opTabs struct {
	labels        []string
	selectedPtr   sliceBinding // *int, offset-resolved per ForEach element
	styleType     TabsStyle
	gap           int
	activeStyle   Style
	inactiveStyle Style
}

type opTreeView struct {
	root          *TreeNode
	showRoot      bool
	indent        int
	showLines     bool
	expandedChar  rune
	collapsedChar rune
	leafChar      rune
	style         Style
}

type opSelectionList struct {
	opForEach
	listPtr      *selectionList
	selectedPtr  *int
	selectedRef  *NodeRef
	marker       string
	markerWidth  int16
	markerSpaces string
}

type opTextInput struct {
	fieldPtr       *InputState
	focusGroupPtr  *FocusGroup
	focusIndex     int
	valuePtr       *string
	cursorPtr      *int
	focusedPtr     *bool
	syncBound      bool
	lastBoundValue string
	placeholder    string
	mask           rune
	style          Style
	placeholderSty Style
	cursorStyle    Style
	multiline      bool // wrap long text across lines instead of scrolling horizontally
}

// value resolves the input's current text from whichever API is in use.
func (ext *opTextInput) value() string {
	if ext.fieldPtr != nil {
		return ext.fieldPtr.Value
	}
	if ext.valuePtr != nil {
		return *ext.valuePtr
	}
	return ""
}

// inLine is one wrapped display line of a multi-line input: display runes are
// runes[start:end]; next is the start of the following line (skips a dropped break
// space / newline) so a cursor index can be mapped to its line.
type inLine struct{ start, end, next int }

// inputWrapLines word-wraps runes to width: it breaks at the last space that fits,
// hard-breaks words longer than width, and treats '\n' as a forced break. Always
// returns at least one line.
func inputWrapLines(runes []rune, width int) []inLine {
	if width < 1 {
		width = 1
	}
	var lines []inLine
	n := len(runes)
	start, lastSpace := 0, -1
	for i := 0; i < n; {
		switch {
		case runes[i] == '\n':
			lines = append(lines, inLine{start, i, i + 1})
			start, lastSpace = i+1, -1
			i++
		case i-start >= width:
			if lastSpace >= start {
				lines = append(lines, inLine{start, lastSpace, lastSpace + 1})
				start = lastSpace + 1
			} else {
				lines = append(lines, inLine{start, i, i}) // hard break mid-word
				start = i
			}
			lastSpace = -1
		default:
			if runes[i] == ' ' {
				lastSpace = i
			}
			i++
		}
	}
	return append(lines, inLine{start, n, n})
}

// inputCursorPos maps a rune-index cursor to its (line, column) across wrapped lines.
func inputCursorPos(lines []inLine, cursor int) (line, col int) {
	for idx, ln := range lines {
		if cursor < ln.next || idx == len(lines)-1 {
			if col = cursor - ln.start; col < 0 {
				col = 0
			}
			return idx, col
		}
	}
	return 0, 0
}

type opOverlay struct {
	placement    OverlayPlacement
	x, y         int16
	offsetX      *int16
	offsetY      *int16
	backdrop     bool
	backdropFG   Color
	bg           Color
	opacity      *float64
	opacityArmed *bool
	opacityMode  OpacityMode
	childTmpl    *Template
	anchor       *NodeRef
	anchorPos    AnchorPosition
}

type opAutoTable struct {
	slicePtr any
	fields   []int
	headers  []string
	hdrStyle Style
	rowStyle Style
	altStyle *Style
	gap      int8
	fill     Color
	colCfgs  []*ColumnConfig
	sort     *autoTableSortState
	scroll   *autoTableScroll
}

type opLayer struct {
	ptr    *Layer
	width  int16
	height int16
}

type opJump struct {
	onSelect        func()
	onSelectItem    func(unsafe.Pointer)
	onSelectItemRef func(unsafe.Pointer, NodeRef)
	style           Style
}

type opScreenEffect struct {
	fns []Effect
}

// margin helpers (avoid repeating [0]/[1]/[2]/[3] everywhere)
func (op *Op) marginH() int16  { return op.Margin[1] + op.Margin[3] }   // left + right
func (op *Op) marginV() int16  { return op.Margin[0] + op.Margin[2] }   // top + bottom
func (op *Op) paddingH() int16 { return op.Padding[1] + op.Padding[3] } // left + right
func (op *Op) paddingV() int16 { return op.Padding[0] + op.Padding[2] } // top + bottom

// OpKind identifies the type of a compiled template instruction.
type OpKind uint8

const (
	OpText     OpKind = iota // Text (data in Ext)
	OpProgress               // Progress bar (data in Ext)
	OpRichText               // RichText (data in Ext)
	OpLeader                 // Leader dots (data in Ext)
	OpCounter                // Counter (data in Ext)

	OpContainer // VBox or HBox (determined by IsRow)

	OpIf
	OpForEach
	OpSwitch
	OpMatch

	OpCustom // Custom renderer
	OpLayout // Custom layout
	OpLayer  // LayerView (data in Ext)

	OpSelectionList // selectionList (data in Ext)

	OpAutoTable // AutoTable (data in Ext)

	OpSparkline // Sparkline (data in Ext)

	OpHRule        // Horizontal line (data in Ext)
	OpVRule        // Vertical line (data in Ext)
	OpSpacer       // Empty space (data in Ext)
	OpSpinner      // Animated spinner (data in Ext)
	OpScrollbar    // Scroll indicator (data in Ext)
	OpTextBlock    // Multi-line wrapped text (data in Ext)
	OpTabs         // Tab headers (data in Ext)
	OpTreeView     // Hierarchical tree (data in Ext)
	OpJump         // Jump target wrapper (data in Ext)
	OpTextInput    // Single-line text input (data in Ext)
	OpOverlay      // Floating overlay/modal (data in Ext)
	OpScreenEffect // Full-screen post-processing effect (data in Ext)
)

// Build compiles a declarative UI tree into a Template ready for Execute.
// All reflection happens here at compile time; Execute is pure pointer reads.
func Build(ui Component) *Template {
	t := &Template{
		ops:     make([]Op, 0, 32),
		byDepth: make([][]int16, 16),
	}

	for i := range t.byDepth {
		t.byDepth[i] = make([]int16, 0, 8)
	}

	t.compile(ui, -1, 0, nil, 0)

	// Trim unused depths
	for t.maxDepth >= 0 && len(t.byDepth[t.maxDepth]) == 0 {
		t.maxDepth--
	}
	if t.maxDepth >= 0 {
		t.byDepth = t.byDepth[:t.maxDepth+1]
	}

	// Pre-allocate geometry array
	t.geom = make([]Geom, len(t.ops))

	return t
}

// buildWithRoot compiles a child UI tree into a sub-template that shares
// evaluators with this template's root. used by overlays and other sites
// that need an independent template but shared animation/condition state.
func (t *Template) buildWithRoot(ui Component) *Template {
	child := &Template{
		ops:     make([]Op, 0, 32),
		byDepth: make([][]int16, 16),
		root:    t.evalRoot(),
	}
	child.exit.parent = t
	for i := range child.byDepth {
		child.byDepth[i] = make([]int16, 0, 8)
	}
	child.compile(ui, -1, 0, nil, 0)
	for child.maxDepth >= 0 && len(child.byDepth[child.maxDepth]) == 0 {
		child.maxDepth--
	}
	if child.maxDepth >= 0 {
		child.byDepth = child.byDepth[:child.maxDepth+1]
	}
	child.geom = make([]Geom, len(child.ops))
	return child
}

func (t *Template) compileSubTemplate(node any, elemBase unsafe.Pointer, elemSize uintptr) *Template {
	sub := &Template{
		ops:                 make([]Op, 0, 16),
		byDepth:             make([][]int16, 8),
		root:                t.evalRoot(),
		compileElemContexts: append([]elemCompileContext(nil), t.compileElemContexts...),
	}
	if elemBase != nil && elemSize > 0 {
		last := len(sub.compileElemContexts) - 1
		if last < 0 || sub.compileElemContexts[last].base != elemBase || sub.compileElemContexts[last].size != elemSize {
			sub.compileElemContexts = append(sub.compileElemContexts, elemCompileContext{base: elemBase, size: elemSize})
		}
	}
	for i := range sub.byDepth {
		sub.byDepth[i] = make([]int16, 0, 4)
	}
	sub.compile(node, -1, 0, elemBase, elemSize)
	if sub.maxDepth >= 0 {
		sub.byDepth = sub.byDepth[:sub.maxDepth+1]
	}
	sub.geom = make([]Geom, len(sub.ops))
	return sub
}

func (t *Template) addOp(op Op, depth int) int16 {
	idx := int16(len(t.ops))
	op.Depth = int8(depth)
	t.ops = append(t.ops, op)

	// Track by depth for bottom-up traversal
	if depth >= 0 {
		if depth >= len(t.byDepth) {
			for len(t.byDepth) <= depth {
				t.byDepth = append(t.byDepth, make([]int16, 0, 8))
			}
		}
		t.byDepth[depth] = append(t.byDepth[depth], idx)
		if depth > t.maxDepth {
			t.maxDepth = depth
		}
	}

	return idx
}

func (t *Template) compile(node any, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	if node == nil {
		return -1
	}

	switch v := node.(type) {
	case Renderer:
		return t.compileRenderer(v, parent, depth)
	case Box:
		return t.compileBox(v, parent, depth, elemBase, elemSize)
	case conditionNode:
		return t.compileCondition(v, parent, depth, elemBase, elemSize)
	case richTextNode:
		return t.compileRichText(v, parent, depth, elemBase, elemSize)
	case selectionList:
		return t.compileSelectionList(&v, parent, depth, elemBase, elemSize)
	case *selectionList:
		return t.compileSelectionList(v, parent, depth, elemBase, elemSize)
	case TreeView:
		return t.compileTreeView(v, parent, depth)
	case textInput:
		return t.compileTextInput(v, parent, depth)
	case screenEffectNode:
		for i, eff := range v.Effects {
			if ec, ok := eff.(EffectCompilable); ok {
				v.Effects[i] = ec.CompileEffect(effectCompiler{t: t})
			} else if ec, ok := eff.(effectCompilable); ok {
				v.Effects[i] = ec.compileEffect(t)
			}
		}
		ext := &opScreenEffect{fns: v.Effects}
		return t.addOp(Op{Kind: OpScreenEffect, Parent: parent, Ext: ext}, depth)
	case OnC:
		t.collectRouteBindings(v)
		return -1

	case VBoxC:
		return t.compileVBoxC(v, parent, depth, elemBase, elemSize)
	case HBoxC:
		return t.compileHBoxC(v, parent, depth, elemBase, elemSize)
	case TextC:
		return t.compileTextC(v, parent, depth, elemBase, elemSize)
	case TextBlockC:
		return t.compileTextBlockC(v, parent, depth, elemBase, elemSize)
	case SpacerC:
		return t.compileSpacerC(v, parent, depth, elemBase, elemSize)
	case HRuleC:
		return t.compileHRuleC(v, parent, depth, elemBase, elemSize)
	case VRuleC:
		return t.compileVRuleC(v, parent, depth, elemBase, elemSize)
	case ProgressC:
		return t.compileProgressC(v, parent, depth, elemBase, elemSize)
	case SpinnerC:
		return t.compileSpinnerC(v, parent, depth, elemBase, elemSize)
	case LeaderC:
		return t.compileLeaderC(v, parent, depth, elemBase, elemSize)
	case counterC:
		return t.compileCounterC(v, parent, depth, elemBase, elemSize)
	case SparklineC:
		return t.compileSparklineC(v, parent, depth, elemBase, elemSize)
	case JumpC:
		return t.compileJumpC(v, parent, depth, elemBase, elemSize)
	case LayerViewC:
		return t.compileLayerViewC(v, parent, depth, elemBase, elemSize)
	case OverlayC:
		return t.compileOverlayC(v, parent, depth)
	case TabsC:
		return t.compileTabsC(v, parent, depth, elemBase, elemSize)
	case ScrollbarC:
		return t.compileScrollbarC(v, parent, depth, elemBase, elemSize)
	case AutoTableC:
		t.collectBindings(v)
		return t.compileAutoTableC(v, parent, depth)
	case *CheckboxC:
		t.collectBindings(v)
		return t.compileCheckboxC(v, parent, depth, elemBase)
	case *RadioC:
		t.collectBindings(v)
		return t.compileRadioC(v, parent, depth)
	case *InputC:
		t.collectTextInputBinding(v)
		t.collectFocusManager(v)
		return t.compileInputC(v, parent, depth, elemBase, elemSize)
	case *LogC:
		t.collectBindings(v)
		return t.compileLogC(v, parent, depth, elemBase, elemSize)
	case *TextViewC:
		t.collectBindings(v)
		return t.compileTextViewC(v, parent, depth, elemBase, elemSize)
	case *ScrollViewC:
		return t.compileScrollViewC(v, parent, depth)
	case *FilterLogC:
		t.collectFocusManager(v)
		return t.compileFilterLogC(v, parent, depth)
	case customC:
		return t.compileCustom(v, parent, depth)
	}

	// Check for ForEachC[T] via interface
	if fe, ok := node.(forEachCompiler); ok {
		return fe.compileTo(t, parent, depth, elemBase, elemSize)
	}

	// Check for compound components that produce a template subtree
	if tc, ok := node.(templateTree); ok {
		t.collectBindings(node)
		t.collectTextInputBinding(node)
		return t.compile(tc.toTemplate(), parent, depth, elemBase, elemSize)
	}

	// Check for ListC[T] or CheckListC[T] via interface
	// Both implement toSelectionList() which sets up their render functions appropriately
	if lc, ok := node.(listCompiler); ok {
		t.collectBindings(node)
		return t.compileSelectionList(lc.toSelectionList(), parent, depth, elemBase, elemSize)
	}

	// Check for SwitchNodeInterface (generic Switch)
	if sw, ok := node.(switchNodeInterface); ok {
		return t.compileSwitch(sw, parent, depth, elemBase, elemSize)
	}

	// Check for matchNodeInterface (generic Match)
	if mn, ok := node.(matchNodeInterface); ok {
		return t.compileMatch(mn, parent, depth, elemBase, elemSize)
	}

	if c, ok := node.(Component); ok {
		return t.compile(c.Build(), parent, depth, elemBase, elemSize)
	}

	return -1
}

func (t *Template) compileRenderer(r Renderer, parent int16, depth int) int16 {
	return t.addOp(Op{
		Kind:   OpCustom,
		Parent: parent,
		Ext:    &opCustomRenderer{renderer: r},
	}, depth)
}

// customWrapper adapts the Custom struct to the Renderer interface
type customWrapper struct {
	measure func(availW int16) (w, h int16)
	render  func(buf *Buffer, x, y, w, h int16)
}

func (c *customWrapper) Build() Component { return c }

func (c *customWrapper) MinSize() (width, height int) {
	if c.measure == nil {
		return 0, 0
	}
	// Pass -1 to signal "fill available" - widget should return desired minimum
	// or pass back -1 to indicate it wants to fill
	w, h := c.measure(-1)
	if w < 0 {
		w = 0 // will be expanded by parent layout
	}
	return int(w), int(h)
}

// MeasureWithAvail calls measure with actual available width
func (c *customWrapper) MeasureWithAvail(availW int16) (w, h int16) {
	if c.measure == nil {
		return 0, 0
	}
	w, h = c.measure(availW)
	if w < 0 {
		w = availW
	}
	return w, h
}

func (c *customWrapper) Render(buf *Buffer, x, y, w, h int) {
	if c.render != nil {
		c.render(buf, int16(x), int16(y), int16(w), int16(h))
	}
}

func (t *Template) compileCustom(v customC, parent int16, depth int) int16 {
	wrapper := &customWrapper{
		measure: v.Measure,
		render:  v.Render,
	}
	return t.addOp(Op{
		Kind:   OpCustom,
		Parent: parent,
		Ext:    &opCustomRenderer{renderer: wrapper},
	}, depth)
}

func (t *Template) compileBox(box Box, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	// Add layout op first (will fill in ChildStart/ChildEnd)
	idx := t.addOp(Op{
		Kind:       OpLayout,
		Parent:     parent,
		Ext:        &opCustomLayout{layout: box.Layout},
		ChildStart: int16(len(t.ops)),
	}, depth)

	// Compile children
	for _, child := range box.Children {
		t.compile(child, idx, depth+1, elemBase, elemSize)
	}

	// Set child end
	t.ops[idx].ChildEnd = int16(len(t.ops))

	return idx
}

func (t *Template) compileRichText(v richTextNode, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opRichText{charWrap: v.charWrap, preserveBG: v.preserveBG}

	switch spans := v.Spans.(type) {
	case []Span:
		ext.mode = richStatic
		ext.staticSpans = spans
	case *[]Span:
		if elemBase != nil && isWithinRange(unsafe.Pointer(spans), elemBase, elemSize) {
			ext.mode = richOff
			ext.off = uintptr(unsafe.Pointer(spans)) - uintptr(elemBase)
		} else {
			ext.mode = richPtr
			ext.spansPtr = spans
		}
	default:
		ext.mode = richStatic
	}

	// compute per-span *string bindings for Textf: in-item pointers become
	// offsets from elemBase; anything else stays a real *string so the GC
	// pins the allocation (raw uintptr storage trips checkptr and doesn't pin)
	if v.spanPtrs != nil {
		noOffset := ^uintptr(0)
		offs := make([]uintptr, len(v.spanPtrs))
		var ptrs []*string
		for i, ptr := range v.spanPtrs {
			offs[i] = noOffset
			if ptr == nil {
				continue
			}
			if elemBase != nil && isWithinRange(unsafe.Pointer(ptr), elemBase, elemSize) {
				offs[i] = uintptr(unsafe.Pointer(ptr)) - uintptr(elemBase)
			} else {
				if ptrs == nil {
					ptrs = make([]*string, len(v.spanPtrs))
				}
				ptrs[i] = ptr
			}
		}
		ext.spanStrOffs = offs
		ext.spanStrPtrs = ptrs
	}

	return t.addOp(Op{
		Kind:   OpRichText,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

// resolveSpanStrs returns a copy of spans with dynamic *string values re-read.
// In-item spans carry an offset from elemBase in offs[i]; spans bound to
// pointers outside the item carry a typed *string in ptrs[i] (GC-pinned).
// ^uintptr(0) with a nil ptr means that span's text is static.
func resolveSpanStrs(spans []Span, offs []uintptr, ptrs []*string, elemBase unsafe.Pointer) []Span {
	noOffset := ^uintptr(0)
	resolved := make([]Span, len(spans))
	copy(resolved, spans)
	for i, off := range offs {
		if ptrs != nil && ptrs[i] != nil {
			resolved[i].Text = *ptrs[i]
			continue
		}
		if off == noOffset || elemBase == nil {
			continue
		}
		resolved[i].Text = *(*string)(unsafe.Pointer(uintptr(elemBase) + off))
	}
	return resolved
}

func (t *Template) compileSelectionList(v *selectionList, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	// Analyze slice using reflection
	sliceRV := reflect.ValueOf(v.Items)
	if sliceRV.Kind() != reflect.Ptr {
		panic("selectionList Items must be pointer to slice")
	}
	sliceType := sliceRV.Type().Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("selectionList Items must be pointer to slice")
	}
	elemType := sliceType.Elem()
	sliceElemSize := elemType.Size()
	elemIsPtr := elemType.Kind() == reflect.Ptr
	slicePtr := unsafe.Pointer(sliceRV.Pointer())

	// Default marker
	marker := v.Marker
	if marker == "" {
		marker = "> "
	}
	markerWidth := int16(StringWidth(marker))

	// Create iteration template if Render function provided
	var iterTmpl *Template
	if v.Render != nil && !reflect.ValueOf(v.Render).IsNil() {
		renderRV := reflect.ValueOf(v.Render)
		takesPtr := renderRV.Type().In(0).Kind() == reflect.Ptr

		var dummyElem reflect.Value
		var dummyBase unsafe.Pointer
		var compileSize uintptr
		if takesPtr {
			dummyElem = reflect.New(elemType)
			dummyBase = unsafe.Pointer(dummyElem.Pointer())
		} else {
			dummyElem = reflect.New(elemType).Elem()
			dummyBase = unsafe.Pointer(dummyElem.Addr().Pointer())
		}

		if elemIsPtr && takesPtr {
			derefType := elemType.Elem()
			dummy := reflect.New(derefType)
			dummyBase = unsafe.Pointer(dummy.Pointer())
			compileSize = derefType.Size()
			dummyElem.Elem().Set(dummy)
		} else {
			compileSize = sliceElemSize
		}

		// Call render to get template structure
		templateResult := renderRV.Call([]reflect.Value{dummyElem})[0].Interface()

		iterTmpl = t.compileSubTemplate(templateResult, dummyBase, compileSize)
	}

	ext := &opSelectionList{
		listPtr:      v,
		selectedPtr:  v.Selected,
		selectedRef:  v.SelectedRef,
		marker:       marker,
		markerWidth:  markerWidth,
		markerSpaces: strings.Repeat(" ", int(markerWidth)),
	}

	ext.opForEach = opForEach{
		iterTmpl:  iterTmpl,
		slice:     newSliceBinding(slicePtr, elemBase, elemSize),
		elemSize:  sliceElemSize,
		elemIsPtr: elemIsPtr,
	}
	op := Op{
		Kind:   OpSelectionList,
		Parent: parent,
		Margin: v.Style.margin,
		Ext:    ext,
	}

	idx := t.addOp(op, depth)

	// compile dynamic styles — eval writes directly into the Style fields
	if v.StyleDyn != nil {
		ptr := t.compileDynStyle(v.StyleDyn, nil, 0)
		root := t.evalRoot()
		root.evals = append(root.evals, func() { v.Style = *ptr })
	}
	if v.SelectedStyleDyn != nil {
		ptr := t.compileDynStyle(v.SelectedStyleDyn, nil, 0)
		root := t.evalRoot()
		root.evals = append(root.evals, func() { v.SelectedStyle = *ptr })
	}

	return idx
}

func (t *Template) compileTreeView(v TreeView, parent int16, depth int) int16 {
	indent := v.Indent
	if indent == 0 {
		indent = 2
	}
	expandedChar := v.ExpandedChar
	if expandedChar == 0 {
		expandedChar = '▼'
	}
	collapsedChar := v.CollapsedChar
	if collapsedChar == 0 {
		collapsedChar = '▶'
	}
	leafChar := v.LeafChar
	if leafChar == 0 {
		leafChar = ' '
	}
	ext := &opTreeView{
		root:          v.Root,
		showRoot:      v.ShowRoot,
		indent:        indent,
		showLines:     v.ShowLines,
		expandedChar:  expandedChar,
		collapsedChar: collapsedChar,
		leafChar:      leafChar,
		style:         v.Style,
	}
	return t.addOp(Op{
		Kind:   OpTreeView,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileTextInput(v textInput, parent int16, depth int) int16 {
	ext := &opTextInput{
		fieldPtr:       v.Field,
		focusGroupPtr:  v.FocusGroup,
		focusIndex:     v.FocusIndex,
		valuePtr:       v.Value,
		cursorPtr:      v.Cursor,
		focusedPtr:     v.Focused,
		syncBound:      v.SyncBound,
		placeholder:    v.Placeholder,
		mask:           v.Mask,
		style:          v.Style,
		placeholderSty: v.PlaceholderStyle,
		cursorStyle:    v.CursorStyle,
		multiline:      v.MultiLine,
	}

	if ext.placeholderSty.Equal(Style{}) {
		ext.placeholderSty = Style{Attr: AttrDim}
	}
	if ext.cursorStyle.Equal(Style{}) {
		ext.cursorStyle = Style{Attr: AttrInverse}
	}

	return t.addOp(Op{
		Kind:   OpTextInput,
		Parent: parent,
		Width:  int16(v.Width),
		Margin: v.Style.margin,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileContainer(children []Component, gap int8, isRow bool, f flex, border BorderStyle, title string, borderFG, borderBG *Color, fill Color, inheritStyle *Style, margin [4]int16, padding [4]int16, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	op := Op{
		Kind:         OpContainer,
		Parent:       parent,
		IsRow:        isRow,
		Gap:          gap,
		PercentWidth: f.percentWidth,
		Width:        f.width,
		Height:       f.height,
		FlexGrow:     f.flexGrow,
		FitContent:   f.fitContent,
		Border:       border,
		Title:        title,
		BorderFG:     borderFG,
		BorderBG:     borderBG,
		Fill:         fill,
		CascadeStyle: inheritStyle,
		Margin:       margin,
		Padding:      padding,
		OpacityMode:  OpacitySmooth,
	}

	if f.widthPtr != nil || f.heightPtr != nil || f.flexGrowPtr != nil || f.percentWidthPtr != nil {
		op.Dyn = &OpDyn{
			Width:        f.widthPtr,
			Height:       f.heightPtr,
			FlexGrow:     f.flexGrowPtr,
			PercentWidth: f.percentWidthPtr,
		}
	}

	idx := t.addOp(op, depth)

	// Track child range
	childStart := int16(len(t.ops))
	for _, child := range children {
		t.compile(child, idx, depth+1, elemBase, elemSize)
	}
	childEnd := int16(len(t.ops))

	// Update op with child range
	t.ops[idx].ChildStart = childStart
	t.ops[idx].ChildEnd = childEnd

	// Bubble finite intrinsic width up for row layout decisions. A nested HBox
	// made only from finite children (for example Text + Text) should not
	// become implicit flex just because it is a container. VBox still defaults
	// to filling the finite width it is given unless FitContent is explicit.
	if isRow {
		hasFiniteChild := false
		hasFlexibleChild := false
		for i := childStart; i < childEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			if t.opHasFiniteIntrinsicWidth(childOp) {
				hasFiniteChild = true
			} else {
				hasFlexibleChild = true
			}
		}
		if hasFiniteChild && !hasFlexibleChild {
			t.ops[idx].ContentSized = true
		}
	} else {
		for i := childStart; i < childEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			if t.opBubblesFiniteWidth(childOp) {
				t.ops[idx].ContentSized = true
				break
			}
		}
	}

	return idx
}

func (t *Template) opBubblesFiniteWidth(op *Op) bool {
	return op.width() > 0 || (op.Dyn != nil && op.Dyn.Width != nil) || op.FitContent || op.ContentSized
}

func (t *Template) opHasFiniteIntrinsicWidth(op *Op) bool {
	if t.opBubblesFiniteWidth(op) {
		return true
	}
	switch op.Kind {
	case OpText, OpCounter, OpVRule, OpSpinner, OpTabs, OpTreeView:
		return true
	case OpSpacer:
		return op.width() > 0
	}
	return false
}

func (t *Template) compileCondition(cond conditionNode, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	// Check if condition pointer is within element range (ForEach context)
	if elemBase != nil && elemSize > 0 {
		ptrAddr := cond.getPtrAddr()
		baseAddr := uintptr(elemBase)
		if ptrAddr >= baseAddr && ptrAddr < baseAddr+elemSize {
			// Set offset for rebinding during render
			cond.setOffset(ptrAddr - baseAddr)
		}
	}

	ext := &opIf{
		condNode: cond,
	}

	// Compile then branch as sub-template
	if cond.getThen() != nil {
		thenTmpl := t.compileSubTemplate(cond.getThen(), elemBase, elemSize)
		ext.thenTmpl = thenTmpl
		t.pendingBindings = append(t.pendingBindings, thenTmpl.pendingBindings...)
	}

	// Compile else branch if present
	if cond.getElse() != nil {
		elseTmpl := t.compileSubTemplate(cond.getElse(), elemBase, elemSize)
		ext.elseTmpl = elseTmpl
		t.pendingBindings = append(t.pendingBindings, elseTmpl.pendingBindings...)
	}

	return t.addOp(Op{
		Kind:   OpIf,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileSwitch(sw switchNodeInterface, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	// record offset from element base so getMatchIndexWithBase works inside ForEach
	if elemBase != nil && elemSize > 0 {
		ptrAddr := sw.getPtrAddr()
		baseAddr := uintptr(elemBase)
		if ptrAddr >= baseAddr && ptrAddr < baseAddr+elemSize {
			sw.setPtrOffset(ptrAddr - baseAddr)
		}
	}

	ext := &opSwitch{
		node: sw,
	}

	// Compile each case branch
	caseNodes := sw.getCaseNodes()
	ext.cases = make([]*Template, len(caseNodes))
	for i, caseNode := range caseNodes {
		if caseNode != nil {
			caseTmpl := t.compileSubTemplate(caseNode, elemBase, elemSize)
			ext.cases[i] = caseTmpl
			t.pendingBindings = append(t.pendingBindings, caseTmpl.pendingBindings...)
		}
	}

	// Compile default branch
	if defNode := sw.getDefaultNode(); defNode != nil {
		defTmpl := t.compileSubTemplate(defNode, elemBase, elemSize)
		ext.def = defTmpl
		t.pendingBindings = append(t.pendingBindings, defTmpl.pendingBindings...)
	}

	return t.addOp(Op{
		Kind:   OpSwitch,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileMatch(mn matchNodeInterface, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	if elemBase != nil && elemSize > 0 {
		ptrAddr := mn.getPtrAddr()
		baseAddr := uintptr(elemBase)
		if ptrAddr >= baseAddr && ptrAddr < baseAddr+elemSize {
			mn.setPtrOffset(ptrAddr - baseAddr)
		}
	}

	ext := &opMatch{node: mn}

	caseNodes := mn.getCaseNodes()
	ext.cases = make([]*Template, len(caseNodes))
	for i, caseNode := range caseNodes {
		if caseNode != nil {
			caseTmpl := t.compileSubTemplate(caseNode, elemBase, elemSize)
			ext.cases[i] = caseTmpl
			t.pendingBindings = append(t.pendingBindings, caseTmpl.pendingBindings...)
		}
	}

	if defNode := mn.getDefaultNode(); defNode != nil {
		defTmpl := t.compileSubTemplate(defNode, elemBase, elemSize)
		ext.def = defTmpl
		t.pendingBindings = append(t.pendingBindings, defTmpl.pendingBindings...)
	}

	return t.addOp(Op{
		Kind:   OpMatch,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileForEach(items any, render any, limit any, remaining *int, parent int16, depth int, elemBase unsafe.Pointer, parentElemSize uintptr) int16 {
	// Analyze slice
	sliceRV := reflect.ValueOf(items)
	if sliceRV.Kind() != reflect.Ptr {
		panic("ForEach Items must be pointer to slice")
	}
	sliceType := sliceRV.Type().Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("ForEach Items must be pointer to slice")
	}
	elemType := sliceType.Elem()
	elemSize := elemType.Size()
	elemIsPtr := elemType.Kind() == reflect.Ptr
	slicePtr := unsafe.Pointer(sliceRV.Pointer())

	// Create dummy element for template compilation
	renderRV := reflect.ValueOf(render)
	takesPtr := renderRV.Type().In(0).Kind() == reflect.Ptr

	var dummyElem reflect.Value
	var dummyBase unsafe.Pointer
	var compileSize uintptr
	if takesPtr {
		dummyElem = reflect.New(elemType)
		dummyBase = unsafe.Pointer(dummyElem.Pointer())
	} else {
		dummyElem = reflect.New(elemType).Elem()
		dummyBase = unsafe.Pointer(dummyElem.Addr().Pointer())
	}

	// when elements are pointers, the render callback dereferences them
	// (e.g. func(pp **T) { fn(*pp) }). compile against the pointed-to
	// struct so offset calculations work for fields within the struct.
	if elemIsPtr && takesPtr {
		derefType := elemType.Elem()
		dummy := reflect.New(derefType)
		dummyBase = unsafe.Pointer(dummy.Pointer())
		compileSize = derefType.Size()
		dummyElem.Elem().Set(dummy)
	} else {
		compileSize = elemSize
	}

	// Call render to get template structure
	templateResult := renderRV.Call([]reflect.Value{dummyElem})[0].Interface()

	iterTmpl := t.compileSubTemplate(templateResult, dummyBase, compileSize)

	ext := &opForEach{
		iterTmpl:  iterTmpl,
		slice:     newSliceBinding(slicePtr, elemBase, parentElemSize),
		elemSize:  elemSize,
		elemIsPtr: elemIsPtr,
	}
	switch lim := limit.(type) {
	case nil:
	case int:
		ext.hasLimit = true
		ext.limitStatic = lim
	case *int:
		ext.hasLimit = true
		ext.limitPtr = newSliceBinding(unsafe.Pointer(lim), elemBase, parentElemSize)
	default:
		panic("ForEach.Limit: accepts int or *int")
	}
	if remaining != nil {
		ext.remaining = newSliceBinding(unsafe.Pointer(remaining), elemBase, parentElemSize)
	}

	op := Op{
		Kind:   OpForEach,
		Parent: parent,
		Ext:    ext,
	}

	return t.addOp(op, depth)
}

// ============================================================================
// Compile functions for new functional API types
// ============================================================================

func (t *Template) ensureOpDyn(idx int16) *OpDyn {
	if t.ops[idx].Dyn == nil {
		t.ops[idx].Dyn = &OpDyn{}
	}
	return t.ops[idx].Dyn
}

func (t *Template) compileContainerFlex(percentWidth float32, width, height int16, flexGrow float32, fitContent bool, widthPtr, heightPtr *int16, percentWidthPtr, flexGrowPtr *float32, widthCond, heightCond, percentWidthCond, flexGrowCond any, elemBase unsafe.Pointer, elemSize uintptr) flex {
	f := flex{
		percentWidth: percentWidth,
		width:        width,
		height:       height,
		flexGrow:     flexGrow,
		fitContent:   fitContent,
		// raw item-field pointers rebind per ForEach element (frozen otherwise)
		widthPtr:        t.compileDynInt16(widthPtr, elemBase, elemSize),
		heightPtr:       t.compileDynInt16(heightPtr, elemBase, elemSize),
		percentWidthPtr: t.compileDynFloat32(percentWidthPtr, elemBase, elemSize),
		flexGrowPtr:     t.compileDynFloat32(flexGrowPtr, elemBase, elemSize),
	}
	if heightCond != nil {
		f.heightPtr = t.compileDynInt16(heightCond, elemBase, elemSize)
	}
	if widthCond != nil {
		f.widthPtr = t.compileDynInt16(widthCond, elemBase, elemSize)
	}
	if percentWidthCond != nil {
		f.percentWidthPtr = t.compileDynFloat32(percentWidthCond, elemBase, elemSize)
	}
	if flexGrowCond != nil {
		f.flexGrowPtr = t.compileDynFloat32(flexGrowCond, elemBase, elemSize)
	}
	return f
}

func (t *Template) applyContainerDynamics(idx int16, nodeRef *NodeRef, opacityMode OpacityMode, gapPtr *int8, gapCond any, fillPtr *Color, fillCond any, localStyle, localStylePtr *Style, localStyleCond any, opacity dynFloat64, elemBase unsafe.Pointer, elemSize uintptr) {
	if nodeRef != nil {
		t.ops[idx].NodeRef = nodeRef
	}
	t.ops[idx].OpacityMode = opacityMode
	if gapPtr != nil {
		t.ensureOpDyn(idx).Gap = gapPtr
	}
	if gapCond != nil {
		t.ensureOpDyn(idx).Gap = t.compileDynInt8(gapCond, elemBase, elemSize)
	}
	if fillCond != nil {
		t.ensureOpDyn(idx).Fill = t.compileDynColor(fillCond, elemBase, elemSize)
	} else if fillPtr != nil {
		if elemBase != nil && isWithinRange(unsafe.Pointer(fillPtr), elemBase, elemSize) {
			dyn := t.ensureOpDyn(idx)
			dyn.FillOff = uintptr(unsafe.Pointer(fillPtr)) - uintptr(elemBase)
			dyn.FillIsOff = true
		} else {
			t.ensureOpDyn(idx).Fill = fillPtr
		}
	}
	if localStyleCond != nil {
		t.ops[idx].LocalStyle = t.compileDynStyle(localStyleCond, nil, 0)
	} else if localStylePtr != nil {
		t.ops[idx].LocalStyle = localStylePtr
	} else if localStyle != nil {
		t.ops[idx].LocalStyle = localStyle
	}
	if opacity.dyn != nil {
		dyn := t.ensureOpDyn(idx)
		if ptr, ok := opacity.dyn.(*float64); ok && elemBase != nil && isWithinRange(unsafe.Pointer(ptr), elemBase, elemSize) {
			dyn.OpacityOff = uintptr(unsafe.Pointer(ptr)) - uintptr(elemBase)
			dyn.OpacityIsOff = true
		} else {
			opacity.compileArmed(t, elemBase, elemSize)
			dyn.Opacity = opacity.ptr
			dyn.OpacityArmed = opacity.armed
		}
	} else if opacity.isSet {
		val := opacity.val
		t.ensureOpDyn(idx).Opacity = &val
	}
}

func (t *Template) compileVBoxC(v VBoxC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	f := t.compileContainerFlex(v.percentWidth, v.width, v.height, v.flexGrow, v.fitContent, v.widthPtr, v.heightPtr, v.percentWidthPtr, v.flexGrowPtr, v.widthCond, v.heightCond, v.percentWidthCond, v.flexGrowCond, elemBase, elemSize)
	bfg := v.borderFG
	if v.borderFGDyn != nil {
		bfg = t.compileDynColor(v.borderFGDyn, elemBase, elemSize)
	}
	idx := t.compileContainer(
		v.children,
		v.gap,
		false, // isRow
		f,
		v.border,
		v.title,
		bfg,
		v.borderBG,
		v.fill,
		v.inheritStyle,
		v.margin,
		v.padding,
		parent,
		depth,
		elemBase,
		elemSize,
	)
	t.ops[idx].MaxWidth = v.maxWidth
	t.ops[idx].MaxWidthPct = v.maxWidthPct
	t.applyContainerDynamics(idx, v.nodeRef, v.opacityMode, v.gapPtr, v.gapCond, v.fillPtr, v.fillCond, v.localStyle, v.localStylePtr, v.localStyleCond, v.opacity, elemBase, elemSize)
	return idx
}

func (t *Template) compileHBoxC(v HBoxC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	f := t.compileContainerFlex(v.percentWidth, v.width, v.height, v.flexGrow, v.fitContent, v.widthPtr, v.heightPtr, v.percentWidthPtr, v.flexGrowPtr, v.widthCond, v.heightCond, v.percentWidthCond, v.flexGrowCond, elemBase, elemSize)
	bfg := v.borderFG
	if v.borderFGDyn != nil {
		bfg = t.compileDynColor(v.borderFGDyn, elemBase, elemSize)
	}
	idx := t.compileContainer(
		v.children,
		v.gap,
		true, // isRow
		f,
		v.border,
		v.title,
		bfg,
		v.borderBG,
		v.fill,
		v.inheritStyle,
		v.margin,
		v.padding,
		parent,
		depth,
		elemBase,
		elemSize,
	)
	t.ops[idx].MaxWidth = v.maxWidth
	t.ops[idx].MaxWidthPct = v.maxWidthPct
	t.applyContainerDynamics(idx, v.nodeRef, v.opacityMode, v.gapPtr, v.gapCond, v.fillPtr, v.fillCond, v.localStyle, v.localStylePtr, v.localStyleCond, v.opacity, elemBase, elemSize)
	return idx
}

func (t *Template) compileTextC(v TextC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opText{style: v.style}

	switch val := v.content.(type) {
	case string:
		ext.mode = textStatic
		ext.static = val
	case *string:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			ext.mode = textOff
			ext.off = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			ext.mode = textPtr
			ext.ptr = val
		}
	case **string:
		if val == nil || *val == nil {
			ext.mode = textStatic
			ext.static = ""
		} else if elemBase != nil && isWithinRange(unsafe.Pointer(*val), elemBase, elemSize) {
			ext.mode = textOff
			ext.off = uintptr(unsafe.Pointer(*val)) - uintptr(elemBase)
		} else {
			ext.mode = textPtr
			ext.ptr = *val
		}
	case func() string:
		ext.mode = textFn
		ext.fn = val
	case *int:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			ext.mode = textIntOff
			ext.off = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			ext.mode = textIntPtr
			ext.intPtr = val
		}
	case *float64:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			ext.mode = textFloat64Off
			ext.off = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			ext.mode = textFloat64Ptr
			ext.float64Ptr = val
		}
	}

	// compile dynamic style: whole style > individual FG/BG
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)

	idx := t.addOp(Op{
		Kind:        OpText,
		Parent:      parent,
		Width:       v.width,
		Margin:      v.style.margin,
		OpacityMode: v.opacityMode,
		Ext:         ext,
	}, depth)
	if v.widthCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, elemBase, elemSize)
	} else if v.widthPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
	}
	if v.opacity.dyn != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		if ptr, ok := v.opacity.dyn.(*float64); ok && elemBase != nil && isWithinRange(unsafe.Pointer(ptr), elemBase, elemSize) {
			t.ops[idx].Dyn.OpacityOff = uintptr(unsafe.Pointer(ptr)) - uintptr(elemBase)
			t.ops[idx].Dyn.OpacityIsOff = true
		} else {
			v.opacity.compileArmed(t, elemBase, elemSize)
			t.ops[idx].Dyn.Opacity = v.opacity.ptr
			t.ops[idx].Dyn.OpacityArmed = v.opacity.armed
		}
	} else if v.opacity.isSet {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		val := v.opacity.val
		t.ops[idx].Dyn.Opacity = &val
	}
	return idx
}

func (t *Template) compileTextBlockC(v TextBlockC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opText{style: v.style, charWrap: v.charWrap}

	switch val := v.content.(type) {
	case string:
		ext.mode = textStatic
		ext.static = val
	case *string:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			ext.mode = textOff
			ext.off = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			ext.mode = textPtr
			ext.ptr = val
		}
	case func() string:
		ext.mode = textFn
		ext.fn = val
	}

	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)

	return t.addOp(Op{
		Kind:   OpTextBlock,
		Parent: parent,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileSpacerC(v SpacerC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	grow := v.flexGrow
	if grow == 0 && v.width == 0 && v.height == 0 && v.widthPtr == nil && v.heightPtr == nil && v.flexGrowPtr == nil && v.widthCond == nil && v.heightCond == nil && v.flexGrowCond == nil {
		grow = 1
	}
	ext := &opRule{char: v.char, style: v.style}
	idx := t.addOp(Op{
		Kind:     OpSpacer,
		Parent:   parent,
		Width:    v.width,
		Height:   v.height,
		FlexGrow: grow,
		Margin:   v.style.margin,
		Ext:      ext,
	}, depth)
	hasDyn := v.widthPtr != nil || v.heightPtr != nil || v.flexGrowPtr != nil || v.widthCond != nil || v.heightCond != nil || v.flexGrowCond != nil
	if hasDyn {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		if v.widthCond != nil {
			t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, elemBase, elemSize)
		} else if v.widthPtr != nil {
			t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
		}
		if v.heightCond != nil {
			t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightCond, elemBase, elemSize)
		} else if v.heightPtr != nil {
			t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightPtr, elemBase, elemSize)
		}
		if v.flexGrowCond != nil {
			t.ops[idx].Dyn.FlexGrow = t.compileDynFloat32(v.flexGrowCond, elemBase, elemSize)
		} else if v.flexGrowPtr != nil {
			t.ops[idx].Dyn.FlexGrow = t.compileDynFloat32(v.flexGrowPtr, elemBase, elemSize)
		}
	}
	return idx
}

func (t *Template) compileHRuleC(v HRuleC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	char := v.char
	if char == 0 {
		char = '─'
	}
	ext := &opRule{char: char, style: v.style, extend: v.extend}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)
	return t.addOp(Op{
		Kind:   OpHRule,
		Parent: parent,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileVRuleC(v VRuleC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	char := v.char
	if char == 0 {
		char = '│'
	}
	ext := &opRule{char: char, style: v.style, extend: v.extend}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)
	idx := t.addOp(Op{
		Kind:   OpVRule,
		Parent: parent,
		Height: v.height,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
	if v.heightCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightCond, nil, 0)
	} else if v.heightPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightPtr, elemBase, elemSize)
	}
	return idx
}

func (t *Template) compileProgressC(v ProgressC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	width := v.width
	if width == 0 {
		width = 20
	}

	ext := &opProgress{style: v.style}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)

	switch val := v.value.(type) {
	case int:
		ext.mode = progStatic
		ext.static = val
	case *int:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			ext.mode = progOff
			ext.off = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			ext.mode = progPtr
			ext.ptr = val
		}
	case tweenNode:
		ext.mode = progInt16Ptr
		ext.int16Ptr = t.compileTweenInt16(val, elemBase, elemSize)
	}

	idx := t.addOp(Op{
		Kind:   OpProgress,
		Parent: parent,
		Width:  width,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
	if v.widthCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, elemBase, elemSize)
	} else if v.widthPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
	}
	return idx
}

func (t *Template) compileSpinnerC(v SpinnerC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	frames := v.frames
	if frames == nil {
		frames = SpinnerBraille
	}
	selfFps := 0.0
	if v.frame == nil {
		selfFps = v.fps
		if selfFps <= 0 {
			selfFps = 12
		}
	}
	ext := &opSpinner{framePtr: newSliceBinding(unsafe.Pointer(v.frame), elemBase, elemSize), frames: frames, selfFps: selfFps, style: v.style}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)
	return t.addOp(Op{
		Kind:   OpSpinner,
		Parent: parent,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileLeaderC(v LeaderC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	fill := v.fill
	if fill == 0 {
		fill = '.'
	}

	ext := &opLeader{fill: fill, style: v.style}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)

	switch label := v.label.(type) {
	case string:
		ext.label = label
	case *string:
		ext.label = *label
	}

	switch val := v.value.(type) {
	case string:
		ext.mode = leaderStatic
		ext.value = val
	case *string:
		ext.mode = leaderPtr
		ext.valuePtr = newSliceBinding(unsafe.Pointer(val), elemBase, elemSize)
	case *int:
		ext.mode = leaderIntPtr
		ext.intPtr = newSliceBinding(unsafe.Pointer(val), elemBase, elemSize)
	case *float64:
		ext.mode = leaderFloatPtr
		ext.floatPtr = newSliceBinding(unsafe.Pointer(val), elemBase, elemSize)
	case int:
		ext.mode = leaderStatic
		ext.value = fmt.Sprintf("%d", val)
	case float64:
		ext.mode = leaderStatic
		ext.value = fmt.Sprintf("%.1f", val)
	default:
		ext.mode = leaderStatic
		ext.value = fmt.Sprintf("%v", val)
	}

	idx := t.addOp(Op{
		Kind:   OpLeader,
		Parent: parent,
		Width:  v.width,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
	if v.widthCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, nil, 0)
	} else if v.widthPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
	}
	return idx
}

func (t *Template) compileCounterC(v counterC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opCounter{
		currentPtr:   newSliceBinding(unsafe.Pointer(v.current), elemBase, elemSize),
		totalPtr:     newSliceBinding(unsafe.Pointer(v.total), elemBase, elemSize),
		prefix:       v.prefix,
		streamingPtr: v.streaming,
		framePtr:     v.framePtr,
		style:        v.style,
	}
	return t.addOp(Op{
		Kind:   OpCounter,
		Parent: parent,
		Margin: v.style.margin,
		Ext:    ext,
	}, depth)
}

func (t *Template) compileSparklineC(v SparklineC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opSparkline{min: v.min, max: v.max, style: v.style}
	ext.stylePtr = t.compileStyleDyn(v.style, v.styleDyn, v.fgDyn, v.bgDyn, elemBase, elemSize)
	switch vals := v.values.(type) {
	case []float64:
		ext.values = vals
	case *[]float64:
		ext.valuesPtr = newSliceBinding(unsafe.Pointer(vals), elemBase, elemSize)
	}

	op := Op{
		Kind:   OpSparkline,
		Parent: parent,
		Width:  v.width,
		Height: v.height,
		Margin: v.style.margin,
		Ext:    ext,
	}
	idx := t.addOp(op, depth)
	hasDyn := v.widthPtr != nil || v.heightPtr != nil || v.widthCond != nil || v.heightCond != nil
	if hasDyn {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		if v.widthCond != nil {
			t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, nil, 0)
		} else if v.widthPtr != nil {
			t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
		}
		if v.heightCond != nil {
			t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightCond, nil, 0)
		} else if v.heightPtr != nil {
			t.ops[idx].Dyn.Height = t.compileDynInt16(v.heightPtr, elemBase, elemSize)
		}
	}
	return idx
}

func (t *Template) compileJumpC(v JumpC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opJump{onSelect: v.onSelect, onSelectItem: v.onSelectItem, onSelectItemRef: v.onSelectItemRef, style: v.style}
	idx := t.addOp(Op{
		Kind:       OpJump,
		Parent:     parent,
		ChildStart: int16(len(t.ops)),
		Margin:     v.margin,
		Padding:    v.padding,
		Ext:        ext,
	}, depth)

	if v.child != nil {
		t.compile(v.child, idx, depth+1, elemBase, elemSize)
	}

	t.ops[idx].ChildEnd = int16(len(t.ops))
	return idx
}

func (t *Template) compileLayerViewC(v LayerViewC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opLayer{ptr: v.layer, width: v.viewWidth, height: v.viewHeight}
	idx := t.addOp(Op{
		Kind:     OpLayer,
		Parent:   parent,
		FlexGrow: v.flexGrow,
		Margin:   v.margin,
		Padding:  v.padding,
		Ext:      ext,
	}, depth)
	if v.viewHeightPtr != nil {
		t.ensureOpDyn(idx).Height = v.viewHeightPtr
	}
	if v.flexGrowCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.FlexGrow = t.compileDynFloat32(v.flexGrowCond, nil, 0)
	} else if v.flexGrowPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.FlexGrow = t.compileDynFloat32(v.flexGrowPtr, elemBase, elemSize)
	}
	return idx
}

func (t *Template) compileOverlayC(v OverlayC, parent int16, depth int) int16 {
	var childTmpl *Template
	if len(v.children) == 1 {
		childTmpl = t.buildWithRoot(v.children[0])
	} else if len(v.children) > 1 {
		childTmpl = t.buildWithRoot(VBox(v.children...))
	}

	placement := v.placement
	if !v.placementSet && v.anchor == nil {
		placement = OverlayPlacementCentered
	}

	backdropFG := v.backdropFG
	if backdropFG.Mode == ColorDefault && v.backdrop {
		backdropFG = BrightBlack
	}

	ext := &opOverlay{
		placement:   placement,
		x:           int16(v.x),
		y:           int16(v.y),
		offsetX:     t.compileOverlayOffset(v.offsetX),
		offsetY:     t.compileOverlayOffset(v.offsetY),
		backdrop:    v.backdrop,
		backdropFG:  backdropFG,
		bg:          v.bg,
		opacityMode: v.opacityMode,
		childTmpl:   childTmpl,
		anchor:      v.anchor,
		anchorPos:   v.anchorPos,
	}
	if v.opacity.dyn != nil {
		v.opacity.compileArmed(t, nil, 0)
		ext.opacity = v.opacity.ptr
		ext.opacityArmed = v.opacity.armed
	} else if v.opacity.isSet {
		val := v.opacity.val
		ext.opacity = &val
	}

	return t.addOp(Op{
		Kind:   OpOverlay,
		Parent: parent,
		Width:  int16(v.width),
		Height: int16(v.height),
		Ext:    ext,
	}, depth)
}

func (t *Template) compileOverlayOffset(v any) *int16 {
	switch val := v.(type) {
	case nil:
		return nil
	case int16:
		return &val
	case int:
		n := int16(val)
		return &n
	case *int16:
		return val
	case conditionNode:
		return t.compileDynInt16(val, nil, 0)
	case tweenNode:
		return t.compileDynInt16(val, nil, 0)
	}
	return nil
}

func (t *Template) compileTabsC(v TabsC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	ext := &opTabs{
		labels:        v.labels,
		selectedPtr:   newSliceBinding(unsafe.Pointer(v.selected), elemBase, elemSize),
		styleType:     v.tabStyle,
		gap:           int(v.gap),
		activeStyle:   v.activeStyle,
		inactiveStyle: v.inactiveStyle,
	}
	idx := t.addOp(Op{
		Kind:   OpTabs,
		Parent: parent,
		Gap:    v.gap,
		Margin: v.margin,
		Ext:    ext,
	}, depth)
	if v.gapCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Gap = t.compileDynInt8(v.gapCond, nil, 0)
	} else if v.gapPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Gap = v.gapPtr
	}
	return idx
}

func (t *Template) compileScrollbarC(v ScrollbarC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	trackChar := v.trackChar
	thumbChar := v.thumbChar
	if trackChar == 0 {
		if v.horizontal {
			trackChar = '─'
		} else {
			trackChar = '│'
		}
	}
	if thumbChar == 0 {
		thumbChar = '█'
	}
	ext := &opScrollbar{
		contentSize:   v.contentSize,
		viewSize:      v.viewSize,
		contentPtr:    newSliceBinding(unsafe.Pointer(v.contentPtr), elemBase, elemSize),
		viewPtr:       newSliceBinding(unsafe.Pointer(v.viewPtr), elemBase, elemSize),
		posPtr:        newSliceBinding(unsafe.Pointer(v.position), elemBase, elemSize),
		layer:         v.layer,
		horizontal:    v.horizontal,
		trackChar:     trackChar,
		thumbChar:     thumbChar,
		trackStyle:    v.trackStyle,
		thumbStyle:    v.thumbStyle,
		trackStylePtr: t.compileStyleDyn(v.trackStyle, v.trackStyleDyn, nil, nil, nil, 0),
		thumbStylePtr: t.compileStyleDyn(v.thumbStyle, v.thumbStyleDyn, nil, nil, nil, 0),
	}
	width, height := v.length, v.length
	if v.horizontal {
		if height == 0 {
			height = 1
		}
	} else {
		width = 1
	}
	idx := t.addOp(Op{
		Kind:        OpScrollbar,
		Parent:      parent,
		Width:       width,
		Height:      height,
		OpacityMode: v.opacityMode,
		Margin:      v.margin,
		Ext:         ext,
	}, depth)
	if v.opacity.dyn != nil {
		t.ops[idx].Dyn = &OpDyn{}
		t.ops[idx].Dyn.Opacity = t.compileDynFloat64(v.opacity.dyn, nil, 0)
	} else if v.opacity.isSet {
		t.ops[idx].Dyn = &OpDyn{}
		val := v.opacity.val
		t.ops[idx].Dyn.Opacity = &val
	}
	return idx
}

func (t *Template) compileAutoTableC(v AutoTableC, parent int16, depth int) int16 {
	rv := reflect.ValueOf(v.data)

	// pointer to slice -> reactive mode (reads data each frame)
	if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Slice {
		return t.compileAutoTableReactive(v, rv, parent, depth)
	}

	if rv.Kind() != reflect.Slice {
		return t.compileTextC(Text("AutoTable: expected slice or *slice"), parent, depth, nil, 0)
	}

	// static slice -> snapshot mode (existing behaviour)
	return t.compileAutoTableStatic(v, rv, parent, depth)
}

// compileAutoTableReactive compiles an AutoTable backed by *[]T into a single
// OpAutoTable that reads through the pointer on every render frame.
func (t *Template) compileAutoTableReactive(v AutoTableC, rv reflect.Value, parent int16, depth int) int16 {
	sliceType := rv.Elem().Type() // []T
	elemType := sliceType.Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return t.compileTextC(Text("AutoTable: expected *[]struct"), parent, depth, nil, 0)
	}

	columns, fieldIndices := autoTableResolveColumns(v.columns, elemType)
	if len(columns) == 0 {
		return t.compileTextC(Text("AutoTable: no columns"), parent, depth, nil, 0)
	}

	headers := v.headers
	if len(headers) == 0 {
		headers = make([]string, len(columns))
		copy(headers, columns)
	}

	// resolve per-column configs
	colCfgs := make([]*ColumnConfig, len(columns))
	for i, name := range columns {
		cfg := &ColumnConfig{}
		fi := fieldIndices[i]

		// apply type-based default alignment
		ft := elemType.Field(fi).Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		cfg.align = autoTableDefaultAlign(ft)

		// apply user config on top (overrides type defaults)
		if opt, ok := v.columnConfigs[name]; ok {
			opt(cfg)
		}

		colCfgs[i] = cfg
	}

	var altFill Color
	if v.altRowStyle != nil && v.altRowStyle.BG.Mode != ColorDefault {
		altFill = v.altRowStyle.BG
	}

	// resolve SortBy field name to column index (once)
	if ss := v.sortState; ss != nil && ss.initialCol != "" && !ss.initialDone {
		for i, name := range columns {
			if name == ss.initialCol {
				ss.col = i
				ss.asc = ss.initialAsc
				break
			}
		}
		ss.initialDone = true
	}

	ext := &opAutoTable{
		slicePtr: v.data,
		fields:   fieldIndices,
		headers:  headers,
		hdrStyle: v.headerStyle,
		rowStyle: v.rowStyle,
		altStyle: v.altRowStyle,
		gap:      v.gap,
		fill:     altFill,
		colCfgs:  colCfgs,
		sort:     v.sortState,
		scroll:   v.scroll,
	}

	idx := t.addOp(Op{
		Kind:   OpAutoTable,
		Parent: parent,
		Gap:    v.gap,
		Margin: v.margin,
		Ext:    ext,
	}, depth)
	if v.gapCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Gap = t.compileDynInt8(v.gapCond, nil, 0)
	} else if v.gapPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Gap = v.gapPtr
	}
	return idx
}

// alignOffset returns the x offset needed to align text within the given width.
func alignOffset(text string, width int, align Align) int {
	textLen := StringWidth(text)
	if textLen >= width {
		return 0
	}
	pad := width - textLen
	switch align {
	case AlignRight:
		return pad
	case AlignCenter:
		return pad / 2
	default:
		return 0
	}
}

// autoTableDefaultAlign returns sensible default alignment based on type.
func autoTableDefaultAlign(t reflect.Type) Align {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return AlignRight
	case reflect.Bool:
		return AlignCenter
	default:
		return AlignLeft
	}
}

// autoTableResolveColumns resolves column names to struct field indices.
func autoTableResolveColumns(explicit []string, elemType reflect.Type) (names []string, indices []int) {
	if len(explicit) > 0 {
		for _, name := range explicit {
			f, ok := elemType.FieldByName(name)
			if ok {
				names = append(names, name)
				indices = append(indices, f.Index[0])
			}
		}
		return
	}
	// all exported fields
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		if f.PkgPath == "" {
			names = append(names, f.Name)
			indices = append(indices, i)
		}
	}
	return
}

// compileAutoTableStatic compiles a static (non-pointer) slice into a VBox tree (original behaviour).
func (t *Template) compileAutoTableStatic(v AutoTableC, rv reflect.Value, parent int16, depth int) int16 {
	elemType := rv.Type().Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return t.compileTextC(Text("AutoTable: expected slice of structs"), parent, depth, nil, 0)
	}

	columns := v.columns
	if len(columns) == 0 {
		for i := 0; i < elemType.NumField(); i++ {
			f := elemType.Field(i)
			if f.PkgPath == "" {
				columns = append(columns, f.Name)
			}
		}
	}

	if len(columns) == 0 {
		return t.compileTextC(Text("AutoTable: no columns"), parent, depth, nil, 0)
	}

	headers := v.headers
	if len(headers) == 0 {
		headers = columns
	}

	// resolve column configs for static path
	colCfgs := make([]*ColumnConfig, len(columns))
	for i, col := range columns {
		cfg := &ColumnConfig{}
		if f, ok := elemType.FieldByName(col); ok {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			cfg.align = autoTableDefaultAlign(ft)
		}
		if opt, ok := v.columnConfigs[col]; ok {
			opt(cfg)
		}
		colCfgs[i] = cfg
	}

	widths := make([]int, len(columns))
	for i, h := range headers {
		widths[i] = len(h)
	}

	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		for j, col := range columns {
			field := elem.FieldByName(col)
			if field.IsValid() {
				var str string
				if cfg := colCfgs[j]; cfg != nil && cfg.format != nil {
					str = cfg.format(field.Interface())
				} else {
					str = fmt.Sprintf("%v", field.Interface())
				}
				if len(str) > widths[j] {
					widths[j] = len(str)
				}
			}
		}
	}

	var rows []Component

	var headerCells []Component
	for i, h := range headers {
		hdrStyle := v.headerStyle
		if cfg := colCfgs[i]; cfg != nil {
			hdrStyle.Align = cfg.align
		}
		headerCells = append(headerCells, Text(h).Width(int16(widths[i])).Style(hdrStyle))
	}
	// use cond > pointer > static gap
	var tableGap any
	if v.gapCond != nil {
		tableGap = v.gapCond
	} else if v.gapPtr != nil {
		tableGap = v.gapPtr
	} else {
		tableGap = v.gap
	}
	rows = append(rows, HBox.Gap(tableGap)(headerCells...))

	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}

		isAlt := v.altRowStyle != nil && i%2 == 1
		rowStyle := v.rowStyle
		if isAlt {
			rowStyle = *v.altRowStyle
		}

		var cells []Component
		for j, col := range columns {
			field := elem.FieldByName(col)
			var str string
			cellStyle := rowStyle
			if field.IsValid() {
				val := field.Interface()
				cfg := colCfgs[j]
				if cfg != nil && cfg.format != nil {
					str = cfg.format(val)
				} else {
					str = fmt.Sprintf("%v", val)
				}
				if cfg != nil && cfg.style != nil {
					cellStyle = cfg.style(val)
				}
			}
			if cfg := colCfgs[j]; cfg != nil {
				cellStyle.Align = cfg.align
			}
			cells = append(cells, Text(str).Width(int16(widths[j])).Style(cellStyle))
		}

		row := HBox.Gap(tableGap)
		if isAlt && rowStyle.BG.Mode != ColorDefault {
			row = HBox.Gap(tableGap).Fill(rowStyle.BG)
		}
		rows = append(rows, row(cells...))
	}

	var vbox VBoxC
	if v.border.Horizontal != 0 {
		vbox = VBox.Border(v.border)(rows...)
	} else {
		vbox = VBox(rows...)
	}
	vbox.margin = v.margin

	return t.compileVBoxC(vbox, parent, depth, nil, 0)
}

func (t *Template) compileCheckboxC(v *CheckboxC, parent int16, depth int, elemBase unsafe.Pointer) int16 {
	// Checkbox is: [mark] [label]
	// The mark is conditional based on checked state
	var labelNode Component
	if v.labelPtr != nil {
		labelNode = Text(v.labelPtr)
	} else {
		labelNode = Text(v.label)
	}

	// Use If for the checkbox mark
	mark := If(v.checked).Then(Text(v.checkedMark)).Else(Text(v.unchecked))

	box := HBox.Gap(1)(mark, labelNode)
	box.margin = v.style.margin
	return t.compileHBoxC(box, parent, depth, elemBase, 0)
}

func (t *Template) compileRadioC(v *RadioC, parent int16, depth int) int16 {
	// Radio is a list of options with selection marks
	opts := v.getOptions()
	if len(opts) == 0 {
		return t.compileTextC(Text("(no options)"), parent, depth, nil, 0)
	}

	var items []Component
	for i, opt := range opts {
		idx := i // capture for closure
		mark := IfOrd(v.selected).Eq(idx).Then(Text(v.selectedMark)).Else(Text(v.unselected))
		item := HBox.Gap(1)(mark, Text(opt))
		items = append(items, item)
	}

	// use cond > pointer > static gap
	var gap any
	if v.gapCond != nil {
		gap = v.gapCond
	} else if v.gapPtr != nil {
		gap = v.gapPtr
	} else {
		gap = v.gap
	}

	if v.horizontal {
		hbox := HBox.Gap(gap)(items...)
		hbox.margin = v.style.margin
		return t.compileHBoxC(hbox, parent, depth, nil, 0)
	}
	vbox := VBox.Gap(gap)(items...)
	vbox.margin = v.style.margin
	return t.compileVBoxC(vbox, parent, depth, nil, 0)
}

func (t *Template) compileInputC(v *InputC, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	// Convert to TextInput and compile
	ti := v.toTextInput()
	idx := t.compile(ti, parent, depth, nil, 0)
	if v.widthCond != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthCond, nil, 0)
	} else if v.widthPtr != nil {
		if t.ops[idx].Dyn == nil {
			t.ops[idx].Dyn = &OpDyn{}
		}
		t.ops[idx].Dyn.Width = t.compileDynInt16(v.widthPtr, elemBase, elemSize)
	}
	return idx
}

// Execute runs all three phases and renders to the buffer.
func (t *Template) Execute(buf *Buffer, screenW, screenH int16) {
	// Clear pending from previous frame
	t.pendingOverlays = t.pendingOverlays[:0]
	t.pendingScreenEffects = t.pendingScreenEffects[:0]

	// Phase 0: Evaluate reactive bindings (conditions, animations)
	if t.nowFn != nil {
		t.frameTime = t.nowFn()
	} else {
		t.frameTime = time.Now()
	}
	if t.oscEpoch.IsZero() {
		t.oscEpoch = t.frameTime
	}
	t.animating = false
	for _, eval := range t.evals {
		eval()
	}

	// Phase 1: Width distribution (top → down)
	t.distributeWidths(screenW, nil)

	// Phase 2: Layout (bottom → up) - computes content heights
	t.layout(screenH)

	// Phase 2b: Flex distribution (top → down) - expand flex children
	t.distributeFlexGrow(screenH)

	// Phase 3: Render (top → down)
	t.render(buf, 0, 0, screenW)

	// Phase 4: Render overlays (after main content so they appear on top)
	t.renderOverlays(buf, screenW, screenH)

	// fire deferred tween completions now that the frame's reads are done;
	// callbacks may mutate bound state (including ForEach slices) for the
	// next frame, so request one when any ran
	if len(t.completions) > 0 {
		pending := t.completions
		t.completions = t.completions[:0]
		for _, fn := range pending {
			fn()
		}
		if t.requestRender != nil {
			t.requestRender()
		}
	}

	// manage animation ticker — start at ~60fps when animating, stop when settled
	if t.animating && t.animTicker == nil && t.requestRender != nil {
		t.animTicker = time.NewTicker(16 * time.Millisecond)
		go func() {
			for range t.animTicker.C {
				t.requestRender()
			}
		}()
	} else if !t.animating && t.animTicker != nil {
		t.animTicker.Stop()
		t.animTicker = nil
	}
}

// distributeWidths assigns W to all ops, top-down.
// Each container sets its children's widths. For Rows, this includes flex distribution.
// elemBase is optional - used for offset-based text in ForEach sub-templates.
func (t *Template) distributeWidths(screenW int16, elemBase unsafe.Pointer) {
	// Set root-level ops to screen width first (or compute intrinsic width if FitContent)
	for _, idx := range t.byDepth[0] {
		op := &t.ops[idx]
		geom := &t.geom[idx]
		if op.FitContent {
			// Compute intrinsic width from children
			intrinsicW := t.computeIntrinsicWidth(idx, screenW)
			geom.W = intrinsicW
		} else {
			t.setOpWidth(idx, op, geom, screenW, elemBase)
		}
	}

	// Process containers depth-by-depth, each setting its children's widths
	for depth := 0; depth <= t.maxDepth; depth++ {
		for _, idx := range t.byDepth[depth] {
			op := &t.ops[idx]
			geom := &t.geom[idx]

			switch op.Kind {
			case OpContainer:
				t.distributeWidthsToChildren(idx, op, geom, elemBase)
			case OpJump:
				// Jump is a transparent wrapper - distribute full width to children (like VBox)
				t.distributeVBoxChildWidths(idx, op, geom.W, elemBase)
			}
		}
	}
}

// computeIntrinsicWidth computes the minimum width needed for a ContentSized container.
// For VBox: maximum width of children (all children stack vertically, need same width)
// For HBox: sum of children widths + gaps
func (t *Template) computeIntrinsicWidth(idx, availW int16) int16 {
	return t.computeIntrinsicWidthWithBase(idx, nil, availW)
}

// availW is the space this op could occupy; only the MaxWidth branch consults it
// (to resolve a percentage bound). 0 means "unknown" — a pct bound then falls
// through to plain content measurement, matching how WidthPct behaves here.
func (t *Template) computeIntrinsicWidthWithBase(idx int16, elemBase unsafe.Pointer, availW int16) int16 {
	op := &t.ops[idx]

	// If this op has an explicit width, use it
	if w := op.width(); w > 0 {
		return w
	}

	// A MaxWidth container measures the same way it sizes (setOpWidth): wrap at
	// the bound and hug the longest line, so both paths agree and the clamp
	// constrains content-sizing parents too.
	if op.Kind == OpContainer && (op.MaxWidth > 0 || op.MaxWidthPct > 0) {
		bound := op.MaxWidth
		if op.MaxWidthPct > 0 && availW > 0 {
			bound = int16(float32(availW) * op.MaxWidthPct)
			if bound > availW {
				bound = availW
			}
		}
		if bound > 0 {
			w := t.measureMaxWidthContent(idx, bound, elemBase)
			if w > bound {
				w = bound
			}
			return w
		}
	}

	// For containers, compute from children
	if op.Kind == OpContainer {
		var intrinsicW int16

		// budget passed down for a child's pct resolution, shrunk by this
		// container's own chrome (0 stays 0 = unknown)
		childAvail := availW
		if childAvail > 0 {
			childAvail -= op.marginH() + op.paddingH()
			if op.Border.HasBorder() {
				childAvail -= op.Border.PadH()
			}
			if childAvail < 0 {
				childAvail = 0
			}
		}

		// Count children and find max/sum
		childCount := int16(0)
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			childW := t.computeIntrinsicWidthWithBase(i, elemBase, childAvail)
			childCount++

			if op.IsRow {
				// HBox: sum widths
				intrinsicW += childW
			} else {
				// VBox: max width
				if childW > intrinsicW {
					intrinsicW = childW
				}
			}
		}

		// Add gaps for HBox
		if g := op.gap(); op.IsRow && childCount > 1 && g > 0 {
			intrinsicW += int16(g) * (childCount - 1)
		}

		// Add border
		if op.Border.HasBorder() {
			intrinsicW += op.Border.PadH()
		}

		// Add margin + padding
		intrinsicW += op.marginH() + op.paddingH()

		return intrinsicW
	}

	// Jump is a transparent wrapper: it measures as its children.
	if op.Kind == OpJump {
		var w int16
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			if t.ops[i].Parent != idx {
				continue
			}
			w += t.computeIntrinsicWidthWithBase(i, elemBase, availW)
		}
		return w + op.marginH()
	}

	// For text, compute string width
	if op.Kind == OpText {
		return op.Ext.(*opText).textWidth(elemBase) + op.marginH()
	}

	// Rich text measures its natural single-line span width, so branches and
	// wrappers holding a Textf/Rich report real space instead of zero.
	if op.Kind == OpRichText {
		spans := op.Ext.(*opRichText).resolve(elemBase)
		w := 0
		for _, span := range spans {
			w += StringWidth(span.Text)
		}
		return int16(w) + op.marginH()
	}

	switch op.Kind {
	case OpIf:
		ifExt := op.Ext.(*opIf)
		if elemBase == nil {
			var maxW int16
			for _, tmpl := range []*Template{ifExt.thenTmpl, ifExt.elseTmpl} {
				if tmpl == nil || len(tmpl.ops) == 0 {
					continue
				}
				if w := tmpl.computeIntrinsicWidthWithBase(0, nil, availW); w > maxW {
					maxW = w
				}
			}
			return maxW + op.marginH()
		}
		branches, requested := ifBranches(ifExt, elemBase)
		selected, _ := ifExt.selector(elemBase).selectBranch(requested, branches)
		if tmpl := branchAt(branches, selected); tmpl != nil && len(tmpl.ops) > 0 {
			tmpl.runItemEvalsFrom(t, elemBase)
			return tmpl.computeIntrinsicWidthWithBase(0, elemBase, availW)
		}
		return op.marginH()
	case OpSwitch:
		swExt := op.Ext.(*opSwitch)
		var maxW int16
		for _, ct := range append(swExt.cases, swExt.def) {
			if ct == nil || len(ct.ops) == 0 {
				continue
			}
			ct.runItemEvalsFrom(t, elemBase)
			if w := ct.computeIntrinsicWidthWithBase(0, elemBase, availW); w > maxW {
				maxW = w
			}
		}
		return maxW + op.marginH()
	case OpMatch:
		mExt := op.Ext.(*opMatch)
		var maxW int16
		for _, ct := range append(mExt.cases, mExt.def) {
			if ct == nil || len(ct.ops) == 0 {
				continue
			}
			ct.runItemEvalsFrom(t, elemBase)
			if w := ct.computeIntrinsicWidthWithBase(0, elemBase, availW); w > maxW {
				maxW = w
			}
		}
		return maxW + op.marginH()
	case OpLeader:
		w := op.width()
		if w == 0 {
			w = 20
		}
		return w + op.marginH()
	case OpSparkline:
		w := op.width()
		if w == 0 {
			w = int16(op.Ext.(*opSparkline).dataLen(t.elemBase))
		}
		return w + op.marginH()
	case OpCounter:
		ext := op.Ext.(*opCounter)
		var scratch [48]byte
		b := append(scratch[:0], ext.prefix...)
		b = strconv.AppendInt(b, int64(*(*int)(ext.currentPtr.ptrFor(t.elemBase))), 10)
		b = append(b, '/')
		b = strconv.AppendInt(b, int64(*(*int)(ext.totalPtr.ptrFor(t.elemBase))), 10)
		return int16(len(b)) + op.marginH()
	case OpSpinner:
		return 1 + op.marginH()
	case OpVRule:
		return 1 + op.marginH()
	case OpTabs:
		ext := op.Ext.(*opTabs)
		totalW := 0
		for i, label := range ext.labels {
			labelW := StringWidth(label)
			switch ext.styleType {
			case TabsStyleBox:
				labelW += 4
			case TabsStyleBracket:
				labelW += 2
			}
			totalW += labelW
			if i < len(ext.labels)-1 {
				totalW += ext.gap
			}
		}
		return int16(totalW) + op.marginH()
	}

	return op.marginH()
}

// measureMaxWidthContent computes a max-width container's natural content width.
// Unlike computeIntrinsicWidth, wrappable children (TextBlock) wrap at the bound
// and report their longest produced line, so the container hugs that line rather
// than the full unwrapped text. bound is the container's outer ceiling; the
// returned width includes this container's own chrome and is never forced past
// bound here (the caller clamps the final result).
func (t *Template) measureMaxWidthContent(idx, bound int16, elemBase unsafe.Pointer) int16 {
	op := &t.ops[idx]

	chrome := op.marginH() + op.paddingH()
	if op.Border.HasBorder() {
		chrome += op.Border.PadH()
	}
	inner := bound - chrome
	if inner < 0 {
		inner = 0
	}

	var contentW int16
	childCount := int16(0)
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		childCount++

		var cw int16
		switch childOp.Kind {
		case OpTextBlock:
			ext := childOp.Ext.(*opText)
			for _, ln := range wrapText(ext.resolve(elemBase), int(inner), ext.charWrap) {
				if w := int16(StringWidth(ln)); w > cw {
					cw = w
				}
			}
			cw += childOp.marginH()
		case OpContainer:
			cw = t.measureMaxWidthContent(i, inner, elemBase)
		default:
			cw = t.computeIntrinsicWidthWithBase(i, elemBase, inner)
		}
		if cw > inner {
			cw = inner
		}

		if op.IsRow {
			contentW += cw
		} else if cw > contentW {
			contentW = cw
		}
	}

	if g := op.gap(); op.IsRow && childCount > 1 && g > 0 {
		contentW += int16(g) * (childCount - 1)
	}

	return contentW + chrome
}

func templateIntrinsicWidth(tmpl *Template) int16 {
	if tmpl == nil || len(tmpl.ops) == 0 {
		return 0
	}
	return tmpl.computeIntrinsicWidth(0, 0)
}

func templateIntrinsicWidthWithBase(tmpl *Template, elemBase unsafe.Pointer) int16 {
	if tmpl == nil || len(tmpl.ops) == 0 {
		return 0
	}
	return tmpl.computeIntrinsicWidthWithBase(0, elemBase, 0)
}

func (t *Template) clampRootWidth(maxW int16) {
	if t == nil || len(t.geom) == 0 || maxW < 0 {
		return
	}
	if t.geom[0].W > maxW {
		t.geom[0].W = maxW
	}
}

// setOpWidth sets a single op's width based on available space.
func (t *Template) setOpWidth(idx int16, op *Op, geom *Geom, availW int16, elemBase unsafe.Pointer) {
	switch op.Kind {
	case OpText:
		if w := op.width(); w > 0 {
			geom.W = w
		} else {
			geom.W = op.Ext.(*opText).textWidth(elemBase)
		}

	case OpTextBlock:
		geom.W = availW

	case OpProgress:
		geom.W = op.width()

	case OpCounter:
		ext := op.Ext.(*opCounter)
		var scratch [48]byte
		b := append(scratch[:0], ext.prefix...)
		b = strconv.AppendInt(b, int64(*(*int)(ext.currentPtr.ptrFor(t.elemBase))), 10)
		b = append(b, '/')
		b = strconv.AppendInt(b, int64(*(*int)(ext.totalPtr.ptrFor(t.elemBase))), 10)
		geom.W = int16(len(b))

	case OpLeader:
		geom.W = op.width()
		if geom.W == 0 {
			geom.W = 20
		}

	case OpAutoTable:
		geom.W = availW

	case OpSparkline:
		geom.W = op.width()
		if geom.W == 0 {
			if availW > 0 {
				geom.W = availW
			} else {
				geom.W = int16(op.Ext.(*opSparkline).dataLen(t.elemBase))
			}
		}

	case OpHRule:
		geom.W = 0 // fill available

	case OpVRule:
		geom.W = 1 // single column

	case OpSpacer:
		geom.W = op.width() // 0 = fill available

	case OpSpinner:
		geom.W = 1 // single character width

	case OpScrollbar:
		ext := op.Ext.(*opScrollbar)
		if ext.horizontal {
			if w := op.width(); w > 0 {
				geom.W = w
			} else {
				geom.W = availW
			}
		} else {
			geom.W = 1
		}

	case OpTabs:
		ext := op.Ext.(*opTabs)
		totalW := 0
		for i, label := range ext.labels {
			labelW := StringWidth(label)
			switch ext.styleType {
			case TabsStyleBox:
				labelW += 4
			case TabsStyleBracket:
				labelW += 2
			}
			totalW += labelW
			if i < len(ext.labels)-1 {
				totalW += ext.gap
			}
		}
		geom.W = int16(totalW)

	case OpTreeView:
		ext := op.Ext.(*opTreeView)
		maxW := 0
		if ext.root != nil {
			startLevel := 0
			if !ext.showRoot {
				startLevel = -1
			}
			maxW = t.treeMaxWidth(ext.root, startLevel, ext.indent, ext.showRoot)
		}
		geom.W = int16(maxW)

	case OpCustom:
		ext := op.Ext.(*opCustomRenderer)
		if ext.renderer != nil {
			if cw, ok := ext.renderer.(*customWrapper); ok {
				w, _ := cw.MeasureWithAvail(availW)
				geom.W = w
			} else {
				w, _ := ext.renderer.MinSize()
				geom.W = int16(w)
			}
		}

	case OpLayout:
		geom.W = availW

	case OpLayer:
		ext := op.Ext.(*opLayer)
		if ext.width > 0 {
			geom.W = ext.width
		} else {
			geom.W = availW
		}

	case OpSelectionList:
		geom.W = availW

	case OpForEach:
		if op.Parent >= 0 {
			parent := &t.ops[op.Parent]
			if parent.Kind == OpContainer && parent.IsRow {
				_, w := t.layoutForEachRow(idx, op, availW, parent.gap())
				geom.W = w
			} else {
				geom.W = availW
			}
		} else {
			geom.W = availW
		}

	case OpJump:
		// Jump is a transparent wrapper - uses full available width
		// Children will be laid out within this width
		geom.W = availW

	case OpTextInput:
		// TextInput uses explicit width or fills available
		if w := op.width(); w > 0 {
			geom.W = w
		} else {
			geom.W = availW
		}

	case OpOverlay, OpScreenEffect:
		// Overlays and screen effects take zero space in layout
		geom.W = 0

	case OpIf:
		ifExt := op.Ext.(*opIf)
		branches, requested := ifBranches(ifExt, elemBase)
		selected, exiting := ifExt.selector(elemBase).selectBranch(requested, branches)
		subTmpl := branchAt(branches, selected)
		if subTmpl != nil {
			subTmpl.setExitRenderingFor(elemBase, exiting)
			subTmpl.runItemEvalsFrom(t, elemBase)
			// computeIntrinsicWidth handles both ContentSized containers and
			// leaf nodes (OpText, etc.) that have a computable fixed width.
			// Falls back to 0 for truly flexible content (Space, unsized containers).
			// measure with elemBase so offset-bound text resolves its real
			// content (a nil base returns a placeholder width), and clamp to
			// the available space so the branch never claims past its parent.
			intrinsicW := templateIntrinsicWidthWithBase(subTmpl, elemBase)
			if availW > 0 && intrinsicW > availW {
				intrinsicW = availW
			}
			if intrinsicW > 0 {
				subTmpl.distributeWidths(intrinsicW, elemBase)
				geom.W = intrinsicW
			} else {
				subTmpl.distributeWidths(availW, elemBase)
				if len(subTmpl.geom) > 0 {
					geom.W = subTmpl.geom[0].W
				}
			}
		} else {
			// Condition false with no else branch - takes no space
			geom.W = 0
		}

	case OpSwitch:
		swExt := op.Ext.(*opSwitch)
		var maxW int16
		hasVisualBranch := false
		allTmpls := append(swExt.cases, swExt.def)
		for _, ct := range allTmpls {
			if ct == nil {
				continue
			}
			if len(ct.ops) > 0 {
				hasVisualBranch = true
			}
			w := templateIntrinsicWidth(ct)
			if w > maxW {
				maxW = w
			}
		}
		if maxW > 0 {
			geom.W = maxW
		} else if hasVisualBranch {
			geom.W = availW
		} else {
			geom.W = 0
		}

	case OpMatch:
		mExt := op.Ext.(*opMatch)
		var maxW int16
		hasVisualBranch := false
		allTmpls := append(mExt.cases, mExt.def)
		for _, ct := range allTmpls {
			if ct == nil {
				continue
			}
			if len(ct.ops) > 0 {
				hasVisualBranch = true
			}
			w := templateIntrinsicWidth(ct)
			if w > maxW {
				maxW = w
			}
		}
		if maxW > 0 {
			geom.W = maxW
		} else if hasVisualBranch {
			geom.W = availW
		} else {
			geom.W = 0
		}

	case OpContainer:
		if w := op.width(); w > 0 {
			geom.W = w
		} else if op.hasDynWidth() {
			// a present dynamic width binding means "explicitly sized,
			// currently zero" — honour the zero, not full available width
			geom.W = 0
		} else if pw := op.percentWidth(); pw > 0 {
			geom.W = int16(float32(availW) * pw)
		} else if op.MaxWidth > 0 || op.MaxWidthPct > 0 {
			// size to content but clamp at the bound; wrappable children wrap at
			// the bound and the container hugs the longest produced line.
			bound := op.MaxWidth
			if op.MaxWidthPct > 0 {
				bound = int16(float32(availW) * op.MaxWidthPct)
			}
			if availW > 0 && bound > availW {
				bound = availW
			}
			geom.W = t.measureMaxWidthContent(idx, bound, elemBase)
			if geom.W > bound {
				geom.W = bound
			}
		} else if op.FitContent || (op.Parent >= 0 && op.ContentSized && t.ops[op.Parent].Kind == OpContainer && t.ops[op.Parent].IsRow) {
			geom.W = t.computeIntrinsicWidth(idx, availW)
			if availW > 0 && geom.W > availW {
				geom.W = availW
			}
		} else {
			geom.W = availW
		}

	default:
		geom.W = availW
	}

	// generic margin: non-container ops include margin in their outer width
	if op.Kind != OpContainer && op.marginH() > 0 {
		geom.W += op.marginH()
	}
}

// distributeWidthsToChildren sets widths for all children of a container.
// For Rows: two-pass (non-flex first, then flex distribution).
// For Cols: children fill available width.
func (t *Template) distributeWidthsToChildren(idx int16, op *Op, geom *Geom, elemBase unsafe.Pointer) {
	// Calculate content width (subtract margin + padding + border)
	contentW := geom.W - op.marginH() - op.paddingH()
	if op.Border.HasBorder() {
		contentW -= op.Border.PadH()
	}

	if op.IsRow {
		t.distributeHBoxChildWidths(idx, op, contentW, elemBase)
	} else {
		t.distributeVBoxChildWidths(idx, op, contentW, elemBase)
	}
}

// distributeVBoxChildWidths sets widths for children of a VBox (they fill available width).
func (t *Template) distributeVBoxChildWidths(idx int16, op *Op, availW int16, elemBase unsafe.Pointer) {
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		childGeom := &t.geom[i]
		t.setOpWidth(i, childOp, childGeom, availW, elemBase)
	}
}

// getIfContentOp returns the root op of an If's active branch content.
// Returns nil if condition is false and no else branch, or if template is empty.
func (t *Template) getIfContentOp(childOp *Op, elemBase unsafe.Pointer) *Op {
	childIfExt := childOp.Ext.(*opIf)
	branches, requested := ifBranches(childIfExt, elemBase)
	selected, _ := childIfExt.selector(elemBase).selectBranch(requested, branches)
	if tmpl := branchAt(branches, selected); tmpl != nil && len(tmpl.ops) > 0 {
		return &tmpl.ops[0]
	}
	return nil
}

// distributeHBoxChildWidths sets widths for children of a HBox using two-pass flex.
func (t *Template) distributeHBoxChildWidths(idx int16, op *Op, availW int16, elemBase unsafe.Pointer) {
	// Pass 1: Set widths for non-flex children, collect flex children
	// Containers without explicit width/flex are treated as implicit flex (share remaining space)
	// OpIf is transparent - we look at its content's properties
	var usedW int16
	var totalFlex float32
	var fixedWidthCount int16 // count of non-flex children with width

	flexChildren := t.flexScratchIdx[:0]
	flexGrowValues := t.flexScratchGrow[:0]
	implicitFlexChildren := t.flexScratchImpl[:0]

	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		childGeom := &t.geom[i]

		// For OpIf, look at the content's properties (transparent wrapper)
		effectiveOp := childOp
		if childOp.Kind == OpIf {
			contentOp := t.getIfContentOp(childOp, elemBase)
			if contentOp == nil {
				// Condition false with no else - takes no space
				childGeom.W = 0
				continue
			}
			effectiveOp = contentOp
		}

		if fg := effectiveOp.flexGrow(); fg > 0 {
			// Explicit flex child - defer to pass 2
			totalFlex += fg
			flexChildren = append(flexChildren, i)
			flexGrowValues = append(flexGrowValues, fg)
		} else if effectiveOp.Kind == OpJump && effectiveOp.width() == 0 {
			// Jump is a transparent wrapper: when its content measures, size
			// to it so flex siblings don't starve it to zero width.
			if w := t.computeIntrinsicWidthWithBase(i, elemBase, availW); w > 0 {
				if w > availW {
					w = availW
				}
				childGeom.W = w
				usedW += w
				fixedWidthCount++
			} else {
				implicitFlexChildren = append(implicitFlexChildren, i)
			}
		} else if !effectiveOp.FitContent && !effectiveOp.ContentSized && effectiveOp.Kind == OpContainer && effectiveOp.width() == 0 && effectiveOp.percentWidth() == 0 && !effectiveOp.hasDynWidth() {
			// Container without explicit width or fixed-content children - implicit flex
			implicitFlexChildren = append(implicitFlexChildren, i)
		} else {
			// Non-flex child with explicit or content-based width
			t.setOpWidth(i, childOp, childGeom, availW, elemBase)
			usedW += childGeom.W
			if childGeom.W > 0 {
				fixedWidthCount++
			}
		}
	}

	t.flexScratchIdx = flexChildren
	t.flexScratchGrow = flexGrowValues
	t.flexScratchImpl = implicitFlexChildren

	// Account for gaps - total children that will take space
	// Note: we track fixedWidthCount during the loop above to avoid double-counting
	// flex children that might have non-zero W from a previous render
	childCount := fixedWidthCount + int16(len(flexChildren)) + int16(len(implicitFlexChildren))
	if g := op.gap(); childCount > 1 && g > 0 {
		usedW += int16(g) * (childCount - 1)
	}

	// Pass 2: Distribute remaining width to flex children
	remaining := availW - usedW
	if remaining > 0 && totalFlex > 0 {
		// Explicit flex children
		distributed := int16(0)
		for i, childIdx := range flexChildren {
			childOp := &t.ops[childIdx]
			childGeom := &t.geom[childIdx]

			flexShare := flexGrowValues[i] / totalFlex
			flexW := int16(float32(remaining) * flexShare)

			// Last flex child gets remainder (avoid rounding loss)
			if i == len(flexChildren)-1 {
				flexW = remaining - distributed
			}
			distributed += flexW

			// Set the flex child's width
			childGeom.W = flexW

			// For OpIf, also distribute to sub-template
			if childOp.Kind == OpIf {
				childIfExt := childOp.Ext.(*opIf)
				condTrue := childIfExt.eval(elemBase)
				if condTrue && childIfExt.thenTmpl != nil {
					childIfExt.thenTmpl.elemBase = elemBase
					childIfExt.thenTmpl.distributeWidths(flexW, elemBase)
				} else if !condTrue && childIfExt.elseTmpl != nil {
					childIfExt.elseTmpl.elemBase = elemBase
					childIfExt.elseTmpl.distributeWidths(flexW, elemBase)
				}
			}
		}
	} else if remaining > 0 && len(implicitFlexChildren) > 0 {
		// No explicit flex, but implicit flex containers - share remaining evenly
		shareW := remaining / int16(len(implicitFlexChildren))
		distributed := int16(0)
		for i, childIdx := range implicitFlexChildren {
			childOp := &t.ops[childIdx]
			childGeom := &t.geom[childIdx]

			w := shareW
			// Last child gets remainder
			if i == len(implicitFlexChildren)-1 {
				w = remaining - distributed
			}
			distributed += w
			childGeom.W = w

			// For OpIf, also distribute to sub-template
			if childOp.Kind == OpIf {
				childIfExt := childOp.Ext.(*opIf)
				condTrue := childIfExt.eval(elemBase)
				if condTrue && childIfExt.thenTmpl != nil {
					childIfExt.thenTmpl.elemBase = elemBase
					childIfExt.thenTmpl.distributeWidths(w, elemBase)
				} else if !condTrue && childIfExt.elseTmpl != nil {
					childIfExt.elseTmpl.elemBase = elemBase
					childIfExt.elseTmpl.distributeWidths(w, elemBase)
				}
			}
		}
	}

	// Annotate Extend HRules: find VRule siblings and stamp their X delta onto
	// any HRule with RuleExtend=true in sibling container children.
	t.annotateHRuleExtensions(idx, op, availW)
}

// annotateHRuleExtensions finds VRule children of the HBox at idx and, for each,
// walks sibling container subtrees to set RuleVRuleX on HRules with RuleExtend=true.
// It also checks whether the HBox's parent has a border and stamps border extension
// deltas (RuleExtendLeft/Right) onto the outermost container HRules.
func (t *Template) annotateHRuleExtensions(hboxIdx int16, hboxOp *Op, availW int16) {
	// compute each direct child's X offset within the HBox content area
	cursor := int16(0)
	type childInfo struct {
		idx    int16
		xStart int16
	}
	var children []childInfo
	for i := hboxOp.ChildStart; i < hboxOp.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != hboxIdx {
			continue
		}
		w := t.geom[i].W
		children = append(children, childInfo{idx: i, xStart: cursor})
		if w > 0 {
			cursor += w + int16(hboxOp.gap())
		}
	}

	// for each container child, stamp deltas to its nearest left and right VRules only.
	// using nearest-neighbor (not all VRules) prevents extending through other content.
	// track hasLeft/hasRight per container so border extension can be skipped when a
	// VRule already terminates the HRule on that side.
	type containerSides struct{ hasLeft, hasRight bool }
	sides := map[int16]containerSides{}
	for ci := range children {
		c := &children[ci]
		if t.ops[c.idx].Kind != OpContainer {
			continue
		}
		var leftDelta, rightDelta int16
		hasLeft, hasRight := false, false
		for _, v := range children {
			if t.ops[v.idx].Kind != OpVRule {
				continue
			}
			d := v.xStart - c.xStart
			if d < 0 {
				// VRule to the left — take nearest (largest xStart, i.e. least negative delta)
				if !hasLeft || d > leftDelta {
					leftDelta = d
					hasLeft = true
				}
			} else if d > 0 {
				// VRule to the right — take nearest (smallest xStart, i.e. smallest delta)
				if !hasRight || d < rightDelta {
					rightDelta = d
					hasRight = true
				}
			}
		}
		sides[c.idx] = containerSides{hasLeft: hasLeft, hasRight: hasRight}
		if hasLeft && hasRight {
			t.stampVRuleXPair(c.idx, leftDelta, rightDelta)
		} else if hasLeft {
			t.stampVRuleX(c.idx, leftDelta)
		} else if hasRight {
			t.stampVRuleX(c.idx, rightDelta)
		}
	}

	// if the HBox's direct parent has a border, extend the leftmost and rightmost
	// container HRules to meet the border walls (producing ├ and ┤ junctions).
	// skip border extension when a VRule already terminates the HRule on that side —
	// the VRule endpoint cap will produce ├/┤ via buffer merge instead.
	if hboxOp.Parent >= 0 && int(hboxOp.Parent) < len(t.ops) {
		parentOp := &t.ops[hboxOp.Parent]
		if parentOp.Kind == OpContainer && parentOp.Border.HasBorder() {
			var leftmost, rightmost *childInfo
			for i := range children {
				c := &children[i]
				if t.ops[c.idx].Kind != OpContainer {
					continue
				}
				if leftmost == nil || c.xStart < leftmost.xStart {
					leftmost = &children[i]
				}
				if rightmost == nil || c.xStart > rightmost.xStart {
					rightmost = &children[i]
				}
			}
			if leftmost != nil && !sides[leftmost.idx].hasLeft {
				leftExt := leftmost.xStart + int16(hboxOp.Margin[3]) + 1
				t.stampHRuleExtendBorder(leftmost.idx, leftExt, 0)
			}
			if rightmost != nil && !sides[rightmost.idx].hasRight {
				rightExt := (availW - rightmost.xStart - t.geom[rightmost.idx].W) + int16(hboxOp.Margin[1]) + 1
				t.stampHRuleExtendBorder(rightmost.idx, 0, rightExt)
			}
		}
	}
}

// stampHRuleExtendBorder recursively sets RuleExtendLeft/Right on HRules with
// RuleExtend=true within the subtree rooted at containerIdx.
func (t *Template) stampHRuleExtendBorder(containerIdx int16, left, right int16) {
	containerOp := &t.ops[containerIdx]
	for i := containerOp.ChildStart; i < containerOp.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Kind == OpHRule {
			ext := childOp.Ext.(*opRule)
			if ext.extend {
				if left > 0 {
					ext.extendLeft = left
				}
				if right > 0 {
					ext.extendRight = right
				}
			}
		}
		if childOp.Kind == OpContainer {
			t.stampHRuleExtendBorder(i, left, right)
		}
	}
}

// stampVRuleX recursively sets RuleVRuleX on all HRules with RuleExtend=true
// within the subtree rooted at containerIdx.
func (t *Template) stampVRuleX(containerIdx int16, delta int16) {
	containerOp := &t.ops[containerIdx]
	for i := containerOp.ChildStart; i < containerOp.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Kind == OpHRule {
			if ext := childOp.Ext.(*opRule); ext.extend {
				ext.vruleX = delta
			}
		}
		if childOp.Kind == OpContainer {
			t.stampVRuleX(i, delta)
		}
	}
}

// stampVRuleXPair recursively sets both RuleVRuleX and RuleVRuleX2 on all HRules
// with RuleExtend=true within the subtree rooted at containerIdx.
// Used when a container is flanked by VRules on both sides.
func (t *Template) stampVRuleXPair(containerIdx int16, delta1, delta2 int16) {
	containerOp := &t.ops[containerIdx]
	for i := containerOp.ChildStart; i < containerOp.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Kind == OpHRule {
			if ext := childOp.Ext.(*opRule); ext.extend {
				ext.vruleX = delta1
				ext.vruleX2 = delta2
			}
		}
		if childOp.Kind == OpContainer {
			t.stampVRuleXPair(i, delta1, delta2)
		}
	}
}

// annotateVRuleExtensions finds HRule children of the VBox at idx and, for each,
// walks sibling container subtrees to set RuleExtendTop/Bot on VRules with RuleExtend=true.
func (t *Template) annotateVRuleExtensions(idx int16, op *Op, totalH int16) {
	contentOffY := op.Margin[0] + op.Border.PadTop()

	type childInfo struct {
		idx    int16
		yStart int16
		height int16
	}
	var children []childInfo
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		children = append(children, childInfo{i, t.geom[i].LocalY, t.geom[i].H})
	}

	// collect HRule Y positions
	hRuleYs := make(map[int16]bool)
	for _, c := range children {
		if t.ops[c.idx].Kind == OpHRule {
			hRuleYs[c.yStart] = true
		}
	}

	hasBorder := op.Border.HasBorder()

	for _, c := range children {
		childOp := &t.ops[c.idx]
		if hasBorder && childOp.Kind == OpHRule {
			if ext := childOp.Ext.(*opRule); ext.extend {
				ext.extendLeft = 1
				ext.extendRight = 1
				continue
			}
		}
		if childOp.Kind != OpContainer {
			continue
		}
		extTop := hRuleYs[c.yStart-1] || (hasBorder && c.yStart == contentOffY)
		extBot := hRuleYs[c.yStart+c.height] || (hasBorder && c.yStart+c.height == contentOffY+totalH)
		if extTop || extBot {
			t.stampVRuleExtend(c.idx, extTop, extBot)
		}
	}
}

// stampVRuleExtend recursively sets RuleExtendTop/Bot on all VRules with RuleExtend=true
// within the subtree rooted at containerIdx.
func (t *Template) stampVRuleExtend(containerIdx int16, top, bot bool) {
	containerOp := &t.ops[containerIdx]
	for i := containerOp.ChildStart; i < containerOp.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Kind == OpVRule {
			if ext := childOp.Ext.(*opRule); ext.extend {
				ext.extendTop = top
				ext.extendBot = bot
			}
		}
		if childOp.Kind == OpContainer {
			t.stampVRuleExtend(i, top, bot)
		}
	}
}

// layout computes H and local positions, bottom-up.
func (t *Template) layout(_ int16) {
	// Bottom-up: deepest first
	for depth := t.maxDepth; depth >= 0; depth-- {
		for _, idx := range t.byDepth[depth] {
			op := &t.ops[idx]
			geom := &t.geom[idx]

			switch op.Kind {
			case OpText, OpProgress, OpLeader, OpCounter:
				geom.H = 1

			case OpRichText:
				ext := op.Ext.(*opRichText)
				spans := ext.resolve(t.elemBase)
				w := int(geom.W)
				if w <= 0 {
					w = 72
				}
				n := wrapSpansLines(spans, w, ext.charWrap)
				if n == 0 {
					geom.H = 1
				} else {
					geom.H = int16(n)
				}

			case OpTextBlock:
				ext := op.Ext.(*opText)
				text := ext.resolve(t.elemBase)
				w := int(geom.W)
				if w <= 0 {
					w = 72
				}
				n := wrapTextLines(text, w, ext.charWrap)
				if n == 0 {
					geom.H = 1
				} else {
					geom.H = int16(n)
				}

			case OpAutoTable:
				ext := op.Ext.(*opAutoTable)
				dataRows := 0
				if ext.slicePtr != nil {
					dataRows = reflect.ValueOf(ext.slicePtr).Elem().Len()
				}
				visibleRows := dataRows
				if sc := ext.scroll; sc != nil && sc.maxVisible < visibleRows {
					visibleRows = sc.maxVisible
				}
				geom.H = int16(visibleRows + 1)
				if geom.H == 0 {
					geom.H = 1
				}

			case OpSparkline:
				geom.H = op.height()
				if geom.H <= 0 {
					geom.H = 1
				}

			case OpHRule:
				geom.H = 1

			case OpVRule:
				geom.H = 1 // default height (will be stretched by flex)

			case OpSpacer:
				geom.H = op.height()

			case OpSpinner:
				geom.H = 1 // single line

			case OpScrollbar:
				ext := op.Ext.(*opScrollbar)
				if ext.horizontal {
					geom.H = 1
				} else {
					if h := op.height(); h > 0 {
						geom.H = h
					} else {
						geom.H = 1
					}
				}

			case OpTabs:
				ext := op.Ext.(*opTabs)
				switch ext.styleType {
				case TabsStyleBox:
					geom.H = 3
				default:
					geom.H = 1
				}

			case OpTreeView:
				ext := op.Ext.(*opTreeView)
				count := 0
				if ext.root != nil {
					count = t.treeVisibleCount(ext.root, ext.showRoot)
				}
				geom.H = int16(count)
				if geom.H == 0 {
					geom.H = 1
				}

			case OpSelectionList:
				ext := op.Ext.(*opSelectionList)
				sliceHdr, ok := ext.sliceHeaderFor(t.elemBase)
				if !ok {
					geom.H = 0
					break
				}
				if ext.listPtr != nil {
					ext.listPtr.len = sliceHdr.Len
					ext.listPtr.ensureVisible()
				}

				// layout each item to get per-item heights (follows ForEach pattern)
				contentW := geom.W - ext.markerWidth
				if cap(ext.geoms) < sliceHdr.Len {
					ext.geoms = make([]Geom, sliceHdr.Len)
				}
				ext.geoms = ext.geoms[:sliceHdr.Len]

				cursor := int16(0)
				for li := 0; li < sliceHdr.Len; li++ {
					itemH := int16(1) // default for simple text items
					if ext.iterTmpl != nil && len(ext.iterTmpl.ops) > 0 {
						firstOp := &ext.iterTmpl.ops[0]
						if firstOp.Kind == OpContainer || firstOp.Kind == OpLayout || firstOp.Kind == OpJump || firstOp.Kind == OpRichText || firstOp.Kind == OpTextBlock {
							elemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(li)*ext.elemSize)
							if ext.elemIsPtr {
								elemPtr = *(*unsafe.Pointer)(elemPtr)
							}
							ext.iterTmpl.bindItemContext(t, elemPtr)
							ext.iterTmpl.itemIndex = li
							for _, eval := range ext.iterTmpl.itemEvals {
								eval()
							}
							ext.iterTmpl.distributeWidths(contentW, elemPtr)
							ext.iterTmpl.layout(0)
							itemH = ext.iterTmpl.Height()
							if itemH < 1 {
								itemH = 1
							}
						}
					}
					ext.geoms[li].LocalX = 0
					ext.geoms[li].LocalY = cursor
					ext.geoms[li].H = itemH
					ext.geoms[li].W = geom.W
					cursor += itemH
				}

				// total height is sum of visible items (windowed by MaxVisible or all)
				startIdx := 0
				endIdx := sliceHdr.Len
				if ext.listPtr != nil && ext.listPtr.MaxVisible > 0 {
					startIdx = ext.listPtr.offset
					endIdx = startIdx + ext.listPtr.MaxVisible
					if endIdx > sliceHdr.Len {
						endIdx = sliceHdr.Len
					}
				}
				totalH := int16(0)
				for li := startIdx; li < endIdx; li++ {
					totalH += ext.geoms[li].H
				}
				geom.H = totalH
				if geom.H == 0 {
					geom.H = 1
				}

			case OpCustom:
				// Custom renderer provides its own size
				crExt := op.Ext.(*opCustomRenderer)
				if crExt.renderer != nil {
					// Use customWrapper with computed width for better sizing
					if cw, ok := crExt.renderer.(*customWrapper); ok {
						_, h := cw.MeasureWithAvail(geom.W)
						geom.H = h
					} else {
						_, h := crExt.renderer.MinSize()
						geom.H = int16(h)
					}
				}

			case OpLayer:
				ext := op.Ext.(*opLayer)
				if h := op.height(); h > 0 {
					geom.H = h
				} else if ext.height > 0 {
					geom.H = ext.height
				} else if op.flexGrow() > 0 {
					geom.H = 1
				} else if ext.ptr != nil && ext.ptr.viewHeight > 0 {
					geom.H = int16(ext.ptr.viewHeight)
				} else {
					geom.H = 1
				}
				geom.ContentH = geom.H

			case OpJump:
				// Jump's height is sum of children's heights (like a VBox)
				totalH := int16(0)
				for i := op.ChildStart; i < op.ChildEnd; i++ {
					childOp := &t.ops[i]
					if childOp.Parent == idx {
						childGeom := &t.geom[i]
						childGeom.LocalX = 0
						childGeom.LocalY = totalH
						totalH += childGeom.H
					}
				}
				geom.H = totalH
				if geom.H == 0 {
					geom.H = 1
				}

			case OpTextInput:
				// single-line by default; a multiline input grows to its wrapped height
				geom.H = 1
				if ext, ok := op.Ext.(*opTextInput); ok && ext.multiline && geom.W > 0 {
					if v := ext.value(); v != "" {
						if n := len(inputWrapLines([]rune(v), int(geom.W))); n > 1 {
							geom.H = int16(n)
						}
					}
				}

			case OpOverlay, OpScreenEffect:
				// Overlays and screen effects take zero space in layout
				geom.H = 0

			case OpIf:
				// root-level OpIf (e.g. ForEach iter template root); container children
				// are handled inline by layoutContainer and skipped here
				if op.Parent != -1 {
					break
				}
				ifExt := op.Ext.(*opIf)
				branches, requested := ifBranches(ifExt, t.elemBase)
				selected, exiting := ifExt.selector(t.elemBase).selectBranch(requested, branches)
				if tmpl := branchAt(branches, selected); tmpl != nil {
					tmpl.setExitRenderingFor(t.elemBase, exiting)
					tmpl.runItemEvalsFrom(t, t.elemBase)
					tmpl.distributeWidths(geom.W, t.elemBase)
					tmpl.layout(0)
					geom.H = tmpl.Height()
				} else {
					geom.H = 0
				}

			case OpSwitch:
				// root-level OpSwitch (e.g. ForEach iter template root); container
				// children are handled inline by layoutContainer and skipped here
				if op.Parent != -1 {
					break
				}
				swExt := op.Ext.(*opSwitch)
				branches, requested := switchBranches(swExt, t.elemBase)
				selected, exiting := swExt.selector(t.elemBase).selectBranch(requested, branches)
				switchTmpl := branchAt(branches, selected)
				if switchTmpl != nil {
					switchTmpl.setExitRenderingFor(t.elemBase, exiting)
					switchTmpl.runItemEvalsFrom(t, t.elemBase)
					switchTmpl.distributeWidths(geom.W, t.elemBase)
					switchTmpl.layout(0)
					geom.H = switchTmpl.Height()
				} else {
					geom.H = 0
				}

			case OpMatch:
				if op.Parent != -1 {
					break
				}
				mExt := op.Ext.(*opMatch)
				branches, requested := matchBranches(mExt, t.elemBase)
				selected, exiting := mExt.selector(t.elemBase).selectBranch(requested, branches)
				matchTmpl := branchAt(branches, selected)
				if matchTmpl != nil {
					matchTmpl.setExitRenderingFor(t.elemBase, exiting)
					matchTmpl.runItemEvalsFrom(t, t.elemBase)
					matchTmpl.distributeWidths(geom.W, t.elemBase)
					matchTmpl.layout(0)
					geom.H = matchTmpl.Height()
				} else {
					geom.H = 0
				}

			case OpLayout:
				t.layoutCustom(idx, op, geom)

			case OpContainer:
				t.layoutContainer(idx, op, geom)
			}

			// generic margin: non-container ops include margin in their outer height
			if op.Kind != OpContainer && op.marginV() > 0 {
				geom.H += op.marginV()
			}
		}
	}
}

// layoutContainer positions children and computes container height.
func (t *Template) layoutContainer(idx int16, op *Op, geom *Geom) {
	// Content area offset for margin + border + padding
	contentOffX := op.Margin[3] // left margin
	contentOffY := op.Margin[0] // top margin
	contentOffX += op.Border.PadLeft()
	contentOffY += op.Border.PadTop()
	contentOffX += op.Padding[3] // left padding
	contentOffY += op.Padding[0] // top padding

	availW := geom.W - op.marginH() - op.paddingH()
	if op.Border.HasBorder() {
		availW -= op.Border.PadH()
	}

	if op.IsRow {
		// Horizontal layout
		cursor := int16(0)
		maxH := int16(0)
		needGap := false // Add gap before next visible child

		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue // not direct child
			}

			// Control flow ops expand to their content
			switch childOp.Kind {
			case OpIf:
				childIfExt := childOp.Ext.(*opIf)
				branches, requested := ifBranches(childIfExt, t.elemBase)
				selected, exiting := childIfExt.selector(t.elemBase).selectBranch(requested, branches)
				tmpl := branchAt(branches, selected)
				// Use pre-calculated width if set (from flex distribution), otherwise use availW
				ifWidth := t.geom[i].W
				if ifWidth == 0 {
					ifWidth = availW
				}
				if tmpl != nil {
					// Add gap before this child if needed
					if g := op.gap(); needGap && g > 0 {
						cursor += int16(g)
					}
					tmpl.setExitRenderingFor(t.elemBase, exiting)
					tmpl.runItemEvalsFrom(t, t.elemBase)
					tmpl.distributeWidths(ifWidth, t.elemBase)
					tmpl.layout(0)
					h := tmpl.Height()
					t.geom[i].LocalX = contentOffX + cursor
					t.geom[i].LocalY = contentOffY
					t.geom[i].H = h
					// Use sub-template width only if we didn't have a pre-set width
					if t.geom[i].W == 0 && len(tmpl.geom) > 0 {
						t.geom[i].W = tmpl.geom[0].W
					}
					cursor += t.geom[i].W
					if h > maxH {
						maxH = h
					}
					needGap = true // Next visible child needs gap
				}
				// If condition false with no else, don't set needGap (takes no space)

			case OpForEach:
				// Add gap before this child if needed
				if g := op.gap(); needGap && g > 0 {
					cursor += int16(g)
				}
				childAvailW := t.geom[i].W
				if childAvailW <= 0 {
					childAvailW = availW
				}
				h, w := t.layoutForEachRow(i, childOp, childAvailW, op.gap())
				t.geom[i].LocalX = contentOffX + cursor
				t.geom[i].LocalY = contentOffY
				t.geom[i].H = h
				t.geom[i].W = w
				cursor += w
				if h > maxH {
					maxH = h
				}
				if w > 0 {
					needGap = true
				}

			case OpSwitch:
				// Layout all cases to find the maximum width. In a ForEach, all rows
				// share one geom array (last-element wins), so the Switch must reserve
				// enough space for any case that could render — otherwise wider cases
				// get truncated and column positions vary per row, breaking alignment.
				childSwExt := childOp.Ext.(*opSwitch)
				var maxCaseW, maxCaseH int16
				allCaseTmpls := append(childSwExt.cases, childSwExt.def)
				for _, ct := range allCaseTmpls {
					if ct == nil {
						continue
					}
					ct.elemBase = t.elemBase
					ct.distributeWidths(availW, t.elemBase)
					ct.layout(0)
					if len(ct.geom) > 0 && ct.geom[0].W > maxCaseW {
						maxCaseW = ct.geom[0].W
					}
					if h := ct.Height(); h > maxCaseH {
						maxCaseH = h
					}
				}
				if maxCaseW > 0 {
					if g := op.gap(); needGap && g > 0 {
						cursor += int16(g)
					}
					t.geom[i].LocalX = contentOffX + cursor
					t.geom[i].LocalY = contentOffY
					t.geom[i].W = maxCaseW
					t.geom[i].H = maxCaseH
					cursor += maxCaseW
					if maxCaseH > maxH {
						maxH = maxCaseH
					}
					needGap = true
				}

			case OpMatch:
				childMExt := childOp.Ext.(*opMatch)
				var maxCaseW, maxCaseH int16
				allCaseTmpls := append(childMExt.cases, childMExt.def)
				for _, ct := range allCaseTmpls {
					if ct == nil {
						continue
					}
					ct.elemBase = t.elemBase
					ct.distributeWidths(availW, t.elemBase)
					ct.layout(0)
					if len(ct.geom) > 0 && ct.geom[0].W > maxCaseW {
						maxCaseW = ct.geom[0].W
					}
					if h := ct.Height(); h > maxCaseH {
						maxCaseH = h
					}
				}
				if maxCaseW > 0 {
					if g := op.gap(); needGap && g > 0 {
						cursor += int16(g)
					}
					t.geom[i].LocalX = contentOffX + cursor
					t.geom[i].LocalY = contentOffY
					t.geom[i].W = maxCaseW
					t.geom[i].H = maxCaseH
					cursor += maxCaseW
					if maxCaseH > maxH {
						maxH = maxCaseH
					}
					needGap = true
				}

			default:
				childGeom := &t.geom[i]
				// Add gap before this child if needed
				if g := op.gap(); needGap && g > 0 && childGeom.W > 0 {
					cursor += int16(g)
				}
				childGeom.LocalX = contentOffX + cursor
				childGeom.LocalY = contentOffY
				cursor += childGeom.W
				if childGeom.H > maxH {
					maxH = childGeom.H
				}
				if childGeom.W > 0 {
					needGap = true
				}
			}
		}

		geom.H = maxH
		if op.Border.HasBorder() {
			geom.H += op.Border.PadV()
		}
		geom.H += op.marginV() + op.paddingV()
	} else {
		// Vertical layout
		cursor := int16(0)
		firstChild := true

		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}

			// Handle gap
			if g := op.gap(); !firstChild && g > 0 {
				cursor += int16(g)
			}
			firstChild = false

			// Control flow ops expand to their content
			switch childOp.Kind {
			case OpIf:
				childIfExt := childOp.Ext.(*opIf)
				branches, requested := ifBranches(childIfExt, t.elemBase)
				selected, exiting := childIfExt.selector(t.elemBase).selectBranch(requested, branches)
				tmpl := branchAt(branches, selected)
				if tmpl != nil {
					tmpl.setExitRenderingFor(t.elemBase, exiting)
					tmpl.runItemEvalsFrom(t, t.elemBase)
					tmpl.distributeWidths(availW, t.elemBase)
					tmpl.layout(0)
					h := tmpl.Height()
					t.geom[i].LocalX = contentOffX
					t.geom[i].LocalY = contentOffY + cursor
					t.geom[i].H = h
					t.geom[i].ContentH = h // Track content height for flex
					t.geom[i].W = availW
					cursor += h
				} else {
					t.geom[i].H = 0 // condition false and no else, takes no space
					t.geom[i].ContentH = 0
				}

			case OpForEach:
				h, _ := t.layoutForEach(i, childOp, availW)
				t.geom[i].LocalX = contentOffX
				t.geom[i].LocalY = contentOffY + cursor
				t.geom[i].H = h
				t.geom[i].W = availW
				cursor += h

			case OpSwitch:
				childSwExt := childOp.Ext.(*opSwitch)
				branches, requested := switchBranches(childSwExt, t.elemBase)
				selected, exiting := childSwExt.selector(t.elemBase).selectBranch(requested, branches)
				tmpl := branchAt(branches, selected)
				if tmpl != nil {
					tmpl.setExitRenderingFor(t.elemBase, exiting)
					tmpl.runItemEvalsFrom(t, t.elemBase)
					tmpl.distributeWidths(availW, t.elemBase)
					tmpl.layout(0)
					h := tmpl.Height()
					t.geom[i].LocalX = contentOffX
					t.geom[i].LocalY = contentOffY + cursor
					t.geom[i].H = h
					t.geom[i].W = availW
					cursor += h
				} else {
					t.geom[i].H = 0
				}

			case OpMatch:
				childMExt := childOp.Ext.(*opMatch)
				branches, requested := matchBranches(childMExt, t.elemBase)
				selected, exiting := childMExt.selector(t.elemBase).selectBranch(requested, branches)
				tmpl := branchAt(branches, selected)
				if tmpl != nil {
					tmpl.setExitRenderingFor(t.elemBase, exiting)
					tmpl.runItemEvalsFrom(t, t.elemBase)
					tmpl.distributeWidths(availW, t.elemBase)
					tmpl.layout(0)
					h := tmpl.Height()
					t.geom[i].LocalX = contentOffX
					t.geom[i].LocalY = contentOffY + cursor
					t.geom[i].H = h
					t.geom[i].W = availW
					cursor += h
				} else {
					t.geom[i].H = 0
				}

			default:
				childGeom := &t.geom[i]
				childGeom.LocalX = contentOffX
				childGeom.LocalY = contentOffY + cursor
				cursor += childGeom.H
			}
		}

		// Annotate VRule extensions: find HRule siblings and stamp extend flags onto VRules.
		t.annotateVRuleExtensions(idx, op, cursor)

		geom.H = cursor
		if op.Border.HasBorder() {
			geom.H += op.Border.PadV()
		}
		geom.H += op.marginV() + op.paddingV()
	}

	// Store content height before any override (for flex distribution)
	geom.ContentH = geom.H

	// Explicit height overrides
	if h := op.height(); h > 0 {
		geom.H = h
	}
}

// distributeFlexGrow distributes remaining space to flex children.
// Called top-down after layout phase.
// Vertical containers (VBox) distribute height, horizontal containers (HBox) distribute width.
// distributeFlexGrow distributes remaining height to VBox flex children.
// HBox flex is handled during width distribution (single pass).
// VBox flex must happen after layout since it needs content heights.
func (t *Template) distributeFlexGrow(rootH int16) {
	// First pass: ensure root element fills screen height
	// This makes the common case "just work" without needing VBox wrappers
	if len(t.byDepth[0]) > 0 {
		for _, idx := range t.byDepth[0] {
			op := &t.ops[idx]
			geom := &t.geom[idx]
			if op.Kind == OpContainer && op.Parent == -1 {
				// Root container fills screen height (unless explicit height or FitContent)
				if op.height() == 0 && !op.FitContent {
					geom.H = rootH
				}
			}
		}
	}

	// Second pass: process depth by depth
	for depth := 0; depth <= t.maxDepth; depth++ {
		for _, idx := range t.byDepth[depth] {
			op := &t.ops[idx]

			if op.Kind == OpContainer {
				if op.IsRow {
					// HBox: stretch children to fill HBox height
					t.stretchRowChildren(idx, op)
				} else {
					// VBox: distribute vertical flex space
					t.distributeFlexInCol(idx, op, rootH)
				}
			}
		}
	}
}

// stretchRowChildren stretches HBox children to fill the HBox's height.
// This enables VBox children inside an HBox to use flex for vertical distribution.
func (t *Template) stretchRowChildren(idx int16, op *Op) {
	geom := &t.geom[idx]
	availH := geom.H - op.marginV() - op.paddingV()
	if op.Border.HasBorder() {
		availH -= op.Border.PadV()
	}

	// Stretch each child to fill the row height
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		childGeom := &t.geom[i]

		// Stretch containers, layers, vertical rules, and vertical scrollbars to fill height.
		if childOp.Kind == OpContainer || childOp.Kind == OpLayer || childOp.Kind == OpVRule || childOp.Kind == OpScrollbar {
			if childOp.height() == 0 && childGeom.H < availH {
				childGeom.H = availH
			}
		}

		// Handle If ops - stretch their content too
		if childOp.Kind == OpIf {
			childGeom.H = availH
			t.stretchIfContent(childOp, availH)
		}
	}
}

// stretchIfContent stretches the active branch of an If to the given height.
func (t *Template) stretchIfContent(op *Op, newH int16) {
	ifExt := op.Ext.(*opIf)
	condTrue := ifExt.eval(t.elemBase)

	var tmpl *Template
	if condTrue && ifExt.thenTmpl != nil {
		tmpl = ifExt.thenTmpl
	} else if !condTrue && ifExt.elseTmpl != nil {
		tmpl = ifExt.elseTmpl
	}

	if tmpl == nil || len(tmpl.ops) == 0 {
		return
	}

	// Stretch root of sub-template
	rootOp := &tmpl.ops[0]
	if rootOp.Kind == OpContainer || rootOp.Kind == OpLayer {
		if rootOp.height() == 0 {
			tmpl.geom[0].H = newH
			// redistribute the branch's INTERNAL flex against the stretched height —
			// the branch was laid out at content height (layout(0)), so without this
			// its Grow children and height-0 stretch elements (scrollbars, vrules)
			// keep the content-sized pass and never see the real height (e.g. a
			// scrollbar inside an If-wrapped pane collapsing to ~0 rows). Mirrors
			// propagateFlexToIf, which already does this on the VBox flex path.
			if rootOp.Kind == OpContainer {
				tmpl.distributeFlexGrow(newH)
			}
		}
	}
}

// distributeFlexInCol distributes vertical flex space within a column container.
func (t *Template) distributeFlexInCol(idx int16, op *Op, rootH int16) {
	geom := &t.geom[idx]

	// Calculate available height
	// If this container is a flex child, it already has its height set by parent's distribution
	// Use that height, not the parent's full height
	var availH int16
	if op.flexGrow() > 0 && geom.H > 0 {
		// This container is a flex child - use its own height (already computed)
		availH = geom.H - op.marginV() - op.paddingV()
		if op.Border.HasBorder() {
			availH -= op.Border.PadV()
		}
	} else if op.Parent >= 0 {
		parentGeom := &t.geom[op.Parent]
		parentOp := &t.ops[op.Parent]
		availH = parentGeom.H - parentOp.marginV() - parentOp.paddingV()
		if parentOp.Border.HasBorder() {
			availH -= parentOp.Border.PadV()
		}
	} else {
		availH = rootH - op.marginV() - op.paddingV()
		if op.Border.HasBorder() {
			availH -= op.Border.PadV()
		}
	}

	// If this container has explicit height, use that
	if h := op.height(); h > 0 {
		availH = h - op.marginV() - op.paddingV()
		if op.Border.HasBorder() {
			availH -= op.Border.PadV()
		}
	}

	// Calculate used height and total flex grow (reuse scratch slices)
	var usedH int16
	var totalFlex float32
	var childCount int16
	flexChildren := t.flexScratchIdx[:0]
	flexGrowValues := t.flexScratchGrow[:0]

	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		childCount++

		childGeom := &t.geom[i]

		// Check for direct flex child (container, layer or spacer)
		if fg := childOp.flexGrow(); (childOp.Kind == OpContainer || childOp.Kind == OpLayer || childOp.Kind == OpSpacer) && fg > 0 {
			totalFlex += fg
			flexChildren = append(flexChildren, i)
			flexGrowValues = append(flexGrowValues, fg)
			usedH += childGeom.ContentH // Use content height for flex children
			continue
		}

		// Check for If containing a flex child in its active branch
		if childOp.Kind == OpIf {
			flexGrow := t.getIfFlexGrow(childOp)
			if flexGrow > 0 {
				totalFlex += flexGrow
				flexChildren = append(flexChildren, i)
				flexGrowValues = append(flexGrowValues, flexGrow)
				usedH += childGeom.ContentH
				continue
			}
		}

		usedH += childGeom.H
	}
	t.flexScratchIdx = flexChildren
	t.flexScratchGrow = flexGrowValues

	// Add gaps to used height
	if g := op.gap(); childCount > 1 && g > 0 {
		usedH += int16(g) * (childCount - 1)
	}

	// Distribute remaining space (handles both expansion and shrinkage)
	remaining := availH - usedH
	if remaining != 0 && totalFlex > 0 {
		distributed := int16(0)
		for i, childIdx := range flexChildren {
			childGeom := &t.geom[childIdx]
			flexShare := flexGrowValues[i] / totalFlex
			extraH := int16(float32(remaining) * flexShare)

			// Give any remainder to the last flex child (avoid rounding loss)
			if i == len(flexChildren)-1 {
				extraH = remaining - distributed
			}
			distributed += extraH
			h := childGeom.ContentH + extraH
			if h < 0 {
				h = 0
			}
			childGeom.H = h
		}

		// Recalculate child positions with new heights. Must mirror layoutContainer's
		// content offset exactly — margin + border + PADDING. Without padding here,
		// any column whose flex redistribution ran loses its top padding and children
		// ride up over it.
		contentOffY := int16(op.Margin[0]) + op.Border.PadTop() + int16(op.Padding[0])
		cursor := int16(0)
		firstChild := true

		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}

			if g := op.gap(); !firstChild && g > 0 {
				cursor += int16(g)
			}
			firstChild = false

			childGeom := &t.geom[i]
			childGeom.LocalY = contentOffY + cursor
			cursor += childGeom.H
		}

		// Propagate extra height to nested templates in If ops
		for _, childIdx := range flexChildren {
			childOp := &t.ops[childIdx]
			if childOp.Kind == OpIf {
				childGeom := &t.geom[childIdx]
				t.propagateFlexToIf(childOp, childGeom.H)
			}
		}

		// Update container height to match available
		geom.H = availH
		if op.Border.HasBorder() {
			geom.H += op.Border.PadV()
		}
	}
}

// propagateFlexToIf propagates flex height to an If's active branch template.
func (t *Template) propagateFlexToIf(op *Op, newH int16) {
	ifExt := op.Ext.(*opIf)
	condTrue := ifExt.eval(t.elemBase)

	var tmpl *Template
	if condTrue && ifExt.thenTmpl != nil {
		tmpl = ifExt.thenTmpl
	} else if !condTrue && ifExt.elseTmpl != nil {
		tmpl = ifExt.elseTmpl
	}

	if tmpl == nil || len(tmpl.ops) == 0 {
		return
	}

	// If root is a flex container, update its height and redistribute
	rootOp := &tmpl.ops[0]
	if rootOp.Kind == OpContainer && rootOp.flexGrow() > 0 {
		tmpl.geom[0].H = newH
		tmpl.distributeFlexGrow(newH)
	}
}

// getIfFlexGrow returns the FlexGrow value from an If's active branch, if any.
// This allows If-wrapped containers to participate in flex distribution.
func (t *Template) getIfFlexGrow(op *Op) float32 {
	// Determine which branch is active
	ifExt := op.Ext.(*opIf)
	condTrue := ifExt.eval(t.elemBase)

	var tmpl *Template
	if condTrue && ifExt.thenTmpl != nil {
		tmpl = ifExt.thenTmpl
	} else if !condTrue && ifExt.elseTmpl != nil {
		tmpl = ifExt.elseTmpl
	}

	if tmpl == nil || len(tmpl.ops) == 0 {
		return 0
	}

	// Check if root op of the branch is a Container with FlexGrow
	rootOp := &tmpl.ops[0]
	if fg := rootOp.flexGrow(); rootOp.Kind == OpContainer && fg > 0 {
		return fg
	}

	return 0
}

// layoutCustom handles custom layout containers using the Arranger interface.
func (t *Template) layoutCustom(idx int16, op *Op, geom *Geom) {
	clExt := op.Ext.(*opCustomLayout)
	if clExt.layout == nil {
		return
	}

	// Collect child sizes
	var childSizes []ChildSize
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue // not direct child
		}
		childGeom := &t.geom[i]
		childSizes = append(childSizes, ChildSize{
			MinW: int(childGeom.W),
			MinH: int(childGeom.H),
		})
	}

	// Call the layout function
	rects := clExt.layout(childSizes, int(geom.W), int(geom.H))

	// Apply positions to children
	childIdx := 0
	maxH := int16(0)
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent != idx {
			continue
		}
		if childIdx < len(rects) {
			r := rects[childIdx]
			t.geom[i].LocalX = int16(r.X)
			t.geom[i].LocalY = int16(r.Y)
			t.geom[i].W = int16(r.W)
			t.geom[i].H = int16(r.H)
			if int16(r.Y)+int16(r.H) > maxH {
				maxH = int16(r.Y) + int16(r.H)
			}
		}
		childIdx++
	}

	// Set container height to encompass all children
	geom.H = maxH
}

// layoutForEach iterates items, lays each item out vertically, and returns
// total height plus max width.
func (t *Template) layoutForEach(_ int16, op *Op, availW int16) (totalH, maxW int16) {
	feExt := op.Ext.(*opForEach)
	if feExt.iterTmpl == nil {
		return 0, 0
	}

	sliceHdr, ok := feExt.sliceHeaderFor(t.elemBase)
	if !ok {
		return 0, 0
	}
	visible := feExt.visibleLen(t.elemBase, sliceHdr.Len)
	if visible == 0 {
		return 0, 0
	}

	// Ensure we have enough geometry slots for items
	if cap(feExt.geoms) < visible {
		feExt.geoms = make([]Geom, visible)
	}
	feExt.geoms = feExt.geoms[:visible]

	cursor := int16(0)
	for i := 0; i < visible; i++ {
		// Get element pointer for this item
		elemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(i)*feExt.elemSize)
		if feExt.elemIsPtr {
			elemPtr = *(*unsafe.Pointer)(elemPtr)
		}

		// Layout sub-template for this item with element base
		feExt.iterTmpl.runItemEvalsFrom(t, elemPtr)
		feExt.iterTmpl.distributeWidths(availW, elemPtr)
		feExt.iterTmpl.layout(0)
		itemH := feExt.iterTmpl.Height()

		feExt.geoms[i].LocalX = 0
		feExt.geoms[i].LocalY = cursor
		feExt.geoms[i].H = itemH
		feExt.geoms[i].W = availW

		cursor += itemH

		if availW > maxW {
			maxW = availW
		}
	}

	return cursor, maxW
}

// layoutForEachRow iterates items, lays each item out horizontally, and returns
// max height plus total width. This is used when ForEach is a direct child of
// an HBox, where the repeated items should behave like row children.
func (t *Template) layoutForEachRow(_ int16, op *Op, availW int16, gap int8) (maxH, totalW int16) {
	feExt := op.Ext.(*opForEach)
	if feExt.iterTmpl == nil {
		return 0, 0
	}

	sliceHdr, ok := feExt.sliceHeaderFor(t.elemBase)
	if !ok {
		return 0, 0
	}
	visible := feExt.visibleLen(t.elemBase, sliceHdr.Len)
	if visible == 0 {
		return 0, 0
	}

	if cap(feExt.geoms) < visible {
		feExt.geoms = make([]Geom, visible)
	}
	feExt.geoms = feExt.geoms[:visible]

	cursor := int16(0)
	for i := 0; i < visible; i++ {
		elemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(i)*feExt.elemSize)
		if feExt.elemIsPtr {
			elemPtr = *(*unsafe.Pointer)(elemPtr)
		}

		if i > 0 && gap > 0 {
			cursor += int16(gap)
		}
		if cursor >= availW {
			feExt.geoms[i].LocalX = availW
			feExt.geoms[i].LocalY = 0
			feExt.geoms[i].H = 0
			feExt.geoms[i].W = 0
			continue
		}

		remainingW := availW - cursor
		feExt.iterTmpl.runItemEvalsFrom(t, elemPtr)
		itemW := templateIntrinsicWidthWithBase(feExt.iterTmpl, elemPtr)
		if itemW <= 0 {
			itemW = remainingW
		}
		if itemW > remainingW {
			itemW = remainingW
		}
		feExt.iterTmpl.distributeWidths(itemW, elemPtr)
		feExt.iterTmpl.layout(0)
		feExt.iterTmpl.clampRootWidth(itemW)

		itemH := feExt.iterTmpl.Height()
		if len(feExt.iterTmpl.geom) > 0 {
			itemW = feExt.iterTmpl.geom[0].W
		}
		if itemW < 0 {
			itemW = 0
		}
		if itemW > remainingW {
			itemW = remainingW
		}

		feExt.geoms[i].LocalX = cursor
		feExt.geoms[i].LocalY = 0
		feExt.geoms[i].H = itemH
		feExt.geoms[i].W = itemW

		cursor += itemW
		if itemH > maxH {
			maxH = itemH
		}
	}

	return maxH, cursor
}

// render draws to buffer, accumulating global positions top-down.
func (t *Template) render(buf *Buffer, globalX, globalY, maxW int16) {
	t.renderOp(buf, 0, globalX, globalY, maxW)
}

// applyTransform applies a text transform to a string.
func applyTransform(s string, transform TextTransform) string {
	switch transform {
	case TransformUppercase:
		return strings.ToUpper(s)
	case TransformLowercase:
		return strings.ToLower(s)
	case TransformCapitalize:
		// capitalize first letter of each word
		var result strings.Builder
		result.Grow(len(s))
		capitalizeNext := true
		for _, r := range s {
			if r == ' ' || r == '\t' || r == '\n' {
				capitalizeNext = true
				result.WriteRune(r)
			} else if capitalizeNext {
				result.WriteRune(unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				result.WriteRune(r)
			}
		}
		return result.String()
	default:
		return s
	}
}

// effectiveStyle returns the style to use, merging with inherited style.
// If s is completely empty, returns the inherited style.
// Otherwise, cascades: Fill→BG, Attr (merged), Transform (if not set).
func (t *Template) effectiveStyle(s Style) Style {
	if t.inheritedStyle == nil && t.inheritedFill.Mode == ColorDefault {
		return s
	}
	// fully empty style inherits everything (except margin, which never cascades)
	if s.Equal(Style{}) && t.inheritedStyle != nil {
		result := *t.inheritedStyle
		result.margin = [4]int16{}
		// use cascaded Fill as BG for text rendering
		if result.BG.Mode == ColorDefault && t.inheritedFill.Mode != ColorDefault {
			result.BG = t.inheritedFill
		}
		return result
	}
	// partial style: merge inherited properties
	if t.inheritedStyle != nil {
		// merge Attr (combine both)
		s.Attr = s.Attr | t.inheritedStyle.Attr
		// inherit FG if not set
		if s.FG.Mode == ColorDefault && t.inheritedStyle.FG.Mode != ColorDefault {
			s.FG = t.inheritedStyle.FG
		}
		// inherit BG if not set (cascaded Fill may override below)
		if s.BG.Mode == ColorDefault && t.inheritedStyle.BG.Mode != ColorDefault {
			s.BG = t.inheritedStyle.BG
		}
		// inherit Transform if not set
		if s.Transform == TransformNone && t.inheritedStyle.Transform != TransformNone {
			s.Transform = t.inheritedStyle.Transform
		}
	}
	// use cascaded Fill as BG if no explicit BG
	if s.BG.Mode == ColorDefault && t.inheritedFill.Mode != ColorDefault {
		s.BG = t.inheritedFill
	}
	return s
}

func clampOpacity(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (t *Template) currentRefOpacity() float64 {
	if !t.refOpacitySet {
		return 1
	}
	return clampOpacity(t.refOpacity)
}

func refOpacity(ref *NodeRef) float64 {
	if ref == nil {
		return 1
	}
	if !ref.opacitySet && ref.Opacity == 0 {
		return 1
	}
	return clampOpacity(ref.Opacity)
}

// NodeOpacity returns a node ref's effective rendered opacity. Refs without an
// opacity-producing ancestor are treated as fully opaque.
func NodeOpacity(ref *NodeRef) float64 {
	return refOpacity(ref)
}

func (t *Template) opacityForOp(op *Op) (float64, bool) {
	if op.Dyn == nil || (op.Dyn.Opacity == nil && !op.Dyn.OpacityIsOff) {
		return 1, false
	}
	if op.Dyn.OpacityIsOff {
		if t.elemBase == nil {
			return 1, false
		}
		ptr := (*float64)(unsafe.Pointer(uintptr(t.elemBase) + op.Dyn.OpacityOff))
		return clampOpacity(*ptr), true
	}
	if op.Dyn.OpacityArmed != nil {
		*op.Dyn.OpacityArmed = true
	}
	return clampOpacity(*op.Dyn.Opacity), true
}

func (t *Template) snapshotRect(buf *Buffer, x, y, w, h int) []Cell {
	if w <= 0 || h <= 0 {
		return nil
	}
	cells := make([]Cell, w*h)
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			cells[cy*w+cx] = buf.Get(x+cx, y+cy)
		}
	}
	return cells
}

func (t *Template) composeOpacityRect(buf *Buffer, x, y, w, h int, backing []Cell, opacity float64, mode OpacityMode) {
	if len(backing) == 0 || w <= 0 || h <= 0 || opacity >= 1 {
		return
	}
	opacity = clampOpacity(opacity)
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			if !buf.InBounds(x+cx, y+cy) {
				continue
			}
			back := backing[cy*w+cx]
			if opacity <= 0 {
				buf.SetFast(x+cx, y+cy, back)
				continue
			}
			src := buf.Get(x+cx, y+cy)
			buf.SetFast(x+cx, y+cy, composeOpacityCell(src, back, opacity, mode, cx, cy, buf.defaultStyle))
		}
	}
}

func composeOpacityCell(src, back Cell, opacity float64, mode OpacityMode, x, y int, defaultStyle Style) Cell {
	opacity = clampOpacity(opacity)
	if opacity <= 0 {
		return back
	}
	if opacity >= 1 {
		return src
	}

	hasSourceRune := cellHasRune(src)
	hasBackingRune := cellHasRune(back)
	sourceOwnsRune := opacitySourceOwnsRune(opacity, mode, x, y, hasSourceRune, hasBackingRune)
	bg := blendOpacityColor(back.Style.BG, src.Style.BG, opacity, defaultStyle.BG)
	result := back
	if sourceOwnsRune {
		result.Rune = src.Rune
		result.Style.Attr = src.Style.Attr
		target := bg
		strength := opacity
		if hasBackingRune {
			threshold := opacityHandoffThreshold(mode, x, y, hasBackingRune)
			if mode == OpacityPaint {
				target = blendSourceRuneFG(src.Style.FG, back.Style.FG, threshold)
			} else {
				target = blendBackingRuneFG(back.Style.FG, bg, opacity, defaultStyle)
			}
			strength = opacityAboveHandoff(opacity, threshold)
		}
		result.Style.FG = blendSourceRuneFG(src.Style.FG, target, strength)
	} else if mode == OpacityPaint && hasBackingRune {
		result.Style.FG = blendSourceRuneFG(src.Style.FG, result.Style.FG, opacity)
	} else {
		result.Style.FG = blendBackingRuneFG(back.Style.FG, bg, opacity, defaultStyle)
	}
	result.Style.BG = bg
	return result
}

func opacitySourceOwnsRune(opacity float64, mode OpacityMode, x, y int, hasSourceRune, hasBackingRune bool) bool {
	if !hasSourceRune {
		return false
	}
	if !hasBackingRune {
		return opacity > 0
	}
	return opacity >= opacityHandoffThreshold(mode, x, y, hasBackingRune)
}

func opacityHandoffThreshold(mode OpacityMode, x, y int, hasBackingRune bool) float64 {
	if !hasBackingRune {
		return 0
	}
	if mode == OpacityPaint {
		return 0.7
	}
	if mode == OpacityDither {
		return 0.55 - 0.35*bayer4Threshold(x, y)
	}
	return 0.35
}

func opacityAboveHandoff(opacity, threshold float64) float64 {
	if threshold <= 0 {
		return opacity
	}
	return clampOpacity((opacity - threshold) / (1 - threshold))
}

func cellHasRune(c Cell) bool {
	return c.Rune != 0 && c.Rune != ' '
}

func bayer4Threshold(x, y int) float64 {
	matrix := [16]uint8{
		0, 8, 2, 10,
		12, 4, 14, 6,
		3, 11, 1, 9,
		15, 7, 13, 5,
	}
	idx := (y&3)*4 + (x & 3)
	return (float64(matrix[idx]) + 0.5) / 16
}

func blendOpacityColor(back, src Color, opacity float64, fallback Color) Color {
	if src.Mode == ColorDefault {
		return back
	}
	if back.Mode == ColorDefault {
		if fallback.Mode == ColorDefault {
			return src
		}
		back = fallback
	}
	return Lerp(back, src, opacity)
}

func blendSourceRuneFG(src, bg Color, opacity float64) Color {
	if src.Mode == ColorDefault {
		return bg
	}
	if bg.Mode == ColorDefault {
		return src
	}
	return Lerp(bg, src, opacity)
}

func blendBackingRuneFG(backFG, bg Color, opacity float64, defaultStyle Style) Color {
	if backFG.Mode == ColorDefault {
		backFG = defaultStyle.FG
	}
	if bg.Mode == ColorDefault {
		return backFG
	}
	if backFG.Mode == ColorDefault {
		return bg
	}
	return Lerp(backFG, bg, opacity)
}

// clipLines returns the rows available for wrapped text starting at absY,
// bounded by the buffer bottom and any active vertical clip.
func (t *Template) clipLines(buf *Buffer, absY int16) int {
	maxLines := buf.Height() - int(absY)
	if t.clipMaxY > 0 {
		if avail := int(t.clipMaxY) - int(absY); avail < maxLines {
			maxLines = avail
		}
	}
	return maxLines
}

// branchBudget clamps a branch's render width to the space remaining in the
// parent's budget, so a branch never paints past its container's edge.
func branchBudget(geomW, maxW, absX, globalX int16) int16 {
	if avail := maxW - (absX - globalX); avail < geomW {
		return avail
	}
	return geomW
}

func (t *Template) renderOp(buf *Buffer, idx int16, globalX, globalY, maxW int16) {
	if idx < 0 || int(idx) >= len(t.ops) {
		return
	}

	op := &t.ops[idx]
	geom := &t.geom[idx]

	// Compute absolute position
	absX := globalX + geom.LocalX
	absY := globalY + geom.LocalY

	// generic margin offset for non-container ops (containers handle margin themselves)
	if op.Kind != OpContainer && op.marginH()+op.marginV() > 0 {
		absX += op.Margin[3] // left
		absY += op.Margin[0] // top
		maxW -= op.marginH()
	}

	// content dimensions exclude margin (for ops without margin, marginH/V == 0)
	contentW := geom.W - op.marginH()
	contentH := geom.H - op.marginV()

	switch op.Kind {
	case OpText:
		ext := op.Ext.(*opText)
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := t.effectiveStyle(baseStyle)
		raw := ext.resolve(t.elemBase)
		text := applyTransform(raw, style.Transform)
		x := int(absX)
		drawW := int(maxW - (absX - globalX))
		// an explicit width (static or dynamic binding) clips the content:
		// declared size means what it says, matching container behaviour
		if w := op.width(); w > 0 && int(w) < drawW {
			drawW = int(w)
		}
		if drawW <= 0 {
			return
		}
		if style.Align != AlignLeft {
			alignW := op.width()
			if alignW == 0 {
				alignW = int16(drawW)
			}
			x += alignOffset(text, int(alignW), style.Align)
		}
		opacity, hasOpacity := t.opacityForOp(op)
		var backing []Cell
		if hasOpacity && opacity < 1 {
			backing = t.snapshotRect(buf, x, int(absY), drawW, 1)
		}
		buf.WriteStringFast(x, int(absY), text, style, drawW)
		if hasOpacity && opacity < 1 {
			t.composeOpacityRect(buf, x, int(absY), drawW, 1, backing, opacity, op.OpacityMode)
		}

	case OpTextBlock:
		ext := op.Ext.(*opText)
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := t.effectiveStyle(baseStyle)
		raw := ext.resolve(t.elemBase)
		maxLines := t.clipLines(buf, absY)
		if maxLines > 0 {
			wrapTextDraw(raw, buf, int(absX), int(absY), int(contentW), maxLines, style, ext.charWrap)
		}

	case OpProgress:
		ext := op.Ext.(*opProgress)
		ratio := float32(ext.resolve(t.elemBase)) / 100.0
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := t.effectiveStyle(baseStyle)
		buf.WriteProgressBar(int(absX), int(absY), int(op.width()), ratio, style)

	case OpRichText:
		ext := op.Ext.(*opRichText)
		spans := ext.resolve(t.elemBase)
		if spans != nil {
			spans = styleSpans(spans, t.effectiveStyle)
			if ext.preserveBG {
				for i := range spans {
					spans[i].Style.Attr |= AttrPreserveBG
				}
			}
			maxLines := t.clipLines(buf, absY)
			if maxLines > 0 {
				wrapSpansDraw(spans, buf, int(absX), int(absY), int(contentW), maxLines, ext.charWrap, t.richSpanJumpFunc(buf))
			}
		}

	case OpLeader:
		t.renderLeader(buf, op, absX, absY, maxW)

	case OpCounter:
		t.renderCounter(buf, op, absX, absY, maxW)

	case OpAutoTable:
		t.renderAutoTable(buf, op, absX, absY, maxW)

	case OpSparkline:
		op.Ext.(*opSparkline).render(t, buf, absX, absY, contentW, geom.H)

	case OpHRule:
		ext := op.Ext.(*opRule)
		width := int(maxW)
		if contentW > 0 {
			width = int(contentW)
		}
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		ruleStyle := t.effectiveStyle(baseStyle)
		for i := 0; i < width; i++ {
			buf.Set(int(absX)+i, int(absY), Cell{Rune: ext.char, Style: ruleStyle})
		}
		if ext.extend && ext.vruleX != 0 {
			delta := int(ext.vruleX)
			if delta > 0 {
				for i := width; i <= delta; i++ {
					r := ext.char
					if i == delta {
						r = '╴'
					}
					buf.Set(int(absX)+i, int(absY), Cell{Rune: r, Style: ruleStyle})
				}
			} else {
				for i := delta; i < 0; i++ {
					r := ext.char
					if i == delta {
						r = '╶'
					}
					buf.Set(int(absX)+i, int(absY), Cell{Rune: r, Style: ruleStyle})
				}
			}
		}
		if ext.extend && ext.vruleX2 != 0 {
			delta := int(ext.vruleX2)
			if delta > 0 {
				for i := width; i <= delta; i++ {
					r := ext.char
					if i == delta {
						r = '╴'
					}
					buf.Set(int(absX)+i, int(absY), Cell{Rune: r, Style: ruleStyle})
				}
			} else {
				for i := delta; i < 0; i++ {
					r := ext.char
					if i == delta {
						r = '╶'
					}
					buf.Set(int(absX)+i, int(absY), Cell{Rune: r, Style: ruleStyle})
				}
			}
		}
		if ext.extendLeft > 0 {
			n := int(ext.extendLeft)
			buf.Set(int(absX)-n, int(absY), Cell{Rune: '╶', Style: ruleStyle})
			for i := 1; i < n; i++ {
				buf.Set(int(absX)-i, int(absY), Cell{Rune: ext.char, Style: ruleStyle})
			}
		}
		if ext.extendRight > 0 {
			n := int(ext.extendRight)
			buf.Set(int(absX)+width+n-1, int(absY), Cell{Rune: '╴', Style: ruleStyle})
			for i := 0; i < n-1; i++ {
				buf.Set(int(absX)+width+i, int(absY), Cell{Rune: ext.char, Style: ruleStyle})
			}
		}

	case OpVRule:
		ext := op.Ext.(*opRule)
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		ruleStyle := t.effectiveStyle(baseStyle)
		for i := 0; i < int(contentH); i++ {
			buf.Set(int(absX), int(absY)+i, Cell{Rune: ext.char, Style: ruleStyle})
		}
		if ext.extendTop {
			buf.Set(int(absX), int(absY)-1, Cell{Rune: '╷', Style: ruleStyle})
		}
		if ext.extendBot {
			buf.Set(int(absX), int(absY)+int(contentH), Cell{Rune: '╵', Style: ruleStyle})
		}

	case OpSpacer:
		ext := op.Ext.(*opRule)
		spacerStyle := ext.style
		if ext.stylePtr != nil {
			spacerStyle = *ext.stylePtr
		}
		if ext.char != 0 {
			for x := int16(0); x < contentW; x++ {
				buf.Set(int(absX+x), int(absY), Cell{Rune: ext.char, Style: spacerStyle})
			}
		}

	case OpSpinner:
		ext := op.Ext.(*opSpinner)
		if frameIdx, ok := ext.frameIndex(t); ok {
			frame := ext.frames[frameIdx]
			baseStyle := ext.style
			if ext.stylePtr != nil {
				baseStyle = *ext.stylePtr
			}
			style := t.effectiveStyle(baseStyle)
			buf.WriteStringFast(int(absX), int(absY), frame, style, 1)
		}

	case OpScrollbar:
		t.renderScrollbar(buf, op, geom, absX, absY)

	case OpTabs:
		t.renderTabs(buf, op, geom, absX, absY)

	case OpTreeView:
		t.renderTreeView(buf, op, absX, absY)

	case OpSelectionList:
		t.renderSelectionList(buf, op, geom, absX, absY, maxW)

	case OpJump:
		t.renderJump(buf, op, geom, absX, absY, maxW, idx)

	case OpTextInput:
		t.renderTextInput(buf, op, geom, absX, absY)

	case OpOverlay:
		t.pendingOverlays = append(t.pendingOverlays, pendingOverlay{op: op})

	case OpScreenEffect:
		ext := op.Ext.(*opScreenEffect)
		t.pendingScreenEffects = append(t.pendingScreenEffects, ext.fns...)

	case OpCustom:
		t.renderCustomRenderer(buf, op, absX, absY, contentW, contentH)

	case OpLayout:
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			t.renderOp(buf, i, absX, absY, contentW)
		}

	case OpLayer:
		t.renderLayer(buf, op, absX, absY, contentW, contentH)

	case OpContainer:
		// Margin inset: visible box starts inside the margin
		boxX := absX + op.Margin[3] // left margin
		boxY := absY + op.Margin[0] // top margin
		boxW := geom.W - op.marginH()
		boxH := geom.H - op.marginV()

		opacity, hasOpacity := t.opacityForOp(op)
		effectiveRefOpacity := t.currentRefOpacity()
		if hasOpacity {
			effectiveRefOpacity *= opacity
		}
		if op.NodeRef != nil {
			op.NodeRef.X = int(boxX)
			op.NodeRef.Y = int(boxY)
			op.NodeRef.W = int(boxW)
			op.NodeRef.H = int(boxH)
			op.NodeRef.Opacity = effectiveRefOpacity
			op.NodeRef.opacitySet = true
		}

		var backing []Cell
		if hasOpacity && opacity < 1 {
			backing = t.snapshotRect(buf, int(boxX), int(boxY), int(boxW), int(boxH))
		}

		// Update inherited Fill - cascades through nested containers
		oldInheritedFill := t.inheritedFill
		opFill := op.fillFor(t.elemBase)
		if op.CascadeStyle != nil && op.CascadeStyle.Fill.Mode != ColorDefault {
			t.inheritedFill = op.CascadeStyle.Fill
		} else if opFill.Mode != ColorDefault {
			t.inheritedFill = opFill
		}

		// Update inherited style if this container sets one (before title rendering)
		oldInheritedStyle := t.inheritedStyle
		if op.CascadeStyle != nil {
			t.inheritedStyle = op.CascadeStyle
		}

		// Fill container area - direct Fill takes precedence over inherited
		fillColor := t.inheritedFill
		if opFill.Mode != ColorDefault {
			fillColor = opFill
		}
		if op.LocalStyle != nil && op.LocalStyle.Fill.Mode != ColorDefault {
			fillColor = op.LocalStyle.Fill
		}
		if fillColor.Mode != ColorDefault {
			fillCell := Cell{Rune: ' ', Style: Style{BG: fillColor}}
			buf.FillRect(int(boxX), int(boxY), int(boxW), int(boxH), fillCell)
		}

		// Draw border if present
		if op.Border.HasBorder() {
			style := DefaultStyle()
			if op.BorderFG != nil {
				style.FG = *op.BorderFG
			}
			if op.LocalStyle != nil && op.LocalStyle.FG.Mode != ColorDefault {
				style.FG = op.LocalStyle.FG
			}
			if op.BorderBG != nil {
				style.BG = *op.BorderBG
			} else if op.LocalStyle != nil && op.LocalStyle.BG.Mode != ColorDefault {
				style.BG = op.LocalStyle.BG
			} else if fillColor.Mode != ColorDefault {
				style.BG = fillColor
			}
			if style.FG.Mode != ColorDefault || op.BorderFG == nil {
				buf.DrawBorder(int(boxX), int(boxY), int(boxW), int(boxH), op.Border, style)
			}

			if op.Title != "" {
				titleTransform := TransformNone
				if t.inheritedStyle != nil {
					titleTransform = t.inheritedStyle.Transform
				}
				titleMaxW := int(boxW) - 2
				titleX := int(boxX) + 1
				if titleMaxW > 0 {
					buf.SetFast(titleX, int(boxY), Cell{Rune: op.Border.topChar(), Style: style})
					titleX++
					buf.SetFast(titleX, int(boxY), Cell{Rune: ' ', Style: style})
					titleX++
					title := applyTransform(op.Title, titleTransform)
					titleW := StringWidth(title)
					availTitleW := titleMaxW - 3 // border char + space before + space after
					if availTitleW > 0 {
						if titleW > availTitleW {
							titleW = availTitleW
						}
						buf.WriteStringFast(titleX, int(boxY), title, style, titleW)
						titleX += titleW
						buf.SetFast(titleX, int(boxY), Cell{Rune: ' ', Style: style})
					}
				}
			}
		}

		// Calculate content width (accounting for margin + border)
		contentW := boxW
		if op.Border.HasBorder() {
			contentW -= op.Border.PadH()
		}

		// Set vertical clip for children (content area bottom)
		oldClipMaxY := t.clipMaxY
		contentBottom := boxY + boxH
		contentBottom -= op.Border.PadBottom()
		if t.clipMaxY == 0 || contentBottom < t.clipMaxY {
			t.clipMaxY = contentBottom
		}

		oldRefOpacity := t.refOpacity
		oldRefOpacitySet := t.refOpacitySet
		t.refOpacity = effectiveRefOpacity
		t.refOpacitySet = true

		// Render children with this container's position as their origin
		// children's LocalX/Y already include margin+border offsets from layoutContainer
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			t.renderOp(buf, i, absX, absY, contentW)
		}

		if hasOpacity && opacity < 1 {
			t.composeOpacityRect(buf, int(boxX), int(boxY), int(boxW), int(boxH), backing, opacity, op.OpacityMode)
		}

		// Restore inherited style, fill, and clip
		t.inheritedStyle = oldInheritedStyle
		t.inheritedFill = oldInheritedFill
		t.clipMaxY = oldClipMaxY
		t.refOpacity = oldRefOpacity
		t.refOpacitySet = oldRefOpacitySet

	case OpIf:
		ifExt := op.Ext.(*opIf)
		branches, requested := ifBranches(ifExt, t.elemBase)
		t.renderSelectedBranch(buf, branches, requested, ifExt.selector(t.elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), t.elemBase)

	case OpForEach:
		// Render each item using iterGeoms for positioning
		feExt := op.Ext.(*opForEach)
		if feExt.iterTmpl == nil {
			return
		}
		sliceHdr, ok := feExt.sliceHeaderFor(t.elemBase)
		if !ok {
			return
		}
		if sliceHdr.Len == 0 {
			return
		}

		for i := 0; i < sliceHdr.Len && i < len(feExt.geoms); i++ {
			itemGeom := &feExt.geoms[i]
			itemAbsX := absX + itemGeom.LocalX
			itemAbsY := absY + itemGeom.LocalY

			// Rebind template ops to this element's data
			elemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(i)*feExt.elemSize)
			if feExt.elemIsPtr {
				elemPtr = *(*unsafe.Pointer)(elemPtr)
			}

			// run per-item evaluators so conditions/tweens resolve for this item
			feExt.iterTmpl.runItemEvalsFrom(t, elemPtr)
			feExt.iterTmpl.itemIndex = i
			feExt.iterTmpl.distributeWidths(itemGeom.W, elemPtr)
			feExt.iterTmpl.layout(0)
			feExt.iterTmpl.clampRootWidth(itemGeom.W)

			// apply dynamic fills on root container before rendering
			if len(feExt.iterTmpl.ops) > 0 {
				rootOp := &feExt.iterTmpl.ops[0]
				if rootOp.Kind == OpContainer {
					rootGeom := &feExt.iterTmpl.geom[0]
					fillColor := rootOp.fillFor(elemPtr)
					if fillColor.Mode != ColorDefault {
						bx := int(itemAbsX) + int(rootOp.Margin[3])
						by := int(itemAbsY) + int(rootOp.Margin[0])
						bw := int(rootGeom.W) - int(rootOp.marginH())
						bh := int(rootGeom.H) - int(rootOp.marginV())
						buf.FillRect(bx, by, bw, bh, Cell{Rune: ' ', Style: Style{BG: fillColor}})
					}
				}
			}
			t.renderSubTemplate(buf, feExt.iterTmpl, itemAbsX, itemAbsY, itemGeom.W, elemPtr)
		}

	case OpSwitch:
		swExt := op.Ext.(*opSwitch)
		branches, requested := switchBranches(swExt, t.elemBase)
		t.renderSelectedBranch(buf, branches, requested, swExt.selector(t.elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), t.elemBase)

	case OpMatch:
		mExt := op.Ext.(*opMatch)
		branches, requested := matchBranches(mExt, t.elemBase)
		t.renderSelectedBranch(buf, branches, requested, mExt.selector(t.elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), t.elemBase)
	}
}

// renderSubTemplate renders a sub-template (for ForEach) with element-bound data.
func (t *Template) renderSubTemplate(buf *Buffer, sub *Template, globalX, globalY, maxW int16, elemBase unsafe.Pointer) {
	sub.app = t.app
	sub.clipMaxY = t.clipMaxY // propagate vertical clip
	sub.setJumpViewport(t.jumpOffsetX, t.jumpOffsetY, t.jumpMinY, t.jumpMaxY)
	if sub.rowBG.Mode != ColorDefault {
		sub.inheritedFill = sub.rowBG
	} else {
		sub.inheritedFill = t.inheritedFill // propagate fill so blank cells use parent bg
	}
	sub.bindItemContext(t, elemBase) // ensure renderOp paths (e.g. via renderJump) see the correct element
	sub.pendingOverlays = sub.pendingOverlays[:0]
	sub.pendingScreenEffects = sub.pendingScreenEffects[:0]
	for i := range sub.ops {
		if sub.ops[i].Parent == -1 {
			sub.renderSubOp(buf, int16(i), globalX, globalY, maxW, elemBase)
		}
	}
	t.pendingOverlays = append(t.pendingOverlays, sub.pendingOverlays...)
	t.pendingScreenEffects = append(t.pendingScreenEffects, sub.pendingScreenEffects...)
}

func (t *Template) renderBranchTemplate(buf *Buffer, sub *Template, globalX, globalY, maxW int16, elemBase unsafe.Pointer, exiting bool) {
	sub.app = t.app
	sub.inheritedStyle = t.inheritedStyle
	sub.inheritedFill = t.inheritedFill
	sub.clipMaxY = t.clipMaxY
	sub.setJumpViewport(t.jumpOffsetX, t.jumpOffsetY, t.jumpMinY, t.jumpMaxY)
	sub.bindItemContext(t, elemBase)
	sub.setExitRenderingFor(elemBase, exiting)
	sub.pendingOverlays = sub.pendingOverlays[:0]
	sub.pendingScreenEffects = sub.pendingScreenEffects[:0]
	if exiting && !sub.hasActiveExitLeases() {
		sub.runExitEvals()
	}
	sub.runItemEvalsFrom(t, elemBase)
	oldRefOpacity := sub.refOpacity
	oldRefOpacitySet := sub.refOpacitySet
	sub.refOpacity = t.currentRefOpacity()
	sub.refOpacitySet = true
	sub.render(buf, globalX, globalY, maxW)
	sub.refOpacity = oldRefOpacity
	sub.refOpacitySet = oldRefOpacitySet
	// an exiting branch's overlays carry that state up, so renderOverlay releases their
	// modal routers instead of re-pushing them while the fade plays (orphan fix).
	for _, po := range sub.pendingOverlays {
		po.exiting = po.exiting || exiting
		t.pendingOverlays = append(t.pendingOverlays, po)
	}
	t.pendingScreenEffects = append(t.pendingScreenEffects, sub.pendingScreenEffects...)
}

func (t *Template) renderSelectedBranch(buf *Buffer, branches []*Template, requested int, selector *branchSelector, globalX, globalY, maxW int16, elemBase unsafe.Pointer) {
	setRouteBranchActive(branches, requested)
	selected, exiting := selector.selectBranch(requested, branches)
	tmpl := branchAt(branches, selected)
	if tmpl == nil {
		return
	}
	t.renderBranchTemplate(buf, tmpl, globalX, globalY, maxW, elemBase, exiting)
	if exiting {
		selector.markExitRendered()
	}
}

func (t *Template) renderLeader(buf *Buffer, op *Op, absX, absY, maxW int16) {
	ext := op.Ext.(*opLeader)
	width := int(op.width())
	if width == 0 {
		width = int(maxW)
	}
	baseStyle := ext.style
	if ext.stylePtr != nil {
		baseStyle = *ext.stylePtr
	}
	style := t.effectiveStyle(baseStyle)
	label := applyTransform(ext.label, style.Transform)
	var value string
	switch ext.mode {
	case leaderPtr:
		value = applyTransform(*(*string)(ext.valuePtr.ptrFor(t.elemBase)), style.Transform)
	case leaderIntPtr:
		var scratch [20]byte
		b := strconv.AppendInt(scratch[:0], int64(*(*int)(ext.intPtr.ptrFor(t.elemBase))), 10)
		value = applyTransform(unsafe.String(&b[0], len(b)), style.Transform)
	case leaderFloatPtr:
		var scratch [32]byte
		b := strconv.AppendFloat(scratch[:0], *(*float64)(ext.floatPtr.ptrFor(t.elemBase)), 'f', 1, 64)
		value = applyTransform(unsafe.String(&b[0], len(b)), style.Transform)
	default:
		value = applyTransform(ext.value, style.Transform)
	}
	buf.WriteLeader(int(absX), int(absY), label, value, width, ext.fill, style)
}

func (t *Template) renderCounter(buf *Buffer, op *Op, absX, absY, maxW int16) {
	ext := op.Ext.(*opCounter)
	style := t.effectiveStyle(ext.style)
	var scratch [48]byte
	var b []byte
	prefix := ext.prefix
	if ext.streamingPtr != nil && *ext.streamingPtr && len(prefix) > 0 {
		frame := int(atomic.LoadInt32(ext.framePtr))
		b = append(scratch[:0], SpinnerCircle[frame%len(SpinnerCircle)]...)
		b = append(b, prefix[1:]...)
	} else {
		b = append(scratch[:0], prefix...)
	}
	b = strconv.AppendInt(b, int64(*(*int)(ext.currentPtr.ptrFor(t.elemBase))), 10)
	b = append(b, '/')
	b = strconv.AppendInt(b, int64(*(*int)(ext.totalPtr.ptrFor(t.elemBase))), 10)
	text := unsafe.String(&b[0], len(b))
	buf.WriteStringFast(int(absX), int(absY), text, style, int(maxW))
}

func (t *Template) renderCustomRenderer(buf *Buffer, op *Op, absX, absY, contentW, contentH int16) {
	crExt := op.Ext.(*opCustomRenderer)
	if crExt.renderer != nil {
		crExt.renderer.Render(buf, int(absX), int(absY), int(contentW), int(contentH))
	}
}

func (t *Template) renderLayer(buf *Buffer, op *Op, absX, absY, contentW, contentH int16) {
	ext := op.Ext.(*opLayer)
	if ext.ptr == nil {
		return
	}

	layerW := int(contentW)
	if ext.width > 0 {
		layerW = int(ext.width)
	}
	ext.ptr.SetViewport(layerW, int(contentH))
	ext.ptr.screenX = int(absX)
	ext.ptr.screenY = int(absY)
	if t.app != nil {
		ext.ptr.app = t.app
		ext.ptr.defaultStyle = t.app.defaultStyle
		if t.app.JumpModeActive() {
			ext.ptr.Invalidate()
		}
	}
	ext.ptr.prepare()
	ext.ptr.blit(buf, int(absX), int(absY), layerW, int(contentH))

	if t.inheritedFill.Mode != ColorDefault {
		for cy := int(absY); cy < int(absY)+int(contentH) && cy < buf.height; cy++ {
			for cx := int(absX); cx < int(absX)+layerW && cx < buf.width; cx++ {
				cell := buf.Get(cx, cy)
				if cell.Style.BG.Mode == ColorDefault {
					cell.Style.BG = t.inheritedFill
					buf.Set(cx, cy, cell)
				}
			}
		}
	}

	if ext.ptr.cursor.Visible && t.app != nil {
		t.app.activeLayer = ext.ptr
	}
}

// renderSubOp renders a single op in a sub-template, recursing into children.
func (t *Template) renderSubOp(buf *Buffer, idx int16, globalX, globalY, maxW int16, elemBase unsafe.Pointer) {
	op := &t.ops[idx]
	geom := &t.geom[idx]

	absX := globalX + geom.LocalX
	absY := globalY + geom.LocalY

	// generic margin offset for non-container ops
	if op.Kind != OpContainer && op.marginH()+op.marginV() > 0 {
		absX += op.Margin[3]
		absY += op.Margin[0]
		maxW -= op.marginH()
	}

	// content dimensions exclude margin
	contentW := geom.W - op.marginH()
	contentH := geom.H - op.marginV()

	// merge row selection style with text style (also applies inherited style)
	mergeStyle := func(s Style) Style {
		s = t.effectiveStyle(s)
		if t.rowBG.Mode != 0 && s.BG.Mode == 0 {
			s.BG = t.rowBG
		}
		if t.rowFG.Mode != 0 && s.FG.Mode == 0 {
			s.FG = t.rowFG
		}
		s.Attr = s.Attr | t.rowAttr
		return s
	}

	switch op.Kind {
	case OpText:
		ext := op.Ext.(*opText)
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := mergeStyle(baseStyle)
		raw := ext.resolve(elemBase)
		text := applyTransform(raw, style.Transform)
		x := int(absX)
		drawW := int(maxW - (absX - globalX))
		// an explicit width (static or dynamic binding) clips the content
		if w := op.width(); w > 0 && int(w) < drawW {
			drawW = int(w)
		}
		if drawW <= 0 {
			return
		}
		if style.Align != AlignLeft {
			alignW := op.width()
			if alignW == 0 {
				alignW = int16(drawW)
			}
			x += alignOffset(text, int(alignW), style.Align)
		}
		opacity, hasOpacity := t.opacityForOp(op)
		var backing []Cell
		if hasOpacity && opacity < 1 {
			backing = t.snapshotRect(buf, x, int(absY), drawW, 1)
		}
		buf.WriteStringFast(x, int(absY), text, style, drawW)
		if hasOpacity && opacity < 1 {
			t.composeOpacityRect(buf, x, int(absY), drawW, 1, backing, opacity, op.OpacityMode)
		}

	case OpTextBlock:
		ext := op.Ext.(*opText)
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := mergeStyle(baseStyle)
		raw := ext.resolve(elemBase)
		maxLines := t.clipLines(buf, absY)
		if maxLines > 0 {
			wrapTextDraw(raw, buf, int(absX), int(absY), int(contentW), maxLines, style, ext.charWrap)
		}

	case OpProgress:
		ext := op.Ext.(*opProgress)
		ratio := float32(ext.resolve(elemBase)) / 100.0
		baseStyle := ext.style
		if ext.stylePtr != nil {
			baseStyle = *ext.stylePtr
		}
		style := t.effectiveStyle(baseStyle)
		buf.WriteProgressBar(int(absX), int(absY), int(op.width()), ratio, style)

	case OpRichText:
		ext := op.Ext.(*opRichText)
		spans := ext.resolve(elemBase)
		if spans != nil {
			spans = styleSpans(spans, mergeStyle)
			if ext.preserveBG {
				for i := range spans {
					spans[i].Style.Attr |= AttrPreserveBG
				}
			}
			maxLines := t.clipLines(buf, absY)
			if maxLines > 0 {
				wrapSpansDraw(spans, buf, int(absX), int(absY), int(contentW), maxLines, ext.charWrap, t.richSpanJumpFunc(buf))
			}
		}

	case OpLeader:
		t.renderLeader(buf, op, absX, absY, maxW)

	case OpCounter:
		t.renderCounter(buf, op, absX, absY, maxW)

	case OpSparkline:
		op.Ext.(*opSparkline).render(t, buf, absX, absY, contentW, geom.H)

	case OpHRule:
		ext := op.Ext.(*opRule)
		width := int(maxW)
		if contentW > 0 {
			width = int(contentW)
		}
		hBaseStyle := ext.style
		if ext.stylePtr != nil {
			hBaseStyle = *ext.stylePtr
		}
		ruleStyle := t.effectiveStyle(hBaseStyle)
		for i := 0; i < width; i++ {
			buf.Set(int(absX)+i, int(absY), Cell{Rune: ext.char, Style: ruleStyle})
		}

	case OpVRule:
		ext := op.Ext.(*opRule)
		vBaseStyle := ext.style
		if ext.stylePtr != nil {
			vBaseStyle = *ext.stylePtr
		}
		ruleStyle := t.effectiveStyle(vBaseStyle)
		for i := 0; i < int(contentH); i++ {
			buf.Set(int(absX), int(absY)+i, Cell{Rune: ext.char, Style: ruleStyle})
		}

	case OpSpacer:
		ext := op.Ext.(*opRule)
		sBaseStyle := ext.style
		if ext.stylePtr != nil {
			sBaseStyle = *ext.stylePtr
		}
		spacerStyle := mergeStyle(sBaseStyle)
		if ext.char != 0 {
			for x := int16(0); x < contentW; x++ {
				buf.Set(int(absX+x), int(absY), Cell{Rune: ext.char, Style: spacerStyle})
			}
		} else if t.rowBG.Mode != 0 {
			for x := int16(0); x < contentW; x++ {
				buf.Set(int(absX+x), int(absY), Cell{Rune: ' ', Style: spacerStyle})
			}
		}

	case OpSpinner:
		ext := op.Ext.(*opSpinner)
		if frameIdx, ok := ext.frameIndex(t); ok {
			frame := ext.frames[frameIdx]
			spinBaseStyle := ext.style
			if ext.stylePtr != nil {
				spinBaseStyle = *ext.stylePtr
			}
			style := t.effectiveStyle(spinBaseStyle)
			buf.WriteStringFast(int(absX), int(absY), frame, style, 1)
		}

	case OpScrollbar:
		t.renderScrollbar(buf, op, geom, absX, absY)

	case OpTabs:
		t.renderTabs(buf, op, geom, absX, absY)

	case OpTreeView:
		t.renderTreeView(buf, op, absX, absY)

	case OpSelectionList:
		t.renderSelectionList(buf, op, geom, absX, absY, maxW)

	case OpJump:
		t.renderJump(buf, op, geom, absX, absY, maxW, idx)

	case OpTextInput:
		t.renderTextInput(buf, op, geom, absX, absY)

	case OpOverlay:
		t.pendingOverlays = append(t.pendingOverlays, pendingOverlay{op: op})

	case OpScreenEffect:
		ext := op.Ext.(*opScreenEffect)
		t.pendingScreenEffects = append(t.pendingScreenEffects, ext.fns...)

	case OpCustom:
		t.renderCustomRenderer(buf, op, absX, absY, contentW, contentH)

	case OpLayout:
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			t.renderSubOp(buf, i, absX, absY, contentW, elemBase)
		}

	case OpLayer:
		t.renderLayer(buf, op, absX, absY, contentW, contentH)

	case OpContainer:
		// Margin inset: visible box starts inside the margin
		boxX := absX + op.Margin[3]
		boxY := absY + op.Margin[0]
		boxW := geom.W - op.marginH()
		boxH := geom.H - op.marginV()

		opacity, hasOpacity := t.opacityForOp(op)
		effectiveRefOpacity := t.currentRefOpacity()
		if hasOpacity {
			effectiveRefOpacity *= opacity
		}
		if op.NodeRef != nil {
			op.NodeRef.X = int(boxX)
			op.NodeRef.Y = int(boxY)
			op.NodeRef.W = int(boxW)
			op.NodeRef.H = int(boxH)
			op.NodeRef.Opacity = effectiveRefOpacity
			op.NodeRef.opacitySet = true
		}
		var backing []Cell
		if hasOpacity && opacity < 1 {
			backing = t.snapshotRect(buf, int(boxX), int(boxY), int(boxW), int(boxH))
		}

		// Update inherited Fill - cascades through nested containers
		oldInheritedFill := t.inheritedFill
		opFill := op.fillFor(elemBase)
		if op.CascadeStyle != nil && op.CascadeStyle.Fill.Mode != ColorDefault {
			t.inheritedFill = op.CascadeStyle.Fill
		} else if opFill.Mode != ColorDefault {
			t.inheritedFill = opFill
		}

		// Update inherited style if this container sets one (before title rendering)
		oldInheritedStyle := t.inheritedStyle
		if op.CascadeStyle != nil {
			t.inheritedStyle = op.CascadeStyle
		}

		// Fill container area - direct Fill takes precedence over inherited
		fillColor := t.inheritedFill
		if opFill.Mode != ColorDefault {
			fillColor = opFill
		}
		if op.LocalStyle != nil && op.LocalStyle.Fill.Mode != ColorDefault {
			fillColor = op.LocalStyle.Fill
		}
		if fillColor.Mode != ColorDefault {
			fillCell := Cell{Rune: ' ', Style: Style{BG: fillColor}}
			buf.FillRect(int(boxX), int(boxY), int(boxW), int(boxH), fillCell)
		}

		// Draw border if present
		if op.Border.HasBorder() {
			style := DefaultStyle()
			if op.BorderFG != nil {
				style.FG = *op.BorderFG
			}
			if op.LocalStyle != nil && op.LocalStyle.FG.Mode != ColorDefault {
				style.FG = op.LocalStyle.FG
			}
			if op.BorderBG != nil {
				style.BG = *op.BorderBG
			} else if op.LocalStyle != nil && op.LocalStyle.BG.Mode != ColorDefault {
				style.BG = op.LocalStyle.BG
			} else if fillColor.Mode != ColorDefault {
				style.BG = fillColor
			}
			if style.FG.Mode != ColorDefault || op.BorderFG == nil {
				buf.DrawBorder(int(boxX), int(boxY), int(boxW), int(boxH), op.Border, style)
			}

			if op.Title != "" {
				titleTransform := TransformNone
				if t.inheritedStyle != nil {
					titleTransform = t.inheritedStyle.Transform
				}
				titleMaxW := int(boxW) - 2
				titleX := int(boxX) + 1
				if titleMaxW > 0 {
					buf.SetFast(titleX, int(boxY), Cell{Rune: op.Border.topChar(), Style: style})
					titleX++
					buf.SetFast(titleX, int(boxY), Cell{Rune: ' ', Style: style})
					titleX++
					title := applyTransform(op.Title, titleTransform)
					titleW := StringWidth(title)
					availTitleW := titleMaxW - 3 // border char + space before + space after
					if availTitleW > 0 {
						if titleW > availTitleW {
							titleW = availTitleW
						}
						buf.WriteStringFast(titleX, int(boxY), title, style, titleW)
						titleX += titleW
						buf.SetFast(titleX, int(boxY), Cell{Rune: ' ', Style: style})
					}
				}
			}
		}

		// Calculate content width (accounting for margin + border)
		contentW := boxW
		if op.Border.HasBorder() {
			contentW -= op.Border.PadH()
		}

		// Set vertical clip for children (content area bottom)
		oldClipMaxY := t.clipMaxY
		contentBottom := boxY + boxH
		contentBottom -= op.Border.PadBottom()
		if t.clipMaxY == 0 || contentBottom < t.clipMaxY {
			t.clipMaxY = contentBottom
		}

		oldRefOpacity := t.refOpacity
		oldRefOpacitySet := t.refOpacitySet
		t.refOpacity = effectiveRefOpacity
		t.refOpacitySet = true

		// Recurse into children with this container's position as their origin
		// children's LocalX/Y already include margin+border offsets
		for i := op.ChildStart; i < op.ChildEnd; i++ {
			childOp := &t.ops[i]
			if childOp.Parent != idx {
				continue
			}
			t.renderSubOp(buf, i, absX, absY, contentW, elemBase)
		}

		// Restore inherited style, fill, and clip
		t.inheritedStyle = oldInheritedStyle
		t.inheritedFill = oldInheritedFill
		t.clipMaxY = oldClipMaxY
		t.refOpacity = oldRefOpacity
		t.refOpacitySet = oldRefOpacitySet

		if hasOpacity && opacity < 1 {
			t.composeOpacityRect(buf, int(boxX), int(boxY), int(boxW), int(boxH), backing, opacity, op.OpacityMode)
		}

	case OpIf:
		ifExt := op.Ext.(*opIf)
		branches, requested := ifBranches(ifExt, elemBase)
		t.renderSelectedBranch(buf, branches, requested, ifExt.selector(elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), elemBase)

	case OpForEach:
		// Nested ForEach - render with nested element base
		feExt := op.Ext.(*opForEach)
		if feExt.iterTmpl != nil {
			sliceHdr, ok := feExt.sliceHeaderFor(elemBase)
			if !ok {
				return
			}
			for j := 0; j < sliceHdr.Len && j < len(feExt.geoms); j++ {
				itemGeom := &feExt.geoms[j]
				itemAbsX := absX + itemGeom.LocalX
				itemAbsY := absY + itemGeom.LocalY
				nestedElemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(j)*feExt.elemSize)
				if feExt.elemIsPtr {
					nestedElemPtr = *(*unsafe.Pointer)(nestedElemPtr)
				}
				feExt.iterTmpl.bindItemContext(t, nestedElemPtr)
				feExt.iterTmpl.itemIndex = j
				feExt.iterTmpl.runItemEvalsFrom(t, nestedElemPtr)
				feExt.iterTmpl.distributeWidths(itemGeom.W, nestedElemPtr)
				feExt.iterTmpl.layout(0)
				feExt.iterTmpl.clampRootWidth(itemGeom.W)
				t.renderSubTemplate(buf, feExt.iterTmpl, itemAbsX, itemAbsY, itemGeom.W, nestedElemPtr)
			}
		}

	case OpSwitch:
		swExt := op.Ext.(*opSwitch)
		branches, requested := switchBranches(swExt, elemBase)
		t.renderSelectedBranch(buf, branches, requested, swExt.selector(elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), elemBase)

	case OpMatch:
		mExt := op.Ext.(*opMatch)
		branches, requested := matchBranches(mExt, elemBase)
		t.renderSelectedBranch(buf, branches, requested, mExt.selector(elemBase), absX, absY, branchBudget(geom.W, maxW, absX, globalX), elemBase)
	}
}

// renderSelectionList renders a selection list with marker and windowing.
func (t *Template) renderSelectionList(buf *Buffer, op *Op, geom *Geom, absX, absY, maxW int16) {
	ext := op.Ext.(*opSelectionList)
	sliceHdr, ok := ext.sliceHeaderFor(t.elemBase)
	if !ok {
		return
	}
	if sliceHdr.Len == 0 || len(ext.geoms) == 0 {
		return
	}
	visibleLen := sliceHdr.Len
	if visibleLen > len(ext.geoms) {
		visibleLen = len(ext.geoms)
	}
	if visibleLen == 0 {
		return
	}

	selectedIdx := -1
	if ext.selectedPtr != nil {
		selectedIdx = *ext.selectedPtr
		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= visibleLen {
			selectedIdx = visibleLen - 1
		}
	}

	// height-aware windowing: determine visible item range using per-item heights.
	// startIdx always seeds from the persisted offset when there's a list pointer,
	// even when MaxVisible==0 (the viewport is bounded by clipMaxY instead). Without
	// this, an unbounded clipped list resets startIdx to 0 every frame, so the
	// scroll-adjustment below recomputes the window from the selection alone and
	// re-pins the selection to the bottom — making the viewport scroll on every
	// move UP, not just when the cursor reaches the top edge.
	startIdx := 0
	endIdx := visibleLen
	if ext.listPtr != nil {
		startIdx = ext.listPtr.offset
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx >= visibleLen {
			startIdx = visibleLen - 1
		}
		if ext.listPtr.MaxVisible > 0 {
			endIdx = startIdx + ext.listPtr.MaxVisible
			if endIdx > visibleLen {
				endIdx = visibleLen
			}
		}
	}

	availableRows := 0 // viewport rows for the list; 0 means unclipped
	if t.clipMaxY > 0 {
		availableRows = int(t.clipMaxY - absY)
		if availableRows <= 0 {
			return
		}

		// window the rows that FULLY fit, then include one extra partial row at
		// the edge — clipped to the viewport bottom at draw time — so a row that
		// only partly fits peeks in (the "more below" affordance) instead of
		// vanishing. fullEnd is the fully-fitting boundary, used for the
		// scroll-into-view decision so a SELECTED edge row still scrolls fully in.
		rowsUsed := 0
		fullEnd := endIdx
		partial := false
		for ci := startIdx; ci < endIdx; ci++ {
			ih := int(ext.geoms[ci].H)
			if rowsUsed+ih > availableRows {
				fullEnd = ci
				partial = true
				break
			}
			rowsUsed += ih
		}
		endIdx = fullEnd
		if partial && fullEnd < visibleLen {
			endIdx = fullEnd + 1 // render the edge row partially
		}

		// ensure selected item is visible (scroll adjustment)
		if ext.listPtr != nil && selectedIdx >= 0 {
			if selectedIdx < startIdx {
				startIdx = selectedIdx
				// recalculate endIdx forward from new startIdx
				rowsUsed = 0
				endIdx = visibleLen
				for ci := startIdx; ci < visibleLen; ci++ {
					ih := int(ext.geoms[ci].H)
					if rowsUsed+ih > availableRows {
						endIdx = ci
						break
					}
					rowsUsed += ih
				}
				ext.listPtr.offset = startIdx
			} else if selectedIdx >= fullEnd {
				// scroll down: place selected item at the bottom of the window
				endIdx = selectedIdx + 1
				rowsUsed = 0
				startIdx = endIdx
				for ci := endIdx - 1; ci >= 0; ci-- {
					ih := int(ext.geoms[ci].H)
					if rowsUsed+ih > availableRows {
						break
					}
					rowsUsed += ih
					startIdx = ci
				}
				ext.listPtr.offset = startIdx
			}
		}
	}

	// ScrollState writeback: publish the finalized window in SCREEN ROWS (summing each
	// item's measured height), not item counts — rows are what the user sees scroll, so
	// a ScrollbarDyn tracking these matches the visual exactly even when rows have
	// different heights (e.g. multi-line comments). Matches the Layer scrollbar's model.
	if ext.listPtr != nil {
		totalRows, offRows, visRows := 0, 0, 0
		for i := 0; i < visibleLen && i < len(ext.geoms); i++ {
			h := int(ext.geoms[i].H)
			totalRows += h
			if i < startIdx {
				offRows += h
			} else if i < endIdx {
				visRows += h
			}
		}
		if ext.listPtr.scrollTotalPtr != nil {
			*ext.listPtr.scrollTotalPtr = totalRows
		}
		if ext.listPtr.scrollOffsetPtr != nil {
			*ext.listPtr.scrollOffsetPtr = offRows
		}
		// the edge row is clipped to the viewport, so the visible rows never
		// exceed the available height — clamp so a ScrollbarDyn reads right.
		if availableRows > 0 && visRows > availableRows {
			visRows = availableRows
		}
		if ext.listPtr.scrollVisiblePtr != nil {
			*ext.listPtr.scrollVisiblePtr = visRows
		}
	}

	spaces := ext.markerSpaces

	rowW := geom.W
	if maxW > 0 && maxW < rowW {
		rowW = maxW
	}
	contentW := rowW - ext.markerWidth
	if contentW < 0 {
		contentW = 0
	}
	contentX := absX + ext.markerWidth

	needsFullPipeline := false
	if ext.iterTmpl != nil && len(ext.iterTmpl.ops) > 0 {
		firstOp := &ext.iterTmpl.ops[0]
		needsFullPipeline = firstOp.Kind == OpContainer || firstOp.Kind == OpLayout || firstOp.Kind == OpJump || firstOp.Kind == OpRichText || firstOp.Kind == OpTextBlock
	}

	var defaultStyle, selectedStyle, markerBaseStyle Style
	if ext.listPtr != nil {
		defaultStyle = ext.listPtr.Style
		selectedStyle = ext.listPtr.SelectedStyle
		markerBaseStyle = ext.listPtr.MarkerStyle
	}

	// Render visible items using per-item heights from layout phase. Clip the
	// draw to the viewport bottom so the edge row (which may overflow by design)
	// is shown partially rather than overdrawing whatever sits below the list.
	if t.clipMaxY > 0 {
		buf.PushClip(0, int(absY), buf.Width(), int(t.clipMaxY))
		defer buf.PopClip()
	}
	y := int(absY)
	for i := startIdx; i < endIdx; i++ {
		itemH := int(ext.geoms[i].H)
		isSelected := i == selectedIdx

		// fill item area with selection style (covers full item height)
		var rowStyle Style
		if isSelected {
			rowStyle = selectedStyle
		} else if defaultStyle.BG.Mode != 0 {
			rowStyle.BG = defaultStyle.BG
		}
		if rowStyle.BG.Mode != 0 || rowStyle.Attr != 0 {
			buf.FillRect(int(absX), y, int(rowW), itemH, Cell{Rune: ' ', Style: rowStyle})
		}

		// Determine marker text and style
		var markerText string
		markerStyle := markerBaseStyle
		if isSelected {
			markerText = ext.marker
			if markerStyle.BG.Mode == 0 && selectedStyle.BG.Mode != 0 {
				markerStyle.BG = selectedStyle.BG
			}
			if markerStyle.FG.Mode == 0 && selectedStyle.FG.Mode != 0 {
				markerStyle.FG = selectedStyle.FG
			}
		} else {
			markerText = spaces
			if markerStyle.BG.Mode == 0 && defaultStyle.BG.Mode != 0 {
				markerStyle.BG = defaultStyle.BG
			}
		}

		// write marker on first row of the item
		buf.WriteStringFast(int(absX), y, markerText, t.effectiveStyle(markerStyle), int(maxW))

		// Get content from iteration template
		if ext.iterTmpl != nil && len(ext.iterTmpl.ops) > 0 {
			elemPtr := unsafe.Pointer(uintptr(sliceHdr.Data) + uintptr(i)*ext.elemSize)
			if ext.elemIsPtr {
				elemPtr = *(*unsafe.Pointer)(elemPtr)
			}

			if needsFullPipeline {
				// complex layout: use pre-calculated heights from layout phase
				ext.iterTmpl.bindItemContext(t, elemPtr)
				ext.iterTmpl.itemIndex = i
				ext.iterTmpl.runItemEvalsFrom(t, elemPtr)
				ext.iterTmpl.distributeWidths(contentW, elemPtr)
				ext.iterTmpl.layout(0)
				if isSelected {
					ext.iterTmpl.rowBG = selectedStyle.BG
					ext.iterTmpl.rowFG = selectedStyle.FG
					ext.iterTmpl.rowAttr = selectedStyle.Attr
					if ext.iterTmpl.rowBG.Mode == 0 && defaultStyle.BG.Mode != 0 {
						ext.iterTmpl.rowBG = defaultStyle.BG
					}
				} else {
					ext.iterTmpl.rowBG = defaultStyle.BG
					ext.iterTmpl.rowFG = defaultStyle.FG
					ext.iterTmpl.rowAttr = defaultStyle.Attr
				}
				// apply dynamic fills on root container before rendering
				if len(ext.iterTmpl.ops) > 0 {
					rootOp := &ext.iterTmpl.ops[0]
					if rootOp.Kind == OpContainer {
						rootGeom := &ext.iterTmpl.geom[0]
						fillColor := rootOp.fillFor(elemPtr)
						if fillColor.Mode != ColorDefault {
							bx := int(contentX) + int(rootOp.Margin[3])
							by := y + int(rootOp.Margin[0])
							bw := int(rootGeom.W) - int(rootOp.marginH())
							bh := int(rootGeom.H) - int(rootOp.marginV())
							buf.FillRect(bx, by, bw, bh, Cell{Rune: ' ', Style: Style{BG: fillColor}})
						}
					}
				}
				t.renderSubTemplate(buf, ext.iterTmpl, contentX, int16(y), contentW, elemPtr)
			} else {
				// Simple text: fast path (no layout needed)
				ext.iterTmpl.bindItemContext(t, elemPtr)
				ext.iterTmpl.itemIndex = i
				ext.iterTmpl.runItemEvalsFrom(t, elemPtr)
				iterOp := &ext.iterTmpl.ops[0]

				switch iterOp.Kind {
				case OpText:
					ext := iterOp.Ext.(*opText)
					textStyle := ext.style
					if ext.stylePtr != nil {
						textStyle = *ext.stylePtr
					}
					if isSelected {
						if textStyle.BG.Mode == 0 && selectedStyle.BG.Mode != 0 {
							textStyle.BG = selectedStyle.BG
						}
						if textStyle.FG.Mode == 0 && selectedStyle.FG.Mode != 0 {
							textStyle.FG = selectedStyle.FG
						}
						textStyle.Attr = textStyle.Attr | selectedStyle.Attr
					} else {
						if textStyle.BG.Mode == 0 && defaultStyle.BG.Mode != 0 {
							textStyle.BG = defaultStyle.BG
						}
						if textStyle.FG.Mode == 0 && defaultStyle.FG.Mode != 0 {
							textStyle.FG = defaultStyle.FG
						}
						textStyle.Attr = textStyle.Attr | defaultStyle.Attr
					}
					effStyle := t.effectiveStyle(textStyle)
					raw := ext.resolve(elemPtr)
					txt := applyTransform(raw, effStyle.Transform)
					buf.WriteStringFast(int(contentX), y, txt, effStyle, int(contentW))
				case OpRichText:
					ext := iterOp.Ext.(*opRichText)
					spans := ext.resolve(elemPtr)
					if spans != nil {
						buf.WriteSpans(int(contentX), y, spans, int(contentW))
					}
				}
			}
		}
		if isSelected && ext.selectedRef != nil {
			ext.selectedRef.X = int(absX)
			ext.selectedRef.Y = y
			ext.selectedRef.W = int(maxW)
			ext.selectedRef.H = itemH
		}
		y += itemH
	}
}

// treeVisibleCount returns the number of visible nodes in the tree.
func (t *Template) treeVisibleCount(node *TreeNode, includeRoot bool) int {
	if node == nil {
		return 0
	}
	count := 0
	if includeRoot {
		count = 1
	}
	if node.Expanded || !includeRoot {
		for _, child := range node.Children {
			count += t.treeVisibleCount(child, true)
		}
	}
	return count
}

// treeMaxWidth returns the maximum width of visible nodes.
func (t *Template) treeMaxWidth(node *TreeNode, level, indent int, includeRoot bool) int {
	if node == nil {
		return 0
	}
	maxW := 0
	if includeRoot && level >= 0 {
		// 2 for indicator + space, then indent + label
		lineW := 2 + level*indent + StringWidth(node.Label)
		if lineW > maxW {
			maxW = lineW
		}
	}
	if node.Expanded || !includeRoot {
		for _, child := range node.Children {
			childW := t.treeMaxWidth(child, level+1, indent, true)
			if childW > maxW {
				maxW = childW
			}
		}
	}
	return maxW
}

func (t *Template) renderTreeView(buf *Buffer, op *Op, absX, absY int16) {
	ext := op.Ext.(*opTreeView)
	if ext.root == nil {
		return
	}
	y := int(absY)
	t.renderTreeNode(buf, ext, ext.root, int(absX), &y, 0, ext.showRoot, nil)
}

func (t *Template) renderTreeNode(buf *Buffer, ext *opTreeView, node *TreeNode, x int, y *int, level int, render bool, linePrefix []bool) {
	if node == nil {
		return
	}

	if render && level >= 0 {
		posX := x
		if ext.showLines && level > 0 {
			for i := 0; i < level; i++ {
				if i < len(linePrefix) && linePrefix[i] {
					buf.Set(posX, *y, Cell{Rune: '│', Style: ext.style})
				}
				posX += ext.indent
			}
		} else {
			posX += level * ext.indent
		}

		var indicator rune
		if len(node.Children) > 0 {
			if node.Expanded {
				indicator = ext.expandedChar
			} else {
				indicator = ext.collapsedChar
			}
		} else {
			indicator = ext.leafChar
		}
		buf.Set(posX, *y, Cell{Rune: indicator, Style: ext.style})
		posX++
		buf.Set(posX, *y, Cell{Rune: ' ', Style: ext.style})
		posX++

		effStyle := t.effectiveStyle(ext.style)
		labelText := applyTransform(node.Label, effStyle.Transform)
		buf.WriteStringFast(posX, *y, labelText, ext.style, StringWidth(labelText))
		(*y)++
	}

	if node.Expanded || !render {
		childCount := len(node.Children)
		for i, child := range node.Children {
			for len(t.treeScratchPfx) <= level {
				t.treeScratchPfx = append(t.treeScratchPfx, false)
			}
			if level >= 0 {
				t.treeScratchPfx[level] = i < childCount-1
			}
			t.renderTreeNode(buf, ext, child, x, y, level+1, true, t.treeScratchPfx)
		}
	}
}

// renderJump renders a jump target and its children.
func (t *Template) renderJump(buf *Buffer, op *Op, geom *Geom, absX, absY, maxW int16, idx int16) {
	// Render children first
	for i := op.ChildStart; i < op.ChildEnd; i++ {
		childOp := &t.ops[i]
		if childOp.Parent == idx {
			t.renderOp(buf, i, absX, absY, maxW)
		}
	}

	// If jump mode is active, register this target. Labels are painted by the
	// app after the final visible frame is composed. Targets outside the
	// buffer, the active vertical clip, or the jump viewport never drew
	// anything visible (the span jump path filters the same way), so they
	// must not register either. Layer offsets translate buffer-local
	// positions to screen positions.
	if t.app != nil && t.app.JumpModeActive() {
		if absY < 0 || int(absY) >= buf.Height() || absX < 0 || int(absX) >= buf.Width() {
			return
		}
		if t.clipMaxY > 0 && absY >= t.clipMaxY {
			return
		}
		x := int(absX) + t.jumpOffsetX
		y := int(absY) + t.jumpOffsetY
		if t.jumpMaxY > t.jumpMinY && (y < t.jumpMinY || y >= t.jumpMaxY) {
			return
		}
		ext := op.Ext.(*opJump)
		onSelect := ext.onSelect
		if base := t.elemBase; base != nil {
			if ext.onSelectItem != nil {
				fn := ext.onSelectItem
				onSelect = func() { fn(base) }
			} else if ext.onSelectItemRef != nil {
				fn := ext.onSelectItemRef
				w := geom.W
				if maxW < w {
					w = maxW
				}
				ref := NodeRef{X: x, Y: y, W: int(w), H: max(1, int(geom.H))}
				onSelect = func() { fn(base, ref) }
			}
		}
		t.app.AddJumpTarget(int16(x), int16(y), onSelect, ext.style)
	}
}

func (t *Template) richSpanJumpFunc(buf *Buffer) spanJumpFunc {
	if t.app == nil || !t.app.JumpModeActive() {
		return nil
	}
	return func(x, y int, span Span) {
		if span.OnSelect == nil && span.OnSelectRef == nil {
			return
		}
		x += t.jumpOffsetX
		y += t.jumpOffsetY
		if t.jumpMaxY > t.jumpMinY && (y < t.jumpMinY || y >= t.jumpMaxY) {
			return
		}
		onSelect := span.OnSelect
		if span.OnSelectRef != nil {
			ref := NodeRef{X: x, Y: y, W: max(1, StringWidth(span.Text)), H: 1}
			onSelect = func() { span.OnSelectRef(ref) }
		}
		t.app.AddJumpTarget(int16(x), int16(y), onSelect, Style{})
	}
}

func (t *Template) renderTextInput(buf *Buffer, op *Op, geom *Geom, absX, absY int16) {
	width := int(geom.W)
	if width <= 0 {
		return
	}

	ext := op.Ext.(*opTextInput)

	// Resolve styles through the cascade so inheritedFill applies as BG
	textStyle := t.effectiveStyle(ext.style)
	placeholderStyle := t.effectiveStyle(ext.placeholderSty)
	cursorStyle := t.effectiveStyle(ext.cursorStyle)

	// Get value and cursor - prefer Field API, fall back to pointer API
	var value string
	var cursor int
	if ext.fieldPtr != nil {
		if ext.syncBound && ext.valuePtr != nil {
			bound := *ext.valuePtr
			if bound != ext.lastBoundValue && bound != ext.fieldPtr.Value {
				ext.fieldPtr.Value = bound
				ext.fieldPtr.Cursor = len(bound)
			}
			ext.lastBoundValue = bound
		}
		value = ext.fieldPtr.Value
		cursor = ext.fieldPtr.Cursor
	} else {
		if ext.valuePtr != nil {
			value = *ext.valuePtr
		}
		cursor = len(value) // default to end
		if ext.cursorPtr != nil {
			cursor = *ext.cursorPtr
		}
	}

	// Clamp cursor to valid range
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(value) {
		cursor = len(value)
	}

	// Determine if cursor should be shown
	// Priority: FocusGroup > Focused > always show
	var showCursor bool
	if ext.focusGroupPtr != nil {
		showCursor = ext.focusGroupPtr.Current == ext.focusIndex
	} else if ext.focusedPtr != nil {
		showCursor = *ext.focusedPtr
	} else {
		// Default: show cursor if we have cursor tracking
		showCursor = ext.fieldPtr != nil || ext.cursorPtr != nil
	}

	// Handle empty state with placeholder
	if value == "" {
		if ext.placeholder != "" {
			buf.WriteStringFast(int(absX), int(absY), ext.placeholder, placeholderStyle, width)
		}
		// Draw cursor at start if focused
		if showCursor {
			// use first placeholder character under cursor so it remains visible
			cursorRune := ' '
			if ext.placeholder != "" {
				cursorRune = []rune(ext.placeholder)[0]
			}
			buf.Set(int(absX), int(absY), Cell{Rune: cursorRune, Style: cursorStyle})
		}
		return
	}

	// Apply mask if set
	displayValue := value
	if ext.mask != 0 {
		runes := make([]rune, len([]rune(value)))
		for i := range runes {
			runes[i] = ext.mask
		}
		displayValue = string(runes)
	}

	// Calculate scroll offset for horizontal scrolling
	// Keep cursor visible within the field
	displayRunes := []rune(displayValue)
	cursorRune := cursor
	if cursorRune > len(displayRunes) {
		cursorRune = len(displayRunes)
	}

	// multiline: wrap the value across lines instead of scrolling one line horizontally.
	if ext.multiline {
		lines := inputWrapLines(displayRunes, width)
		curLine, curCol := inputCursorPos(lines, cursorRune)
		for li, ln := range lines {
			y := int(absY) + li
			x := int(absX)
			for ri := ln.start; ri < ln.end; ri++ {
				style := textStyle
				if showCursor && ri == cursorRune {
					style = cursorStyle
				}
				buf.Set(x, y, Cell{Rune: displayRunes[ri], Style: style})
				x++
			}
		}
		// cursor sitting past the last character (its line has no rune to highlight)
		if showCursor && cursorRune >= len(displayRunes) {
			buf.Set(int(absX)+curCol, int(absY)+curLine, Cell{Rune: ' ', Style: cursorStyle})
		}
		return
	}

	scrollOffset := 0
	if showCursor && cursorRune >= width {
		scrollOffset = cursorRune - width + 1
	}

	// Render visible portion
	visibleEnd := scrollOffset + width
	if visibleEnd > len(displayRunes) {
		visibleEnd = len(displayRunes)
	}

	x := int(absX)
	for i := scrollOffset; i < visibleEnd; i++ {
		style := textStyle
		// Highlight cursor position if focused
		if showCursor && i == cursorRune {
			style = cursorStyle
		}
		buf.Set(x, int(absY), Cell{Rune: displayRunes[i], Style: style})
		x++
	}

	// If cursor is at end (after last char), draw cursor there
	if showCursor && cursorRune >= len(displayRunes) && cursorRune-scrollOffset < width {
		buf.Set(int(absX)+cursorRune-scrollOffset, int(absY), Cell{Rune: ' ', Style: cursorStyle})
	}
}

// ScreenEffects returns the post-processing passes collected from the tree
// during the most recent Execute. The returned slice is reused between frames.
func (t *Template) ScreenEffects() []Effect {
	return t.pendingScreenEffects
}

// renderOverlays renders all collected overlays after main content.
func (t *Template) renderOverlays(buf *Buffer, screenW, screenH int16) {
	for _, po := range t.pendingOverlays {
		t.renderOverlay(buf, po.op, po.exiting, screenW, screenH)
	}
}

// renderOverlay renders a single overlay to the buffer. When exiting (its branch's
// condition went false and it's only animating out), its modal router is released rather
// than re-pushed, so a fading overlay can't orphan its router on the input stack.
func (t *Template) renderOverlay(buf *Buffer, op *Op, exiting bool, screenW, screenH int16) {
	ext := op.Ext.(*opOverlay)
	if ext.childTmpl == nil {
		return
	}

	// Link app to child template for jump mode support
	ext.childTmpl.app = t.app

	// Propagate overlay BG as inheritedFill so all child text cells render with
	// the same explicit background, preventing patchy backdrop bleed-through
	if ext.bg.Mode != ColorDefault {
		ext.childTmpl.inheritedFill = ext.bg
	}

	// Calculate content size by doing a dry-run layout
	childTmpl := ext.childTmpl

	// Determine overlay dimensions
	overlayW := op.width()
	overlayH := op.height()

	if overlayW == 0 || overlayH == 0 {
		// Calculate natural size from content
		// DON'T call distributeFlexGrow - overlays should size to content, not expand
		childTmpl.distributeWidths(screenW, nil)
		childTmpl.layout(screenH)

		// Get root content size (natural height, no flex grow distribution)
		if len(childTmpl.geom) > 0 {
			if overlayW == 0 {
				overlayW = childTmpl.geom[0].W
			}
			if overlayH == 0 {
				overlayH = childTmpl.geom[0].H
			}
		}
	}

	// Calculate position
	var posX, posY int16
	if ext.anchor != nil {
		ref := ext.anchor
		switch ext.anchorPos {
		case AnchorBelow:
			posX = int16(ref.X)
			posY = int16(ref.Y + ref.H)
			if overlayW == 0 {
				overlayW = int16(ref.W)
			}
		case AnchorAbove:
			posX = int16(ref.X)
			posY = int16(ref.Y) - overlayH
			if overlayW == 0 {
				overlayW = int16(ref.W)
			}
		case AnchorOnTop:
			posX = int16(ref.X)
			posY = int16(ref.Y)
			if overlayW == 0 {
				overlayW = int16(ref.W)
			}
			if overlayH == 0 {
				overlayH = int16(ref.H)
			}
		case AnchorRightOf:
			posX = int16(ref.X + ref.W)
			posY = int16(ref.Y)
		case AnchorLeftOf:
			posX = int16(ref.X) - overlayW
			posY = int16(ref.Y)
		}
	} else {
		switch ext.placement {
		case OverlayPlacementCentered:
			posX = (screenW - overlayW) / 2
			posY = (screenH - overlayH) / 2
		case OverlayPlacementTop:
			posX = (screenW - overlayW) / 2
			posY = 0
		case OverlayPlacementBottom:
			posX = (screenW - overlayW) / 2
			posY = screenH - overlayH
		case OverlayPlacementLeft:
			posX = 0
			posY = (screenH - overlayH) / 2
		case OverlayPlacementRight:
			posX = screenW - overlayW
			posY = (screenH - overlayH) / 2
		case OverlayPlacementTopLeft:
			posX = 0
			posY = 0
		case OverlayPlacementTopRight:
			posX = screenW - overlayW
			posY = 0
		case OverlayPlacementBottomLeft:
			posX = 0
			posY = screenH - overlayH
		case OverlayPlacementBottomRight:
			posX = screenW - overlayW
			posY = screenH - overlayH
		default:
			posX = ext.x
			posY = ext.y
		}
	}
	if ext.offsetX != nil {
		posX += *ext.offsetX
	}
	if ext.offsetY != nil {
		posY += *ext.offsetY
	}

	// Clamp to screen bounds
	if posX < 0 {
		posX = 0
	}
	if posY < 0 {
		posY = 0
	}

	// Draw backdrop if enabled
	if ext.backdrop {
		for y := int16(0); y < screenH; y++ {
			for x := int16(0); x < screenW; x++ {
				cell := buf.Get(int(x), int(y))
				// Dim existing content - preserve background, only modify FG and attr
				cell.Style.FG = ext.backdropFG
				cell.Style.Attr = AttrDim
				buf.Set(int(x), int(y), cell)
			}
		}
	}

	opacity := 1.0
	hasOpacity := ext.opacity != nil
	if hasOpacity {
		if ext.opacityArmed != nil {
			*ext.opacityArmed = true
		}
		opacity = clampOpacity(*ext.opacity)
	}
	var backing []Cell
	if hasOpacity && opacity < 1 {
		backing = t.snapshotRect(buf, int(posX), int(posY), int(overlayW), int(overlayH))
	}

	// Fill overlay content area with background color if set
	if ext.bg.Mode != ColorDefault {
		bgStyle := Style{BG: ext.bg}
		for y := posY; y < posY+overlayH && y < screenH; y++ {
			for x := posX; x < posX+overlayW && x < screenW; x++ {
				buf.Set(int(x), int(y), Cell{Rune: ' ', Style: bgStyle})
			}
		}
	}

	// Render the overlay content
	// Re-layout with actual available space
	childTmpl.pendingScreenEffects = childTmpl.pendingScreenEffects[:0]
	// release the modal router while the overlay fades out (exiting), otherwise re-pushing
	// it here every frame leaves it orphaned once the fade ends and renderOverlay stops.
	childTmpl.setRouteActive(!exiting)
	childTmpl.distributeWidths(overlayW, nil)
	childTmpl.layout(overlayH)
	childTmpl.distributeFlexGrow(overlayH)
	oldChildRefOpacity := childTmpl.refOpacity
	oldChildRefOpacitySet := childTmpl.refOpacitySet
	childTmpl.refOpacity = t.currentRefOpacity() * opacity
	childTmpl.refOpacitySet = true
	childTmpl.render(buf, posX, posY, overlayW)
	childTmpl.refOpacity = oldChildRefOpacity
	childTmpl.refOpacitySet = oldChildRefOpacitySet

	if hasOpacity && opacity < 1 {
		t.composeOpacityRect(buf, int(posX), int(posY), int(overlayW), int(overlayH), backing, opacity, ext.opacityMode)
	}

	// bubble screen effects declared inside overlay up to the parent so they
	// run as full-screen passes after all content (including this overlay) is rendered
	t.pendingScreenEffects = append(t.pendingScreenEffects, childTmpl.pendingScreenEffects...)
}

func (t *Template) renderTabs(buf *Buffer, op *Op, geom *Geom, absX, absY int16) {
	ext := op.Ext.(*opTabs)
	selectedIdx := 0
	if p := ext.selectedPtr.ptrFor(t.elemBase); p != nil {
		selectedIdx = *(*int)(p)
	}

	x := int(absX)
	y := int(absY)

	for i, label := range ext.labels {
		isSelected := i == selectedIdx
		style := t.effectiveStyle(ext.inactiveStyle)
		if isSelected {
			style = t.effectiveStyle(ext.activeStyle)
		}

		// apply transform to label text
		label = applyTransform(label, style.Transform)
		labelLen := StringWidth(label)

		switch ext.styleType {
		case TabsStyleBox:
			// Draw box around tab
			// Top border
			buf.Set(x, y, Cell{Rune: '┌', Style: style})
			for j := 0; j < labelLen+2; j++ {
				buf.Set(x+1+j, y, Cell{Rune: '─', Style: style})
			}
			buf.Set(x+labelLen+3, y, Cell{Rune: '┐', Style: style})
			// Content
			buf.Set(x, y+1, Cell{Rune: '│', Style: style})
			buf.Set(x+1, y+1, Cell{Rune: ' ', Style: style})
			buf.WriteStringFast(x+2, y+1, label, style, labelLen)
			buf.Set(x+labelLen+2, y+1, Cell{Rune: ' ', Style: style})
			buf.Set(x+labelLen+3, y+1, Cell{Rune: '│', Style: style})
			// Bottom border
			buf.Set(x, y+2, Cell{Rune: '└', Style: style})
			for j := 0; j < labelLen+2; j++ {
				buf.Set(x+1+j, y+2, Cell{Rune: '─', Style: style})
			}
			buf.Set(x+labelLen+3, y+2, Cell{Rune: '┘', Style: style})
			x += labelLen + 4 + ext.gap

		case TabsStyleBracket:
			buf.Set(x, y, Cell{Rune: '[', Style: style})
			buf.WriteStringFast(x+1, y, label, style, labelLen)
			buf.Set(x+1+labelLen, y, Cell{Rune: ']', Style: style})
			x += labelLen + 2 + ext.gap

		default: // TabsStyleUnderline
			if isSelected {
				// Write label with underline attribute
				underlineStyle := style
				underlineStyle.Attr = underlineStyle.Attr.With(AttrUnderline)
				buf.WriteStringFast(x, y, label, underlineStyle, labelLen)
			} else {
				buf.WriteStringFast(x, y, label, style, labelLen)
			}
			x += labelLen + ext.gap
		}
	}
}

func (t *Template) renderScrollbar(buf *Buffer, op *Op, geom *Geom, absX, absY int16) {
	ext := op.Ext.(*opScrollbar)
	trackStyle := ext.trackStyle
	if ext.trackStylePtr != nil {
		trackStyle = *ext.trackStylePtr
	}
	thumbStyle := ext.thumbStyle
	if ext.thumbStylePtr != nil {
		thumbStyle = *ext.thumbStylePtr
	}
	length := int(geom.H)
	if ext.horizontal {
		length = int(geom.W)
	}
	if length == 0 {
		return
	}
	opacity, hasOpacity := t.opacityForOp(op)

	pos := 0
	if p := ext.posPtr.ptrFor(t.elemBase); p != nil {
		pos = *(*int)(p)
	}
	contentSize := ext.contentSize
	viewSize := ext.viewSize
	if p := ext.contentPtr.ptrFor(t.elemBase); p != nil {
		contentSize = *(*int)(p)
	}
	if p := ext.viewPtr.ptrFor(t.elemBase); p != nil {
		viewSize = *(*int)(p)
	}
	if ext.layer != nil {
		pos = ext.layer.ScrollY()
		contentSize = ext.layer.ContentHeight()
		viewSize = ext.layer.ViewportHeight()
	}
	if contentSize <= 0 {
		contentSize = 1
	}
	if viewSize <= 0 {
		viewSize = 1
	}

	trackUnits := length * 8
	thumbUnits := (viewSize * trackUnits) / contentSize
	if thumbUnits < 1 {
		thumbUnits = 1
	}
	if thumbUnits > trackUnits {
		thumbUnits = trackUnits
	}
	maxThumbStart := trackUnits - thumbUnits
	scrollRange := contentSize - viewSize
	thumbStart := 0
	if scrollRange > 0 && maxThumbStart > 0 {
		thumbStart = (pos * maxThumbStart) / scrollRange
	}
	if thumbStart < 0 {
		thumbStart = 0
	}
	if thumbStart > maxThumbStart {
		thumbStart = maxThumbStart
	}
	thumbEnd := thumbStart + thumbUnits

	for i := 0; i < length; i++ {
		cellStart := i * 8
		cellEnd := cellStart + 8
		covered := min(cellEnd, thumbEnd) - max(cellStart, thumbStart)
		char := ext.trackChar
		style := trackStyle
		if covered > 0 {
			if covered > 8 {
				covered = 8
			}
			char = scrollbarThumbRune(covered, cellEnd > thumbEnd, ext.thumbChar, ext.horizontal)
			style = thumbStyle
		}
		cell := Cell{Rune: char, Style: style}
		if ext.horizontal {
			if hasOpacity && opacity < 1 {
				buf.SetOpacity(int(absX)+i, int(absY), cell, opacity, op.OpacityMode)
			} else {
				buf.Set(int(absX)+i, int(absY), cell)
			}
		} else {
			if hasOpacity && opacity < 1 {
				buf.SetOpacity(int(absX), int(absY)+i, cell, opacity, op.OpacityMode)
			} else {
				buf.Set(int(absX), int(absY)+i, cell)
			}
		}
	}
}

var scrollbarLowerBlocks = [...]rune{'│', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
var scrollbarUpperBlocks = [...]rune{'│', '▔', '🮂', '🮃', '▀', '🮄', '🮅', '🮆', '█'}
var scrollbarLeftBlocks = [...]rune{'─', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

func scrollbarThumbRune(covered int, trailing bool, fallback rune, horizontal bool) rune {
	if covered >= 8 {
		return fallback
	}
	if horizontal {
		return scrollbarLeftBlocks[covered]
	}
	if trailing {
		return scrollbarUpperBlocks[covered]
	}
	return scrollbarLowerBlocks[covered]
}

func (t *Template) writeTableCell(buf *Buffer, x, y int, text string, width int, align Align, style Style) {
	textLen := StringWidth(text)
	if textLen > width {
		runes := []rune(text)
		text = string(runes[:width])
		textLen = StringWidth(text)
	}

	padding := width - textLen
	var leftPad, rightPad int
	switch align {
	case AlignRight:
		leftPad = padding
	case AlignCenter:
		leftPad = padding / 2
		rightPad = padding - leftPad
	default:
		rightPad = padding
	}

	pos := x
	for i := 0; i < leftPad; i++ {
		buf.Set(pos, y, Cell{Rune: ' ', Style: style})
		pos++
	}
	for _, r := range text {
		buf.Set(pos, y, Cell{Rune: r, Style: style})
		pos++
	}
	for i := 0; i < rightPad; i++ {
		buf.Set(pos, y, Cell{Rune: ' ', Style: style})
		pos++
	}
}

func (t *Template) renderAutoTable(buf *Buffer, op *Op, absX, absY, maxW int16) {
	ext := op.Ext.(*opAutoTable)
	if ext.slicePtr == nil {
		return
	}

	rv := reflect.ValueOf(ext.slicePtr).Elem()
	nRows := rv.Len()
	nCols := len(ext.fields)
	gap := int(ext.gap)

	// re-apply sort if active (keeps data consistent after mutations)
	if ss := ext.sort; ss != nil && ss.col >= 0 && ss.col < nCols {
		autoTableSort(ext.slicePtr, ext.fields[ss.col], ss.asc)
	}

	// compute natural column widths from current data
	// if sorting is enabled, reserve space for the indicator on every header
	// since the user can cycle to any column
	indicatorW := 0
	if ext.sort != nil {
		indicatorW = 2 // " ▲" or " ▼"
	}
	widths := make([]int, nCols)
	for i, h := range ext.headers {
		widths[i] = len(h) + indicatorW
	}

	for i := 0; i < nRows; i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		for j, fi := range ext.fields {
			val := elem.Field(fi).Interface()
			var str string
			if cfg := ext.colCfgs[j]; cfg != nil && cfg.format != nil {
				str = cfg.format(val)
			} else {
				str = fmt.Sprintf("%v", val)
			}
			if len(str) > widths[j] {
				widths[j] = len(str)
			}
		}
	}

	// distribute remaining width proportionally to natural column widths
	availW := int(maxW) - int(absX)
	totalNatural := 0
	for _, w := range widths {
		totalNatural += w
	}
	totalGaps := gap * (nCols - 1)
	remaining := availW - totalNatural - totalGaps

	if remaining > 0 && totalNatural > 0 {
		for i, w := range widths {
			extra := remaining * w / totalNatural
			widths[i] += extra
		}
	}

	hdrStyle := t.effectiveStyle(ext.hdrStyle)
	y := int(absY)

	// header row
	x := int(absX)
	jumpActive := ext.sort != nil && t.app != nil && t.app.JumpModeActive()

	for i, h := range ext.headers {
		text := applyTransform(h, hdrStyle.Transform)
		if ext.sort != nil && ext.sort.col == i {
			if ext.sort.asc {
				text += " ▲"
			} else {
				text += " ▼"
			}
		}
		hdrAlign := AlignLeft
		if cfg := ext.colCfgs[i]; cfg != nil {
			hdrAlign = cfg.align
		}
		t.writeTableCell(buf, x, y, text, widths[i], hdrAlign, hdrStyle)

		// register column header as a jump target for sorting
		if jumpActive {
			colIdx := i
			fieldIdx := ext.fields[i]
			ss := ext.sort
			slicePtr := ext.slicePtr
			t.app.AddJumpTarget(int16(x), int16(y), func() {
				if ss.col == colIdx {
					ss.asc = !ss.asc
				} else {
					ss.col = colIdx
					ss.asc = true
				}
				autoTableSort(slicePtr, fieldIdx, ss.asc)
			}, Style{})

			// draw jump label if assigned (second render pass)
			jm := t.app.JumpMode()
			for j := len(jm.Targets) - 1; j >= 0; j-- {
				target := &jm.Targets[j]
				if target.X == int16(x) && target.Y == int16(y) && target.Label != "" {
					style := t.app.JumpStyle().LabelStyle
					for k, r := range target.Label {
						buf.Set(x+k, y, Cell{Rune: r, Style: style})
					}
					break
				}
			}
		}

		x += widths[i] + gap
	}
	y++

	// data rows -- when scrolling is enabled, render all rows to an internal
	// buffer and blit only the visible viewport to the screen buffer.
	sc := ext.scroll
	if sc != nil {
		sc.clamp(nRows)

		// allocate or resize internal buffer (width = availW, height = nRows)
		if sc.buf == nil || sc.bufW != availW || sc.buf.Height() < nRows {
			sc.buf = NewBuffer(availW, nRows)
			sc.bufW = availW
		} else {
			sc.buf.Clear()
		}

		// render all data rows into internal buffer at y=0..nRows-1
		for i := 0; i < nRows; i++ {
			elem := rv.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}

			rowStyle := t.effectiveStyle(ext.rowStyle)
			isAlt := ext.altStyle != nil && i%2 == 1
			if isAlt {
				rowStyle = t.effectiveStyle(*ext.altStyle)
			}

			if isAlt && ext.fill.Mode != ColorDefault {
				for fx := 0; fx < availW; fx++ {
					sc.buf.Set(fx, i, Cell{Rune: ' ', Style: Style{BG: ext.fill}})
				}
			}

			bx := 0
			for j, fi := range ext.fields {
				val := elem.Field(fi).Interface()
				cfg := ext.colCfgs[j]

				var str string
				if cfg != nil && cfg.format != nil {
					str = cfg.format(val)
				} else {
					str = fmt.Sprintf("%v", val)
				}

				cellStyle := rowStyle
				if cfg != nil && cfg.style != nil {
					cellStyle = cfg.style(val)
				}

				cellAlign := AlignLeft
				if cfg != nil {
					cellAlign = cfg.align
				}

				t.writeTableCell(sc.buf, bx, i, str, widths[j], cellAlign, cellStyle)
				bx += widths[j] + gap
			}
		}

		// blit visible window from internal buffer to screen
		visH := sc.maxVisible
		if visH > nRows {
			visH = nRows
		}
		buf.Blit(sc.buf, 0, sc.offset, int(absX), y, availW, visH)
	} else {
		// no scroll -- render directly (backwards compatible)
		for i := 0; i < nRows; i++ {
			elem := rv.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}

			rowStyle := t.effectiveStyle(ext.rowStyle)
			isAlt := ext.altStyle != nil && i%2 == 1
			if isAlt {
				rowStyle = t.effectiveStyle(*ext.altStyle)
			}

			// fill entire row background for alt rows
			if isAlt && ext.fill.Mode != ColorDefault {
				for fx := int(absX); fx < int(maxW); fx++ {
					buf.Set(fx, y, Cell{Rune: ' ', Style: Style{BG: ext.fill}})
				}
			}

			x = int(absX)
			for j, fi := range ext.fields {
				val := elem.Field(fi).Interface()
				cfg := ext.colCfgs[j]

				var str string
				if cfg != nil && cfg.format != nil {
					str = cfg.format(val)
				} else {
					str = fmt.Sprintf("%v", val)
				}

				cellStyle := rowStyle
				if cfg != nil && cfg.style != nil {
					cellStyle = cfg.style(val)
				}

				cellAlign := AlignLeft
				if cfg != nil {
					cellAlign = cfg.align
				}

				t.writeTableCell(buf, x, y, str, widths[j], cellAlign, cellStyle)
				x += widths[j] + gap
			}
			y++
		}
	}
}

// Height returns the computed height after layout.
// Must call Execute first.
func (t *Template) Height() int16 {
	if len(t.geom) == 0 {
		return 0
	}
	// Find root-level ops and sum their heights
	var totalH int16
	for i, op := range t.ops {
		if op.Parent == -1 {
			totalH += t.geom[i].H
		}
	}
	return totalH
}

// DebugDump prints the template's op tree for debugging layout issues.
func (t *Template) DebugDump(prefix string) {
	fmt.Fprintf(os.Stderr, "%s=== Template Debug (%d ops) ===\n", prefix, len(t.ops))
	for i, op := range t.ops {
		geom := Geom{}
		if i < len(t.geom) {
			geom = t.geom[i]
		}
		kindStr := opKindName(op.Kind)
		flags := ""
		if op.ContentSized {
			flags += " [ContentSized]"
		}
		if op.FlexGrow > 0 {
			flags += fmt.Sprintf(" [Flex:%.1f]", op.FlexGrow)
		}
		if op.Width > 0 {
			flags += fmt.Sprintf(" [W:%d]", op.Width)
		}
		fmt.Fprintf(os.Stderr, "%s  [%d] %s parent=%d geom={W:%d H:%d}%s\n",
			prefix, i, kindStr, op.Parent, geom.W, geom.H, flags)

		// Dump sub-templates for If
		if op.Kind == OpIf {
			ifExt := op.Ext.(*opIf)
			if ifExt.thenTmpl != nil {
				ifExt.thenTmpl.DebugDump(prefix + "    Then: ")
			}
			if ifExt.elseTmpl != nil {
				ifExt.elseTmpl.DebugDump(prefix + "    Else: ")
			}
		}
	}
}

func opKindName(k OpKind) string {
	names := map[OpKind]string{
		OpText: "Text", OpProgress: "Progress", OpRichText: "RichText",
		OpLeader: "Leader", OpCounter: "Counter",
		OpContainer: "Container", OpIf: "If", OpForEach: "ForEach", OpSwitch: "Switch", OpMatch: "Match",
		OpCustom: "Custom", OpLayout: "Layout", OpLayer: "Layer",
		OpSelectionList: "selectionList",
		OpAutoTable:     "AutoTable", OpSparkline: "Sparkline",
		OpHRule: "HRule", OpVRule: "VRule", OpSpacer: "Spacer",
		OpSpinner: "Spinner", OpScrollbar: "Scrollbar", OpTabs: "Tabs", OpTreeView: "TreeView",
		OpJump: "Jump", OpTextInput: "TextInput", OpOverlay: "Overlay", OpScreenEffect: "ScreenEffect",
	}
	if name, ok := names[k]; ok {
		return name
	}
	return fmt.Sprintf("Op(%d)", k)
}
