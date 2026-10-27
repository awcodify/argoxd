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
Sleep 2.5s

# Worst first: sort by health. The column header shows the sort.
Type ":sort health"
Sleep 900ms
Enter
Sleep 2.5s

# The overview: what is healthy, what is not, and what needs attention.
Type ":pulse"
Sleep 800ms
Enter
Sleep 2.8s

# Open the worst Application. Its conditions are listed above the cards.
Sleep 800ms
Enter
Sleep 2.5s

# The same resources as a table, then back to cards.
Type "w"
Sleep 2.5s
Type "w"
Sleep 2s

# Keep the selected resource's events in a pane that follows the cursor.
Sleep 800ms
Type "E"
Sleep 2.2s
Down@600ms 2
Sleep 1.8s
Down@600ms 3
Sleep 2.2s

# Fold everything down to the top-level resources, then open it up again.
Type "z"
Sleep 2.2s
Type "z"
Sleep 1.8s

# Why did its last sync fail?
Sleep 600ms
Type "i"
Sleep 2.5s
Escape
Sleep 1.2s

# Read its logs, then go back to the overview.
Type "l"
Sleep 2.5s
Escape
Sleep 1s
Escape
Sleep 1.5s

# Try themes: the screen repaints as you move, Esc brings the original back.
Type ":theme"
Sleep 800ms
Enter
Sleep 2s
Tab
Sleep 1.8s
Tab
Sleep 1.8s
Tab
Sleep 1.8s
Tab
Sleep 1.8s
Escape
Sleep 1.8s

# Mark the OutOfSync Applications and sync them together.
Type ":app"
Sleep 700ms
Enter
Sleep 1.5s
Type "S"
Sleep 1s
Type "OutOfSync"
Sleep 900ms
Enter
Sleep 2s
Space
Sleep 1s
Down
Sleep 800ms
Space
Sleep 1.5s
Type "s"
Sleep 2.5s
Enter
Sleep 3s
