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

# The overview: what is healthy, what is not, and what needs attention.
Type ":pulse"
Enter
Sleep 3.5s

# Open the worst Application. Its conditions are listed above the cards.
Enter
Sleep 3s

# Why did its last sync fail?
Type "i"
Sleep 3s
Escape
Sleep 500ms

# Walk down to a failing Pod. Its events show why it keeps restarting.
Down 5
Sleep 1s
Type "e"
Sleep 3s
Escape
Sleep 500ms

# Read its logs, then go back to the overview.
Type "l"
Sleep 3s
Escape
Sleep 500ms
Escape
Sleep 1s

# Let this Application sync by itself, and heal drift.
Type "P"
Sleep 1.5s
Type "a"
Sleep 700ms
Type "h"
Sleep 1.5s
Enter
Sleep 2s

# Back to the Applications list, filtered by health.
Type ":app"
Enter
Sleep 1.5s
Type "H"
Sleep 600ms
Type "Degraded"
Sleep 600ms
Enter
Sleep 1.5s
Escape
Sleep 1s

# Mark the OutOfSync Applications and sync them together.
Type "S"
Sleep 600ms
Type "OutOfSync"
Sleep 600ms
Enter
Sleep 1.5s
Space
Sleep 500ms
Space
Sleep 1s
Type "s"
Sleep 2s
Enter
Sleep 2.5s

Type "q"
Sleep 500ms
