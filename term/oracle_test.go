package term

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The reference oracle: a real captured session replayed through the emulator, with
// the expected grids produced by tmux rather than written by hand. It catches the
// class of defect no hand-built sequence does, because it is whatever the program
// actually emitted, and it pins the resize rules against a terminal that already
// implements them.
//
// testdata/session.vt is a neovim session over a generated file: paging, jumps,
// line deletes and an insert, so it carries the alternate screen, scroll regions,
// insert/delete-line and absolute positioning. Regenerate the golden grids with
// ORACLE_REGEN=1 (needs tmux); the session bytes themselves are captured by hand.

type oracleCase struct {
	name           string
	cols, rows     int // geometry the bytes were composed at
	toCols, toRows int // 0 = no resize
	golden         string
}

var oracleCases = []oracleCase{
	{"native", 100, 97, 0, 0, "session-100x97.grid"},
	{"shrink", 100, 97, 96, 40, "session-100x97-to-96x40.grid"},
	{"rows shrink, cols grow", 100, 97, 110, 60, "session-100x97-to-110x60.grid"},
	{"grow", 100, 97, 120, 110, "session-100x97-to-120x110.grid"},
}

func oracleGrid(t *testing.T, c oracleCase) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "session.vt"))
	if err != nil {
		t.Fatal(err)
	}
	s := newScreen(c.rows, c.cols)
	s.write(b)
	rows, cols := c.rows, c.cols
	if c.toRows != 0 {
		s.resize(c.toRows, c.toCols)
		rows, cols = c.toRows, c.toCols
	}
	var out strings.Builder
	for y := 0; y < rows; y++ {
		var line strings.Builder
		for x := 0; x < cols; x++ {
			if r := s.cellAt(x, y).Rune; r != 0 {
				line.WriteRune(r)
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		out.WriteByte('\n')
	}
	return out.String()
}

func TestOracleMatchesTmux(t *testing.T) {
	if os.Getenv("ORACLE_REGEN") != "" {
		regenerateOracleGoldens(t)
		return
	}
	for _, c := range oracleCases {
		t.Run(c.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", c.golden))
			if err != nil {
				t.Fatal(err)
			}
			got := oracleGrid(t, c)
			if got == string(want) {
				return
			}
			gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
			for i := range max(len(gotLines), len(wantLines)) {
				var g, w string
				if i < len(gotLines) {
					g = gotLines[i]
				}
				if i < len(wantLines) {
					w = wantLines[i]
				}
				if g != w {
					t.Errorf("row %d:\n got %q\nwant %q", i, g, w)
				}
			}
		})
	}
}

// regenerateOracleGoldens replays session.vt into a tmux pane of each case's
// geometry and writes what tmux painted. Run it when the session bytes change, and
// read the diff before committing: it is the reference moving, not the code.
func regenerateOracleGoldens(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	for _, c := range oracleCases {
		name := "vtoracle-" + c.name
		exec.Command("tmux", "kill-session", "-t", name).Run()
		run := func(args ...string) {
			if out, err := exec.Command("tmux", args...).CombinedOutput(); err != nil {
				t.Fatalf("tmux %v: %v: %s", args, err, out)
			}
		}
		run("new-session", "-d", "-s", name, "-x", strconv.Itoa(c.cols), "-y", strconv.Itoa(c.rows), "cat > /dev/null")
		time.Sleep(400 * time.Millisecond)
		tty, err := exec.Command("tmux", "display-message", "-p", "-t", name, "#{pane_tty}").Output()
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join("testdata", "session.vt"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(strings.TrimSpace(string(tty)), b, 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(800 * time.Millisecond)
		if c.toRows != 0 {
			run("resize-window", "-t", name, "-x", strconv.Itoa(c.toCols), "-y", strconv.Itoa(c.toRows))
			time.Sleep(600 * time.Millisecond)
		}
		out, err := exec.Command("tmux", "capture-pane", "-t", name, "-p").Output()
		if err != nil {
			t.Fatal(err)
		}
		exec.Command("tmux", "kill-session", "-t", name).Run()
		var trimmed strings.Builder
		for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
			trimmed.WriteString(strings.TrimRight(line, " \t"))
			trimmed.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join("testdata", c.golden), []byte(trimmed.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("regenerated %s", c.golden)
	}
}
