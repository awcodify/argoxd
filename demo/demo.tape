# Records demo/demo.gif from the built-in sample data: `make demo`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs

Output demo/demo.gif

Require argoxd

Set Shell "bash"
Set FontSize 14
Set Width 1500
Set Height 850
Set Padding 12
Set TypingSpeed 80ms

Type "argoxd --demo"
Enter
Sleep 2s

# Filter the Applications by health.
Type "H"
Sleep 600ms
Type "Degraded"
Sleep 600ms
Enter
Sleep 1.5s

# Open the first match and walk down to a failing Pod.
Enter
Sleep 2s
Down 5
Sleep 1s

# Read its logs, then go back out (the last Esc clears the filter).
Type "l"
Sleep 3s
Escape
Sleep 500ms
Escape
Sleep 500ms
Escape
Sleep 1s

# Sync an OutOfSync Application.
Type "S"
Sleep 600ms
Type "OutOfSync"
Sleep 600ms
Enter
Sleep 1.5s
Type "s"
Sleep 1.5s
Enter
Sleep 2.5s

Type "q"
Sleep 500ms
