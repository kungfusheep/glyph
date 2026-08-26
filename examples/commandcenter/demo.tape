# commandcenter demo recording
# run from tui dir: vhs cmd/commandcenter/demo.tape

Output cmd/commandcenter/demo.gif
Output cmd/commandcenter/demo.mp4

Set Shell zsh
Set FontFamily "Berkeley Mono"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 20
Set WindowBar Colorful
Set WindowBarSize 40
Set TypingSpeed 0ms
Set Theme "Builtin Dark"

# build and launch
Hide
Type "go build ./cmd/commandcenter/ && ./commandcenter"
Enter
Sleep 500ms
Show

# let the degraded state breathe — sparklines filling, logs scrolling
Sleep 4s

# expand sparklines to show the damage
Ctrl+S
Sleep 3s

# collapse sparklines
Ctrl+S
Sleep 1s

# expand log to see the 503s
Ctrl+L
Sleep 3s

# collapse log
Ctrl+L
Sleep 1s

# navigate to redis-cluster (it's the 3rd item)
Down
Down
Sleep 500ms

# inspect redis
Enter
Sleep 2s

# restart it
Type "r"
Sleep 3s

# modal closes automatically via OnComplete
# watch the recovery — sparklines climb back, logs go green
Sleep 5s

# expand sparklines to show healthy state
Ctrl+S
Sleep 3s

# collapse
Ctrl+S
Sleep 1s

# theme cycling showcase
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s
Ctrl+T
Sleep 1.5s

# back to rose piné
Sleep 1s

# quit
Ctrl+C
