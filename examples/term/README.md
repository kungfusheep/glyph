# term — a basic tmux clone, as a glyph example

This example drives the embeddable terminal component: split panes, each hosting a
live pty, rendered inside a glyph layout. The terminal component itself lives in the
glyph core (this worktree's branch) and is the reusable piece; this example is the
proof it embeds like any other component.

Scope, deliberately small:
- vertical/horizontal splits, focus moves between panes
- each pane runs a shell in a pty; bytes in/out, content-blind
- a status line naming the panes

Not scope: sessions, detach/reattach persistence, copy mode, configuration.
