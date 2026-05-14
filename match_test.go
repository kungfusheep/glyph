package glyph

import (
	"strings"
	"testing"
)

func TestMatchGtFirstMatchWins(t *testing.T) {
	cpu := 95.0
	m := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
		Gt(70.0, Text("WARNING")),
	).Default(Text("OK"))
	idx := m.getMatchIndex()
	if idx != 0 {
		t.Errorf("expected match index 0 for 95 > 90, got %d", idx)
	}
}

func TestMatchGtSecondCase(t *testing.T) {
	cpu := 75.0
	m := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
		Gt(70.0, Text("WARNING")),
	).Default(Text("OK"))
	idx := m.getMatchIndex()
	if idx != 1 {
		t.Errorf("expected match index 1 for 75 > 70, got %d", idx)
	}
}

func TestMatchFallsToElse(t *testing.T) {
	cpu := 50.0
	m := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
		Gt(70.0, Text("WARNING")),
	).Default(Text("OK"))
	idx := m.getMatchIndex()
	if idx != -1 {
		t.Errorf("expected match index -1 (else), got %d", idx)
	}
	if m.getDefaultNode() == nil {
		t.Errorf("expected default node, got nil")
	}
}

func TestMatchNoElseNoMatch(t *testing.T) {
	cpu := 50.0
	m := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
	)
	idx := m.getMatchIndex()
	if idx != -1 {
		t.Errorf("expected -1 when no case matches and no else, got %d", idx)
	}
	if m.getDefaultNode() != nil {
		t.Errorf("expected nil default, got %v", m.getDefaultNode())
	}
}

func TestMatchAllOperators(t *testing.T) {
	tests := []struct {
		name string
		val  int
		m    *MatchC[int]
		want int
	}{
		{"Gt match", 10, Match(&[]int{10}[0], Gt(5, Text("yes"))), 0},
		{"Gt no match", 3, Match(&[]int{3}[0], Gt(5, Text("yes"))), -1},
		{"Lt match", 3, Match(&[]int{3}[0], Lt(5, Text("yes"))), 0},
		{"Lt no match", 10, Match(&[]int{10}[0], Lt(5, Text("yes"))), -1},
		{"Gte match equal", 5, Match(&[]int{5}[0], Gte(5, Text("yes"))), 0},
		{"Gte match above", 6, Match(&[]int{6}[0], Gte(5, Text("yes"))), 0},
		{"Gte no match", 4, Match(&[]int{4}[0], Gte(5, Text("yes"))), -1},
		{"Lte match equal", 5, Match(&[]int{5}[0], Lte(5, Text("yes"))), 0},
		{"Lte match below", 4, Match(&[]int{4}[0], Lte(5, Text("yes"))), 0},
		{"Lte no match", 6, Match(&[]int{6}[0], Lte(5, Text("yes"))), -1},
		{"Eq match", 5, Match(&[]int{5}[0], Eq(5, Text("yes"))), 0},
		{"Eq no match", 6, Match(&[]int{6}[0], Eq(5, Text("yes"))), -1},
		{"Ne match", 6, Match(&[]int{6}[0], Ne(5, Text("yes"))), 0},
		{"Ne no match", 5, Match(&[]int{5}[0], Ne(5, Text("yes"))), -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.getMatchIndex()
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMatchWhere(t *testing.T) {
	t.Run("When predicate matches", func(t *testing.T) {
		val := 42
		m := Match(&val,
			Where(func(v int) bool { return v%2 == 0 }, Text("EVEN")),
		).Default(Text("ODD"))
		if m.getMatchIndex() != 0 {
			t.Error("expected When to match for even number")
		}
	})

	t.Run("When predicate no match falls through", func(t *testing.T) {
		val := 41
		m := Match(&val,
			Where(func(v int) bool { return v%2 == 0 }, Text("EVEN")),
		).Default(Text("ODD"))
		if m.getMatchIndex() != -1 {
			t.Error("expected When to not match for odd number")
		}
	})
}

func TestMatchWhereWithString(t *testing.T) {
	name := "admin:root"
	m := Match(&name,
		Where(func(s string) bool { return strings.HasPrefix(s, "admin:") }, Text("ADMIN")),
	).Default(Text("USER"))
	if m.getMatchIndex() != 0 {
		t.Error("expected When to match admin prefix")
	}
}

func TestMatchWhereWithStruct(t *testing.T) {
	type user struct {
		Role   string
		Active bool
	}
	u := user{Role: "admin", Active: true}
	m := Match(&u,
		Where(func(u user) bool { return u.Role == "admin" && u.Active }, Text("ADMIN_PANEL")),
		Where(func(u user) bool { return !u.Active }, Text("DISABLED")),
	).Default(Text("PROFILE"))
	if m.getMatchIndex() != 0 {
		t.Error("expected first When to match active admin")
	}

	u.Active = false
	if m.getMatchIndex() != 1 {
		t.Errorf("expected second When to match inactive user, got %d", m.getMatchIndex())
	}

	u.Role = "user"
	u.Active = true
	if m.getMatchIndex() != -1 {
		t.Errorf("expected no match for active non-admin, got %d", m.getMatchIndex())
	}
}

func TestMatchDynamic(t *testing.T) {
	cpu := 95.0
	m := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
		Gt(70.0, Text("WARNING")),
	).Default(Text("OK"))
	if m.getMatchIndex() != 0 {
		t.Error("expected CRITICAL for 95")
	}

	cpu = 75.0
	if m.getMatchIndex() != 1 {
		t.Error("expected WARNING for 75")
	}

	cpu = 50.0
	if m.getMatchIndex() != -1 {
		t.Error("expected else for 50")
	}
}

func TestMatchRendersInVBox(t *testing.T) {
	cpu := 95.0
	view := VBox(
		Match(&cpu,
			Gt(90.0, Text("CRITICAL")),
			Gt(70.0, Text("WARNING")),
		).Default(Text("OK")),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)

	line := extractLine(buf, 0, 10)
	if !strings.Contains(line, "CRITICAL") {
		t.Errorf("expected CRITICAL for 95.0, got %q", line)
	}
}

func TestMatchRendersElseInVBox(t *testing.T) {
	cpu := 50.0
	view := VBox(
		Match(&cpu,
			Gt(90.0, Text("CRITICAL")),
			Gt(70.0, Text("WARNING")),
		).Default(Text("OK")),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)

	line := extractLine(buf, 0, 5)
	if !strings.Contains(line, "OK") {
		t.Errorf("expected OK for 50.0, got %q", line)
	}
}

func TestMatchRendersNothingWithNoElse(t *testing.T) {
	cpu := 50.0
	view := VBox(
		Text("HEADER"),
		Match(&cpu,
			Gt(90.0, Text("CRITICAL")),
		),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)

	line0 := extractLine(buf, 0, 10)
	if !strings.Contains(line0, "HEADER") {
		t.Errorf("expected HEADER on line 0, got %q", line0)
	}
	// line 1 should be empty — no match, no else
	line1 := strings.TrimRight(extractLine(buf, 1, 10), " ")
	if line1 != "" {
		t.Errorf("expected empty line 1 with no match, got %q", line1)
	}
}

func TestMatchDynamicRerender(t *testing.T) {
	cpu := 95.0
	view := VBox(
		Match(&cpu,
			Gt(90.0, Text("CRITICAL")),
			Gt(70.0, Text("WARNING")),
		).Default(Text("OK")),
	)

	tmpl := Build(view)

	// first render: CRITICAL
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)
	line := extractLine(buf, 0, 10)
	if !strings.Contains(line, "CRITICAL") {
		t.Errorf("expected CRITICAL, got %q", line)
	}

	// mutate and re-render: WARNING
	cpu = 75.0
	buf = NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)
	line = extractLine(buf, 0, 10)
	if !strings.Contains(line, "WARNING") {
		t.Errorf("expected WARNING after mutation, got %q", line)
	}

	// mutate and re-render: OK
	cpu = 50.0
	buf = NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)
	line = extractLine(buf, 0, 5)
	if !strings.Contains(line, "OK") {
		t.Errorf("expected OK after mutation, got %q", line)
	}
}

func TestMatchInHBox(t *testing.T) {
	cpu := 95.0
	view := HBox(
		Text("CPU: "),
		Match(&cpu,
			Gt(90.0, Text("CRITICAL")),
			Gt(70.0, Text("WARNING")),
		).Default(Text("OK")),
	)

	tmpl := Build(view)
	buf := NewBuffer(30, 1)
	tmpl.Execute(buf, 30, 1)

	line := extractLine(buf, 0, 20)
	if !strings.Contains(line, "CPU: ") || !strings.Contains(line, "CRITICAL") {
		t.Errorf("expected 'CPU: ' and 'CRITICAL' in HBox, got %q", line)
	}
}

func TestMatchMixedComponentBranchesRender(t *testing.T) {
	cpu := 95.0
	view := VBox(
		Text("CPU"),
		Match(&cpu,
			Gt(90.0, HBox(Text("CRIT"), Text("ICAL"))),
			Gt(70.0, Text("WARNING")),
		).Default(VBox(Text("O"), Text("K"))),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)

	line := extractLine(buf, 1, 10)
	if line != "CRITICAL  " {
		t.Errorf("expected HBox branch to render CRITICAL, got %q", line)
	}

	cpu = 50.0
	buf.Clear()
	tmpl.Execute(buf, 20, 5)

	line = extractLine(buf, 1, 5)
	if line != "O    " {
		t.Errorf("expected default VBox first row, got %q", line)
	}
	line = extractLine(buf, 2, 5)
	if line != "K    " {
		t.Errorf("expected default VBox second row, got %q", line)
	}
}

func TestMatchScalarBranchesDriveHeight(t *testing.T) {
	score := 95
	tmpl := Build(VBox.Height(
		Match(&score,
			Gt(90, int16(3)),
			Gt(70, int16(2)),
		).Default(int16(1)),
	)(
		Text("one"),
		Text("two"),
		Text("three"),
	))
	buf := NewBuffer(20, 5)

	tmpl.Execute(buf, 20, 5)
	if got := tmpl.geom[0].H; got != 3 {
		t.Errorf("score=95: got H=%d, want 3", got)
	}

	score = 75
	buf.Clear()
	tmpl.Execute(buf, 20, 5)
	if got := tmpl.geom[0].H; got != 2 {
		t.Errorf("score=75: got H=%d, want 2", got)
	}

	score = 50
	buf.Clear()
	tmpl.Execute(buf, 20, 5)
	if got := tmpl.geom[0].H; got != 1 {
		t.Errorf("score=50: got H=%d, want 1", got)
	}
}

func TestMatchWhereRendersInTemplate(t *testing.T) {
	name := "admin:root"
	view := VBox(
		Match(&name,
			Where(func(s string) bool { return strings.HasPrefix(s, "admin:") }, Text("ADMIN_VIEW")),
		).Default(Text("USER_VIEW")),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)

	line := extractLine(buf, 0, 12)
	if !strings.Contains(line, "ADMIN_VIEW") {
		t.Errorf("expected ADMIN_VIEW, got %q", line)
	}

	name = "user:pete"
	buf = NewBuffer(20, 3)
	tmpl.Execute(buf, 20, 3)
	line = extractLine(buf, 0, 12)
	if !strings.Contains(line, "USER_VIEW") {
		t.Errorf("expected USER_VIEW after change, got %q", line)
	}
}

func TestMatchInsideForEach(t *testing.T) {
	type item struct {
		Score float64
		Name  string
	}
	items := []item{
		{Score: 95, Name: "hot"},
		{Score: 75, Name: "warm"},
		{Score: 30, Name: "cool"},
	}

	view := VBox(
		ForEach(&items, func(it *item) Component {
			return Match(&it.Score,
				Gt(90.0, Text("CRITICAL")),
				Gt(70.0, Text("WARNING")),
			).Default(Text("OK"))
		}),
	)

	tmpl := Build(view)
	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)

	line0 := extractLine(buf, 0, 20)
	line1 := extractLine(buf, 1, 20)
	line2 := extractLine(buf, 2, 20)

	if !strings.Contains(line0, "CRITICAL") {
		t.Errorf("expected CRITICAL on line 0, got %q", line0)
	}
	if !strings.Contains(line1, "WARNING") {
		t.Errorf("expected WARNING on line 1, got %q", line1)
	}
	if !strings.Contains(line2, "OK") {
		t.Errorf("expected OK on line 2, got %q", line2)
	}
}

func TestMatchStringEq(t *testing.T) {
	status := "error"
	m := Match(&status,
		Eq("loading", Text("SPINNER")),
		Eq("error", Text("ERROR_VIEW")),
	).Default(Text("CONTENT"))
	if m.getMatchIndex() != 1 {
		t.Errorf("expected index 1 for Eq('error'), got %d", m.getMatchIndex())
	}

	status = "loading"
	if m.getMatchIndex() != 0 {
		t.Errorf("expected index 0 for Eq('loading'), got %d", m.getMatchIndex())
	}

	status = "ready"
	if m.getMatchIndex() != -1 {
		t.Errorf("expected index -1 (else) for 'ready', got %d", m.getMatchIndex())
	}
}

func TestMatchMixedOperators(t *testing.T) {
	val := 5
	m := Match(&val,
		Eq(0, Text("ZERO")),
		Lt(0, Text("NEGATIVE")),
		Gt(10, Text("HIGH")),
	).Default(Text("NORMAL"))
	if m.getMatchIndex() != -1 {
		t.Errorf("expected else for 5, got %d", m.getMatchIndex())
	}

	val = 0
	if m.getMatchIndex() != 0 {
		t.Errorf("expected index 0 for Eq(0), got %d", m.getMatchIndex())
	}

	val = -3
	if m.getMatchIndex() != 1 {
		t.Errorf("expected index 1 for Lt(0), got %d", m.getMatchIndex())
	}

	val = 15
	if m.getMatchIndex() != 2 {
		t.Errorf("expected index 2 for Gt(10), got %d", m.getMatchIndex())
	}
}
