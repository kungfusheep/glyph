# Soft keyboard demo

A small touch-first app for trying the [Soft Keyboard Protocol (OSC 523)](https://github.com/kungfusheep/OSC-523) in a terminal that implements it.

  <img width="709" height="380" alt="image" src="https://github.com/user-attachments/assets/edc176a9-2019-408a-98cf-66a4df0db20a" />

## Run it

```sh
git clone -b osc-523 https://github.com/kungfusheep/glyph
cd glyph
go run ./examples/softkeyboard
```

Tested with Go 1.27.1 darwin/amd64

## What to try

- **Home** is a tappable list. Taps select rows and press the buttons; the keyboard should stay down. The app sends `control=app:keyboard=hide` at startup.
- **Search** (tap `[ search ]` or press `/`) declares a search keyboard with a Search return key. Enter or Esc goes back, and the keyboard should go down with it.
- **Sign in** (tap `[ sign in ]` or press `l`) is a form with three fields. Tapping a field, or moving with Tab, sends that field's keyboard: email, then a secure keyboard for the password, then numeric with a Go key for the PIN. Enter goes back.

## The wire log

The panel at the top of every view lists the last six sequences across the pty, stamped with milliseconds since startup. A right arrow marks a sequence the app sent, a left arrow one the terminal sent back, colour replies included. The status line says whether the terminal answered the detection query.

```sh
go run ./examples/softkeyboard -log /tmp/kb.log
```

appends the same lines to a file, for reading after a run on a phone.

## Where the client lives

- `softkeyboard.go` sends the reports: `Keyboard(...)` on a view, `Keyboard(...)` on an `Input`, and `ShowKeyboard` / `HideKeyboard` for overlays. It takes control at startup when the mouse is on and hands it back on exit, including on a signal.
- `reply.go` reads the terminal's answers through the input loop, so nothing waits on a reply.
- `focusmanager.go` turns a focus change into a report, and a tap on a managed input focuses it.



