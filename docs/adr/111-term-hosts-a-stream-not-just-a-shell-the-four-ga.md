# ADR 111: term hosts a stream, not just a shell: the four gaps between the terminal component and its first consumer

- status: accepted
- date: 2026-07-14 09:13:59
- proposer: Kestrel@recap
- parties: recap, tui
- deliberation: recap proposal show 111 (11 comments)

## Context and Problem Statement

The `term` package on the `glyph-term` branch embeds a terminal in a glyph layout: `term.New()`
returns a Layer-backed `TermC` that runs a shell on a pty, interprets its output into a cell
grid, and composes like any other component (`term/term.go:47`, `term/term.go:134`). The
example proves it embeds (`examples/term/`), and SGR is complete to truecolor (`term/sgr.go`).

It has a consumer waiting. recap hosts long-lived agent processes on a pty and today hands the
whole terminal to one on attach: raw mode, output piped unparsed to stdout, and a scan of the
raw byte stream for the detach key (`recap/agenthost/client.go:30`, `:54`, `:72`). That design
has produced a live bug. A pty capture of the agent recap launches shows what it emits in its
first thirty bytes:

```
ESC[?1049h        alternate screen on
ESC[<u  ESC[>1u   kitty keyboard protocol on
ESC[>4;2m         xterm modifyOtherKeys level 2
ESC[?1000h ?1002h ?1003h ?1006h ?2004h ?1004h
```

Because the attach is a raw pipe, those escapes reconfigure the *human's* terminal. Under the
kitty protocol the terminal stops sending `0x1c` for Ctrl-\, so the detach scan hunts a byte
that no longer arrives and the human cannot leave the agent. recap's answer is to stop piping
raw bytes and render the agent through `TermC` instead: the agent's escapes terminate in the
VT interpreter, the real terminal is never reconfigured, keys arrive as decoded `riffkey.Key`
events, and detach becomes an ordinary binding on a router above the pane. That decision is
recorded on recap's side; this proposal is the glyph work it depends on.

Three gaps stand between the component and that consumer, and the first two block it outright.
A fourth thing has to be settled with them: what the component's lifecycle callbacks mean once it
no longer owns a process.

## Considered Options

1. **Leave `term` as a shell-hosting component.** recap writes its own VT interpreter and pty
   plumbing against `glyph.Layer` directly.
2. **Widen `term` to host any pty-shaped byte stream, and finish the screen model.** The
   component keeps its shape; the pty stops being the only thing it can drive.

## Decision Outcome

Chosen option: 2. `term` grows the capabilities below. They are ordered: the first is the
hard prerequisite, the second is what makes the component reusable at all beyond a local shell.

**1. The alternate screen.** `term/vt.go:361` says it outright: `1049/47/1047 alt-screen
intentionally unhandled in this slice`. Every full-screen program drives it, which is why
`examples/term/README.md` records that vim, htop and less do not render. A hosted agent turns
it on in its first frame, so without this the pane paints garbage. The screen keeps a second
grid and swaps on `?1049h`/`?1049l` (saving and restoring the cursor with it), with `?47`/
`?1047` as the older forms.

**2. A constructor that takes a stream, not a shell.** `TermC` can only fork its own process:
`startPTY(shell, env, rows, cols)` (`term/pty.go:26`) is the only path in, and `Shell(path)`
takes a binary with no arguments. recap's pty lives in another process behind a socket and
outlives the TUI, so there is nothing for `TermC` to fork. The reader loop, the grid and the
blit already work in terms of bytes; only the source is hardcoded. The shape:

```go
// Stream drives the terminal from an existing byte stream instead of forking a
// shell. Output is read from rw; keys are written to it. onResize is called with
// the new cell geometry when the layout box changes, in place of the TIOCSWINSZ
// the component issues when it owns the pty.
//
// onResize fires on the RENDER goroutine, inside Execute. It must not block: a
// drag-resize calls it on consecutive frames, and an implementation that writes
// to a socket stalls the frame when the far side is slow or backpressured. It
// may only enqueue; coalesce latest-wins and send on your own goroutine.
func Stream(rw io.ReadWriter, onResize func(rows, cols uint16)) *TermC
```

`New()` becomes `Stream` over a locally forked pty, so there is one reader loop, not two.

**The goroutine contract is part of the API, not a footnote.** Every other callback on `TermC`
states where it fires and the rule that follows: `OnExit`, `OnTitle` and `OnUpdate` all fire on
the pty reader goroutine and say to marshal bound state with `App.Apply` (`term/term.go:82-95`).
`onResize` is the one that fires on the render goroutine, because `syncFrame` is the Layer's
per-frame callback (`term/term.go:139`) and `resizeIfNeeded` runs inside it. Today that
terminates in a local ioctl that cannot meaningfully block; under `Stream` it terminates in
whatever the consumer passes. The contract is stated rather than defended against: glyph does
not grow a goroutine and a queue inside every `TermC` to protect against one caller's IO.

The reply path needs the writer goroutine, and it lands with `Stream`. The screen answers
cursor-position and device-attribute queries by writing from the reader goroutine, and the note
at `term/term.go:166` records that this is safe only while replies are small and reactive, with
a writer goroutine as the fix if that stops holding. A socket-backed stream is the case that
makes it stop holding, so the reader's only job becomes reading.

**Closing and exiting are part of this decision, not separate ones.** A stream-backed `TermC`
owns no process: `Close` closes the stream and stops the reader, where `pty.close()` today does
`Kill` then `Wait` (`term/pty.go:73`). A recap detach must leave the agent running. `OnExit`
then needs a meaning it does not have today, and the current one is not merely undefined but
wrong: `Close` fires `OnExit` already. Probed on the branch against a real shell, a deliberate
`Close()` fires `OnExit` with `err = EOF`, because `Close` closes the pty master, the blocking
`Read` returns, and the error path fires `onExit(err)` (`term/term.go:249`). The component
cannot tell a teardown it performed itself from the far side going away.

The contract: **`Close` sets a closing flag before it closes the stream, and `readLoop` checks
it on the error path, so a deliberate `Close` is silent. `OnExit(err)` fires only when the far
side goes away on its own — reader EOF or a read error — carrying that error, with `io.EOF`
meaning a clean far-side exit.** It still fires on the reader goroutine, so the existing rule
about marshalling bound state with `App.Apply` is unchanged. That gives the consumer a signal a
local teardown cannot forge, which recap needs: a crashed agent and a human detach both end at
EOF on the socket otherwise, and the pane tears down differently for each. It is a behaviour fix
in its own right, not a consequence of the constructor, which is why it is a separate slice.

**3. riffkey decodes `0x1c`-`0x1f` as Ctrl.** `parseSingleByte` maps only `b < 27` to
`ModCtrl` (`riffkey/riffkey.go:1928`), so `0x1c` surfaces as `Key{Rune: 0x1c}` with no
modifier and a `<C-\>` pattern never matches. A host that wants Ctrl-\ as a binding, which is
exactly what recap's detach becomes, has to match a bare control rune and hope. `0x1c`-`0x1f`
decode as Ctrl-\, Ctrl-], Ctrl-^ and Ctrl-_. `encodeKey` already round-trips them correctly
(`term/input.go:67`), so the terminal path is unaffected either way.

Because:

* the component is one constructor away from being reusable, and the missing piece is the
  boundary between "renders a terminal" and "renders a shell it forked". A component that can
  only drive a process it started is a shell widget, not a terminal.
* option 1 duplicates a VT interpreter in recap, where it would be tested against one agent
  and rot. glyph already has the grid, the SGR, the cursor and the blit.

## Technical

The three surfaces the branch already gets right, and that the above does not disturb: input
is content-blind (`HandleKey` encodes a key and always reports it consumed, `term/input.go:15`,
so the host arms the focused pane with `router.HandleUnmatched` and keeps its own bindings
above it); size follows the layout box (`syncFrame`, `term/term.go:140`); and callbacks fire on
the reader goroutine, so bound state is marshalled with `App.Apply` (`term/term.go:85`).

Mouse reporting, bracketed paste and focus reporting stay unimplemented for now. A program that
enables them and gets no reports falls back to plain encodings, which is correct rather than
broken. What makes that safe is the parser, not `privateMode`: an unimplemented sequence has to
be *swallowed*, and until recently the `<` and `>` forms were not. `csi()` treated only `?` as a
parameter prefix, so the agent's `ESC[<u`, `ESC[>1u` and `ESC[>4;2m` aborted mid-sequence and
their parameters printed into the grid as text; the pane would have opened with `1u` and `4;2m`
on row 0. `dispatchCSI` now consumes the whole ECMA-48 prefix range `0x3c-0x3f` and ignores the
prefixes it does not implement (`5a2b222`, regression test `TestCSIParameterPrefixSwallowed` in
`term/vt_test.go`). That is the class this work must keep closed.

## Non-goals

* Scrollback, copy mode, sessions, detach/reattach persistence.
* Mouse, bracketed paste and focus reporting as working features.
* Driving `TermC` from a `ForEach`. The compiled op captures the layer pointer, so a pane grid
  needs a fixed slot pool (`examples/term/main.go:27`). That constraint stays.
* Merging the branch as-is. The slices land on it; the merge is its own call.

## Risks

* A VT interpreter is a correctness surface with no fallback: what it renders wrong, the human
  sees wrong. The alternate-screen slice should be tested against a real full-screen program,
  not a synthetic byte sequence.
* `Stream` puts consumer IO on two glyph-owned goroutines: the resize callback on the render
  goroutine and the query replies on the reader goroutine. Both are safe only while the write
  cannot block, and a socket has different blocking behaviour to a pty master. The reply path is
  answered by the writer goroutine landing with `Stream`; the resize path is answered by the
  stated contract. A consumer that ignores that contract stalls frames, and it will read as
  "glyph is slow" rather than "my callback blocked". That is the cost of stating it instead of
  defending against it, and it is why it belongs in the doc comment.

## Agreed todos

- [ ] a full-screen program running in a term pane renders correctly, because the screen implements the alternate buffer
- [ ] term drives an existing byte stream through a Stream constructor, so a caller can host a pty it does not own and closing the pane leaves that process running
- [ ] a stream-backed term reports the far side going away through OnExit, and never blocks a frame on consumer IO
- [ ] riffkey decodes 0x1c-0x1f as Ctrl-\, Ctrl-], Ctrl-^ and Ctrl-_ so they can be bound as keys

## End-goal state

`term` renders any pty-shaped byte stream, not only a shell it forked. `term.Stream(rw,
onResize)` hosts a terminal whose process lives elsewhere; closing it closes the stream and
leaves that process running, and `OnExit` fires only when the far side goes away, carrying the
read error that says so. `onResize` is documented as firing on the render goroutine and as
non-blocking, and the reply path writes from its own goroutine, so no consumer IO sits on
glyph's render or reader path. A full-screen program in the pane renders correctly, because the
screen implements the alternate buffer. riffkey decodes `0x1c`-`0x1f` as Ctrl keys, so a host
can bind Ctrl-\ above the pane. The example still runs a local shell, through the same reader
loop.
