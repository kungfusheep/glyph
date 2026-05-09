package glyph

func (v VBoxC) Build() Component             { return v }
func (h HBoxC) Build() Component             { return h }
func (b Box) Build() Component               { return b }
func (t TextC) Build() Component             { return t }
func (r RichTextNode) Build() Component      { return r }
func (s SpacerC) Build() Component           { return s }
func (h HRuleC) Build() Component            { return h }
func (v VRuleC) Build() Component            { return v }
func (p ProgressC) Build() Component         { return p }
func (s SpinnerC) Build() Component          { return s }
func (l LeaderC) Build() Component           { return l }
func (c counterC) Build() Component          { return c }
func (s SparklineC) Build() Component        { return s }
func (j JumpC) Build() Component             { return j }
func (l LayerViewC) Build() Component        { return l }
func (o OverlayC) Build() Component          { return o }
func (t TabsC) Build() Component             { return t }
func (s ScrollbarC) Build() Component        { return s }
func (a AutoTableC) Build() Component        { return a }
func (t TextBlockC) Build() Component        { return t }
func (c Custom) Build() Component            { return c }
func (t Table) Build() Component             { return t }
func (t TabsNode) Build() Component          { return t }
func (t TreeView) Build() Component          { return t }
func (t TextInput) Build() Component         { return t }
func (o OverlayNode) Build() Component       { return o }
func (s ScreenEffectNode) Build() Component  { return s }
func (o OnC) Build() Component               { return o }
func (s SelectionList) Build() Component     { return s }
func (c *CheckboxC) Build() Component        { return c }
func (r *RadioC) Build() Component           { return r }
func (i *InputC) Build() Component           { return i }
func (l *LogC) Build() Component             { return l }
func (t *TextViewC) Build() Component        { return t }
func (s *ScrollViewC) Build() Component      { return s }
func (f *FilterLogC) Build() Component       { return f }
func (f *FormC) Build() Component            { return f }
func (f *FilterListC[T]) Build() Component   { return f }
func (f ForEachC[T]) Build() Component       { return f }
func (l *ListC[T]) Build() Component         { return l }
func (c *CheckListC[T]) Build() Component    { return c }
func (c *ConditionEval[T]) Build() Component { return c }
func (c *OrdConditionEval[T]) Build() Component {
	return c
}
func (s *SwitchNode[T]) Build() Component { return s }
func (m *MatchNode[T]) Build() Component  { return m }
