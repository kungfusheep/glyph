package tui

import (
	"reflect"
	"unsafe"
)

// SerialTemplate is a compiled, phase-aware UI template.
// Build does all reflection. Execute is pure pointer arithmetic across three phases.
type SerialTemplate struct {
	ops      []SerialOp
	byLevel  [][]int16 // byLevel[depth] = op indices at that depth
	maxLevel int
	nodes    []SerialNode // pre-allocated node storage
}

// SerialNode is minimal runtime node data
type SerialNode struct {
	// Identity
	Parent int16
	Level  int8
	Kind   uint8 // for render dispatch

	// Geometry (set during measure, refined during layout)
	W, H int16
	X, Y int16

	// Container info
	IsRow    bool
	Gap      int8
	First    int16 // first child index
	Last     int16 // last child index (for sibling chain)
	NumChild int16

	// Values (populated during measure)
	Text  string
	Ratio float32
	Width int16 // fixed width for progress bars etc

	// Styling
	Bold bool
}

// SerialOp represents a single operation. Kind encodes WHAT and HOW (no runtime mode switches).
type SerialOp struct {
	Kind   uint8
	Level  int8
	Parent int16 // parent op index

	// Value access - exactly one used based on Kind
	StaticStr string
	StrPtr    *string
	StrOff    uintptr

	StaticInt int
	IntPtr    *int
	IntOff    uintptr

	BoolPtr *bool

	// Layout hints
	Width int16
	IsRow bool
	Gap   int8
	Bold  bool
	Style DStyle

	// ForEach
	SlicePtr unsafe.Pointer
	ElemSize uintptr
	IterTmpl *SerialTemplate // sub-template for each element

	// Conditional branches
	ThenTmpl *SerialTemplate
	ElseTmpl *SerialTemplate
}

// Op kinds - each encodes both WHAT and HOW to access values
const (
	SerialOpTextStatic uint8 = iota
	SerialOpTextPtr
	SerialOpTextOffset

	SerialOpProgressStatic
	SerialOpProgressPtr
	SerialOpProgressOffset

	SerialOpContainerStart
	SerialOpContainerEnd

	SerialOpForEach

	SerialOpIf
	SerialOpElse
)

// BuildSerial compiles a declarative UI into a SerialTemplate
func BuildSerial(ui any) *SerialTemplate {
	t := &SerialTemplate{
		ops:     make([]SerialOp, 0, 64),
		byLevel: make([][]int16, 16), // pre-alloc 16 levels
	}

	// Initialize level buckets
	for i := range t.byLevel {
		t.byLevel[i] = make([]int16, 0, 8)
	}

	t.compile(ui, -1, 0, nil, 0)

	// Trim unused levels
	for t.maxLevel >= 0 && len(t.byLevel[t.maxLevel]) == 0 {
		t.maxLevel--
	}
	t.byLevel = t.byLevel[:t.maxLevel+1]

	// Pre-allocate nodes (estimate: same as ops count)
	t.nodes = make([]SerialNode, 0, len(t.ops))

	return t
}

func (t *SerialTemplate) addOp(op SerialOp, level int) int16 {
	idx := int16(len(t.ops))
	op.Level = int8(level)
	t.ops = append(t.ops, op)

	// Add to level bucket
	if level >= len(t.byLevel) {
		// Extend byLevel if needed
		for len(t.byLevel) <= level {
			t.byLevel = append(t.byLevel, make([]int16, 0, 8))
		}
	}
	t.byLevel[level] = append(t.byLevel[level], idx)

	if level > t.maxLevel {
		t.maxLevel = level
	}

	return idx
}

func (t *SerialTemplate) compile(node any, parentIdx int16, level int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	if node == nil {
		return -1
	}

	switch v := node.(type) {
	case Text:
		return t.compileText(v, parentIdx, level, elemBase, elemSize)
	case Progress:
		return t.compileProgress(v, parentIdx, level, elemBase, elemSize)
	case Row:
		return t.compileContainer(v.Gap, v.Children, parentIdx, level, true, elemBase, elemSize)
	case Col:
		return t.compileContainer(v.Gap, v.Children, parentIdx, level, false, elemBase, elemSize)
	case IfNode:
		return t.compileIf(v, parentIdx, level, elemBase, elemSize)
	case ElseNode:
		return t.compileElse(v, parentIdx, level, elemBase, elemSize)
	case ForEachNode:
		return t.compileForEach(v, parentIdx, level)
	}

	return -1
}

func (t *SerialTemplate) compileText(v Text, parentIdx int16, level int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	op := SerialOp{
		Parent: parentIdx,
		Bold:   v.Bold || v.Style.Bold,
		Style:  v.Style,
	}

	// Determine access pattern at compile time - no runtime mode switch
	switch val := v.Content.(type) {
	case string:
		op.Kind = SerialOpTextStatic
		op.StaticStr = val
	case *string:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			op.Kind = SerialOpTextOffset
			op.StrOff = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			op.Kind = SerialOpTextPtr
			op.StrPtr = val
		}
	default:
		// Handle reflection case for other pointer types
		rv := reflect.ValueOf(v.Content)
		if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.String {
			ptr := (*string)(unsafe.Pointer(rv.Pointer()))
			if elemBase != nil && isWithinRange(unsafe.Pointer(ptr), elemBase, elemSize) {
				op.Kind = SerialOpTextOffset
				op.StrOff = uintptr(unsafe.Pointer(ptr)) - uintptr(elemBase)
			} else {
				op.Kind = SerialOpTextPtr
				op.StrPtr = ptr
			}
		} else {
			op.Kind = SerialOpTextStatic
			op.StaticStr = ""
		}
	}

	return t.addOp(op, level)
}

func (t *SerialTemplate) compileProgress(v Progress, parentIdx int16, level int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	width := v.Width
	if width == 0 {
		width = 20
	}

	op := SerialOp{
		Parent: parentIdx,
		Width:  width,
	}

	switch val := v.Value.(type) {
	case int:
		op.Kind = SerialOpProgressStatic
		op.StaticInt = val
	case *int:
		if elemBase != nil && isWithinRange(unsafe.Pointer(val), elemBase, elemSize) {
			op.Kind = SerialOpProgressOffset
			op.IntOff = uintptr(unsafe.Pointer(val)) - uintptr(elemBase)
		} else {
			op.Kind = SerialOpProgressPtr
			op.IntPtr = val
		}
	default:
		rv := reflect.ValueOf(v.Value)
		if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Int {
			ptr := (*int)(unsafe.Pointer(rv.Pointer()))
			if elemBase != nil && isWithinRange(unsafe.Pointer(ptr), elemBase, elemSize) {
				op.Kind = SerialOpProgressOffset
				op.IntOff = uintptr(unsafe.Pointer(ptr)) - uintptr(elemBase)
			} else {
				op.Kind = SerialOpProgressPtr
				op.IntPtr = ptr
			}
		} else {
			op.Kind = SerialOpProgressStatic
			op.StaticInt = 0
		}
	}

	return t.addOp(op, level)
}

func (t *SerialTemplate) compileContainer(gap int8, children []any, parentIdx int16, level int, isRow bool, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	startIdx := t.addOp(SerialOp{
		Kind:   SerialOpContainerStart,
		Parent: parentIdx,
		IsRow:  isRow,
		Gap:    gap,
	}, level)

	for _, child := range children {
		t.compile(child, startIdx, level+1, elemBase, elemSize)
	}

	t.addOp(SerialOp{
		Kind:   SerialOpContainerEnd,
		Parent: startIdx, // points back to start
	}, level)

	return startIdx
}

func (t *SerialTemplate) compileIf(v IfNode, parentIdx int16, level int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	op := SerialOp{
		Kind:   SerialOpIf,
		Parent: parentIdx,
	}

	// Compile condition
	switch val := v.Cond.(type) {
	case *bool:
		op.BoolPtr = val
	default:
		rv := reflect.ValueOf(v.Cond)
		if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Bool {
			op.BoolPtr = (*bool)(unsafe.Pointer(rv.Pointer()))
		}
	}

	// Compile then branch as sub-template
	if v.Then != nil {
		thenTmpl := &SerialTemplate{
			ops:     make([]SerialOp, 0, 16),
			byLevel: make([][]int16, 8),
		}
		for i := range thenTmpl.byLevel {
			thenTmpl.byLevel[i] = make([]int16, 0, 4)
		}
		thenTmpl.compile(v.Then, -1, 0, elemBase, elemSize)
		thenTmpl.byLevel = thenTmpl.byLevel[:thenTmpl.maxLevel+1]
		thenTmpl.nodes = make([]SerialNode, 0, len(thenTmpl.ops))
		op.ThenTmpl = thenTmpl
	}

	return t.addOp(op, level)
}

func (t *SerialTemplate) compileElse(v ElseNode, parentIdx int16, level int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	op := SerialOp{
		Kind:   SerialOpElse,
		Parent: parentIdx,
	}

	// Compile else branch as sub-template
	if v.Then != nil {
		elseTmpl := &SerialTemplate{
			ops:     make([]SerialOp, 0, 16),
			byLevel: make([][]int16, 8),
		}
		for i := range elseTmpl.byLevel {
			elseTmpl.byLevel[i] = make([]int16, 0, 4)
		}
		elseTmpl.compile(v.Then, -1, 0, elemBase, elemSize)
		elseTmpl.byLevel = elseTmpl.byLevel[:elseTmpl.maxLevel+1]
		elseTmpl.nodes = make([]SerialNode, 0, len(elseTmpl.ops))
		op.ElseTmpl = elseTmpl
	}

	return t.addOp(op, level)
}

func (t *SerialTemplate) compileForEach(v ForEachNode, parentIdx int16, level int) int16 {
	// Analyze slice
	sliceRV := reflect.ValueOf(v.Items)
	if sliceRV.Kind() != reflect.Ptr {
		panic("ForEach Items must be pointer to slice")
	}
	sliceType := sliceRV.Type().Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("ForEach Items must be pointer to slice")
	}
	elemType := sliceType.Elem()
	elemSize := elemType.Size()
	slicePtr := unsafe.Pointer(sliceRV.Pointer())

	// Create dummy element for template compilation
	renderRV := reflect.ValueOf(v.Render)
	takesPtr := renderRV.Type().In(0).Kind() == reflect.Ptr

	var dummyElem reflect.Value
	var dummyBase unsafe.Pointer
	if takesPtr {
		dummyElem = reflect.New(elemType)
		dummyBase = unsafe.Pointer(dummyElem.Pointer())
	} else {
		dummyElem = reflect.New(elemType).Elem()
		dummyBase = unsafe.Pointer(dummyElem.Addr().Pointer())
	}

	// Call render to get template structure
	templateResult := renderRV.Call([]reflect.Value{dummyElem})[0].Interface()

	// Compile iteration template
	iterTmpl := &SerialTemplate{
		ops:     make([]SerialOp, 0, 16),
		byLevel: make([][]int16, 8),
	}
	for i := range iterTmpl.byLevel {
		iterTmpl.byLevel[i] = make([]int16, 0, 4)
	}
	iterTmpl.compile(templateResult, -1, 0, dummyBase, elemSize)
	iterTmpl.byLevel = iterTmpl.byLevel[:iterTmpl.maxLevel+1]
	iterTmpl.nodes = make([]SerialNode, 0, len(iterTmpl.ops))

	op := SerialOp{
		Kind:     SerialOpForEach,
		Parent:   parentIdx,
		SlicePtr: slicePtr,
		ElemSize: elemSize,
		IterTmpl: iterTmpl,
	}

	return t.addOp(op, level)
}

// Execute runs all three phases and renders to buffer
func (t *SerialTemplate) Execute(buf *Buffer, w, h int16, statePtr unsafe.Pointer) {
	// Reset nodes
	t.nodes = t.nodes[:0]

	// Track if state for conditionals
	ifSatisfied := false

	// Phase 1: Measure (shallow → deep)
	for level := 0; level <= t.maxLevel; level++ {
		for _, idx := range t.byLevel[level] {
			t.measureOp(idx, statePtr, &ifSatisfied)
		}
	}

	// Phase 2: Layout (deep → shallow)
	for level := t.maxLevel; level >= 0; level-- {
		for _, idx := range t.byLevel[level] {
			t.layoutOp(idx, w, h)
		}
	}

	// Phase 3: Render (shallow → deep)
	for level := 0; level <= t.maxLevel; level++ {
		for _, idx := range t.byLevel[level] {
			t.renderOp(idx, buf)
		}
	}
}

// Measure phase - creates nodes with natural sizes
func (t *SerialTemplate) measureOp(opIdx int16, statePtr unsafe.Pointer, ifSatisfied *bool) {
	op := &t.ops[opIdx]

	switch op.Kind {
	case SerialOpTextStatic:
		t.measureTextStatic(op)
	case SerialOpTextPtr:
		t.measureTextPtr(op)
	case SerialOpTextOffset:
		t.measureTextOffset(op, statePtr)
	case SerialOpProgressStatic:
		t.measureProgressStatic(op)
	case SerialOpProgressPtr:
		t.measureProgressPtr(op)
	case SerialOpProgressOffset:
		t.measureProgressOffset(op, statePtr)
	case SerialOpContainerStart:
		t.measureContainerStart(op)
	case SerialOpContainerEnd:
		t.measureContainerEnd(op)
	case SerialOpForEach:
		t.measureForEach(op)
	case SerialOpIf:
		t.measureIf(op, ifSatisfied)
	case SerialOpElse:
		t.measureElse(op, ifSatisfied)
	}
}

func (t *SerialTemplate) measureTextStatic(op *SerialOp) {
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Text:   op.StaticStr,
		W:      int16(len(op.StaticStr)),
		H:      1,
		Bold:   op.Bold,
	})
}

func (t *SerialTemplate) measureTextPtr(op *SerialOp) {
	text := *op.StrPtr
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Text:   text,
		W:      int16(len(text)),
		H:      1,
		Bold:   op.Bold,
	})
}

func (t *SerialTemplate) measureTextOffset(op *SerialOp, basePtr unsafe.Pointer) {
	text := *(*string)(unsafe.Add(basePtr, op.StrOff))
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Text:   text,
		W:      int16(len(text)),
		H:      1,
		Bold:   op.Bold,
	})
}

func (t *SerialTemplate) measureProgressStatic(op *SerialOp) {
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Ratio:  float32(op.StaticInt) / 100.0,
		W:      op.Width,
		H:      1,
		Width:  op.Width,
	})
}

func (t *SerialTemplate) measureProgressPtr(op *SerialOp) {
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Ratio:  float32(*op.IntPtr) / 100.0,
		W:      op.Width,
		H:      1,
		Width:  op.Width,
	})
}

func (t *SerialTemplate) measureProgressOffset(op *SerialOp, basePtr unsafe.Pointer) {
	val := *(*int)(unsafe.Add(basePtr, op.IntOff))
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		Ratio:  float32(val) / 100.0,
		W:      op.Width,
		H:      1,
		Width:  op.Width,
	})
}

func (t *SerialTemplate) measureContainerStart(op *SerialOp) {
	t.nodes = append(t.nodes, SerialNode{
		Kind:   op.Kind,
		Parent: op.Parent,
		Level:  op.Level,
		IsRow:  op.IsRow,
		Gap:    op.Gap,
		First:  -1,
		Last:   -1,
	})
}

func (t *SerialTemplate) measureContainerEnd(op *SerialOp) {
	// Container end doesn't create a node, just signals end of children
	// Actual measurement happens in layout phase
}

func (t *SerialTemplate) measureForEach(op *SerialOp) {
	sliceHdr := *(*sliceHeader)(op.SlicePtr)
	if sliceHdr.Len == 0 || op.IterTmpl == nil {
		return
	}

	// Execute sub-template for each element
	for i := 0; i < sliceHdr.Len; i++ {
		elemPtr := unsafe.Add(sliceHdr.Data, uintptr(i)*op.ElemSize)

		// Reset sub-template nodes
		op.IterTmpl.nodes = op.IterTmpl.nodes[:0]

		ifSat := false
		for level := 0; level <= op.IterTmpl.maxLevel; level++ {
			for _, idx := range op.IterTmpl.byLevel[level] {
				op.IterTmpl.measureOp(idx, elemPtr, &ifSat)
			}
		}

		// Copy nodes to main template
		for _, node := range op.IterTmpl.nodes {
			t.nodes = append(t.nodes, node)
		}
	}
}

func (t *SerialTemplate) measureIf(op *SerialOp, ifSatisfied *bool) {
	if op.BoolPtr != nil && *op.BoolPtr {
		*ifSatisfied = true
		if op.ThenTmpl != nil {
			op.ThenTmpl.nodes = op.ThenTmpl.nodes[:0]
			ifSat := false
			for level := 0; level <= op.ThenTmpl.maxLevel; level++ {
				for _, idx := range op.ThenTmpl.byLevel[level] {
					op.ThenTmpl.measureOp(idx, nil, &ifSat)
				}
			}
			for _, node := range op.ThenTmpl.nodes {
				t.nodes = append(t.nodes, node)
			}
		}
	} else {
		*ifSatisfied = false
	}
}

func (t *SerialTemplate) measureElse(op *SerialOp, ifSatisfied *bool) {
	if !*ifSatisfied {
		if op.ElseTmpl != nil {
			op.ElseTmpl.nodes = op.ElseTmpl.nodes[:0]
			ifSat := false
			for level := 0; level <= op.ElseTmpl.maxLevel; level++ {
				for _, idx := range op.ElseTmpl.byLevel[level] {
					op.ElseTmpl.measureOp(idx, nil, &ifSat)
				}
			}
			for _, node := range op.ElseTmpl.nodes {
				t.nodes = append(t.nodes, node)
			}
		}
	}
}

// Layout phase - computes positions
func (t *SerialTemplate) layoutOp(opIdx int16, w, h int16) {
	// For now, simple vertical stacking
	// TODO: implement proper flexbox layout
}

// Render phase - draws to buffer
func (t *SerialTemplate) renderOp(opIdx int16, buf *Buffer) {
	// For now, render is handled after all phases
}

// ExecuteSimple is a simpler version that just measures and renders vertically
func (t *SerialTemplate) ExecuteSimple(buf *Buffer, w, h int16, statePtr unsafe.Pointer) {
	t.executeInternal(buf, w, h, statePtr, false)
}

// ExecuteNoClear renders with padded writes, allowing caller to skip Clear().
// Only safe when UI structure is stable (no shrinking content).
func (t *SerialTemplate) ExecuteNoClear(buf *Buffer, w, h int16, statePtr unsafe.Pointer) {
	t.executeInternal(buf, w, h, statePtr, true)
}

func (t *SerialTemplate) executeInternal(buf *Buffer, w, h int16, statePtr unsafe.Pointer, padded bool) {
	// Reset nodes
	t.nodes = t.nodes[:0]

	ifSatisfied := false

	// Phase 1: Measure (creates nodes with sizes)
	for level := 0; level <= t.maxLevel; level++ {
		for _, idx := range t.byLevel[level] {
			t.measureOp(idx, statePtr, &ifSatisfied)
		}
	}

	// Simple layout: stack vertically
	y := int16(0)
	for i := range t.nodes {
		t.nodes[i].X = 0
		t.nodes[i].Y = y
		y += t.nodes[i].H
	}

	// Render - use fast paths that skip border merging
	for i := range t.nodes {
		node := &t.nodes[i]
		switch {
		case node.Kind == SerialOpTextStatic || node.Kind == SerialOpTextPtr || node.Kind == SerialOpTextOffset:
			style := Style{}
			if node.Bold {
				style.Attr = AttrBold
			}
			if padded {
				buf.WriteStringPadded(int(node.X), int(node.Y), node.Text, style, int(w))
			} else {
				buf.WriteStringFast(int(node.X), int(node.Y), node.Text, style, int(w))
			}
		case node.Kind == SerialOpProgressStatic || node.Kind == SerialOpProgressPtr || node.Kind == SerialOpProgressOffset:
			buf.WriteProgressBar(int(node.X), int(node.Y), int(node.Width), node.Ratio, Style{})
		}
	}
}
