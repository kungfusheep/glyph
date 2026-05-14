package glyph

import (
	"cmp"
	"unsafe"
)

// MatchCase holds a single case arm for Match: an operator, a comparison value
// or predicate function, and the node to render when the case matches.
type MatchCase[T comparable] struct {
	op   matchOp
	val  T
	pred func(T) bool
	node any
}

type matchOp int

const (
	matchOpEq matchOp = iota
	matchOpNe
	matchOpGt
	matchOpLt
	matchOpGte
	matchOpLte
	matchOpWhere
)

// Eq matches when *ptr == val.
func Eq[T comparable](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpEq, val: val, node: node}
}

// Ne matches when *ptr != val.
func Ne[T comparable](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpNe, val: val, node: node}
}

// Gt matches when *ptr > val.
func Gt[T cmp.Ordered](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpGt, val: val, node: node}
}

// Lt matches when *ptr < val.
func Lt[T cmp.Ordered](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpLt, val: val, node: node}
}

// Gte matches when *ptr >= val.
func Gte[T cmp.Ordered](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpGte, val: val, node: node}
}

// Lte matches when *ptr <= val.
func Lte[T cmp.Ordered](val T, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpLte, val: val, node: node}
}

// Where matches when fn(*ptr) returns true.
func Where[T comparable](fn func(T) bool, node any) MatchCase[T] {
	return MatchCase[T]{op: matchOpWhere, pred: fn, node: node}
}

// MatchC is the compiled form of a Match, ready for template evaluation.
// T is comparable; ordered operators (Gt/Lt/Gte/Lte) constrain themselves
// to cmp.Ordered at the case level.
type MatchC[T comparable] struct {
	ptr       *T
	offset    uintptr
	hasOffset bool
	cases     []MatchCase[T]
	def       any
}

// Match builds a first-match-wins conditional block.
// Each case is evaluated top-to-bottom; the first true case renders.
// Chain .Default() to set a fallback when no case matches.
//
//	Match(&cpu,
//	    Gt(90.0, Text("CRITICAL").FG(Red)),
//	    Gt(70.0, Text("WARNING").FG(Yellow)),
//	    Where(func(v float64) bool { return v < 10 }, Text("COLD").FG(Blue)),
//	).Default(Text("OK").FG(Green))
func Match[T comparable](ptr *T, cases ...MatchCase[T]) *MatchC[T] {
	m := &MatchC[T]{ptr: ptr}
	m.cases = append(m.cases, cases...)
	return m
}

// Default sets the fallback node when no case matches.
func (m *MatchC[T]) Default(node any) *MatchC[T] {
	m.def = node
	return m
}

func (m *MatchC[T]) evaluate(v T) int {
	for i, c := range m.cases {
		if c.matchWithOrdered(v) {
			return i
		}
	}
	return -1
}

// matchNodeInterface allows the template compiler to handle Match generically.
type matchNodeInterface interface {
	getMatchIndex() int
	getMatchIndexWithBase(unsafe.Pointer) int
	getCaseNodes() []any
	getDefaultNode() any
	getPtrAddr() uintptr
	setPtrOffset(uintptr)
}

func (m *MatchC[T]) getMatchIndex() int {
	return m.evaluate(*m.ptr)
}

func (m *MatchC[T]) getMatchIndexWithBase(base unsafe.Pointer) int {
	var ptr *T
	if base != nil && m.hasOffset {
		ptr = (*T)(unsafe.Add(base, m.offset))
	} else {
		ptr = m.ptr
	}
	return m.evaluate(*ptr)
}

func (m *MatchC[T]) getCaseNodes() []any {
	nodes := make([]any, len(m.cases))
	for i, c := range m.cases {
		nodes[i] = c.node
	}
	return nodes
}

func (m *MatchC[T]) getDefaultNode() any {
	return m.def
}

func (m *MatchC[T]) getPtrAddr() uintptr {
	return uintptr(unsafe.Pointer(m.ptr))
}

func (m *MatchC[T]) setPtrOffset(off uintptr) {
	m.offset = off
	m.hasOffset = true
}

var _ matchNodeInterface = (*MatchC[int])(nil)

func (c *MatchCase[T]) matchWithOrdered(v T) bool {
	switch c.op {
	case matchOpEq:
		return v == c.val
	case matchOpNe:
		return v != c.val
	case matchOpWhere:
		return c.pred(v)
	default:
		// ordered comparison — use any-based assertion to access cmp.Ordered behavior
		return orderedCompare(v, c.val, c.op)
	}
}

// orderedCompare performs Gt/Lt/Gte/Lte by asserting the values to known ordered types.
// this is the tradeoff for having Match accept comparable: ordered ops are checked at
// runtime rather than compile time. if T isn't ordered, the case never matches.
func orderedCompare(a, b any, op matchOp) bool {
	switch va := a.(type) {
	case int:
		return ordCmp(va, b.(int), op)
	case int8:
		return ordCmp(va, b.(int8), op)
	case int16:
		return ordCmp(va, b.(int16), op)
	case int32:
		return ordCmp(va, b.(int32), op)
	case int64:
		return ordCmp(va, b.(int64), op)
	case uint:
		return ordCmp(va, b.(uint), op)
	case uint8:
		return ordCmp(va, b.(uint8), op)
	case uint16:
		return ordCmp(va, b.(uint16), op)
	case uint32:
		return ordCmp(va, b.(uint32), op)
	case uint64:
		return ordCmp(va, b.(uint64), op)
	case float32:
		return ordCmp(va, b.(float32), op)
	case float64:
		return ordCmp(va, b.(float64), op)
	case string:
		return ordCmp(va, b.(string), op)
	default:
		return false
	}
}

func ordCmp[T cmp.Ordered](a, b T, op matchOp) bool {
	switch op {
	case matchOpGt:
		return a > b
	case matchOpLt:
		return a < b
	case matchOpGte:
		return a >= b
	case matchOpLte:
		return a <= b
	default:
		return false
	}
}
