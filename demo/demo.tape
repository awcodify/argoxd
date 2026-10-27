# Records demo/demo.gif from the built-in sample data: `make demo`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs
#
# --show-keys captions each key in a band across the middle of the screen, so
# the recording shows what is being pressed. The pauses are meant to feel like
# a person reading the screen, then deciding what to do next.

Output demo/demo.gif

Require argoxd

Set Shell "bash"
Set FontSize 14
Set Width 1500
Set Height 850
Set Padding 12
Set Framerate 25
Set TypingSpeed 140ms

# Ignore any config file of the person recording, so the demo always looks the same.
Env ARGOXD_CONFIG "/dev/null"

Sleep 1s
Type "argoxd --demo --show-keys"
Sleep 900ms
Enter
Sleep 3s

# Worst first: sort by health. The column header shows the sort.
Type ":sort health"
Sleep 900ms
Enter
Sleep 4s

# The overview: what is healthy, what is not, and what needs attention.
Type ":pulse"
Sleep 800ms
Enter
Sleep 5s

# Open the worst Application. Its conditions are listed above the cards.
Sleep 800ms
Enter
Sleep 5s

# The same resources as a table, then back to cards.
Type "w"
Sleep 4.5s
Type "w"
Sleep 2.5s

# Keep the selected resource's events in a pane that follows the cursor.
Sleep 800ms
Type "E"
Sleep 3s
Down@600ms 2
Sleep 2.2s
Down@600ms 3
Sleep 3.5s

# Fold everything down to the top-level resources, then open it up again.
Type "z"
Sleep 3.5s
Type "z"
Sleep 2.5s

# Why did its last sync fail?
Sleep 600ms
Type "i"
Sleep 5s
Escape
Sleep 1.5s

# Read its logs, then go back to the overview.
Type "l"
Sleep 5s
Escape
Sleep 1.2s
Escape
Sleep 2s

# Try themes: the screen repaints as you move, Esc brings the original back.
Type ":theme"
Sleep 800ms
Enter
Sleep 2.5s
Tab
Sleep 2.5s
Tab
Sleep 2.5s
Tab
Sleep 2.5s
Tab
Sleep 2.5s
Escape
Sleep 2.5s

# Mark the OutOfSync Applications and sync them together.
Type ":app"
Sleep 700ms
Enter
Sleep 2s
Type "S"
Sleep 1s
Type "OutOfSync"
Sleep 900ms
Enter
Sleep 2.5s
Space
Sleep 1.2s
Down
Sleep 900ms
Space
Sleep 2s
Type "s"
Sleep 4s
Enter
Sleep 4s
