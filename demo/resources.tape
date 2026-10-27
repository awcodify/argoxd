# Records demo/resources.gif from the built-in sample data: `make demo-resources`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs

Output demo/resources.gif

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

# Drill down into billing-worker: resources are shown as nested cards.
Sleep 600ms
Enter
Sleep 3s

# w switches to a structured table view of the same resources.
Type "w"
Sleep 3.5s

# w returns to the card view.
Type "w"
Sleep 2.5s

# E opens the live events pane, tracking the selected resource.
Type "E"
Sleep 2.5s

# Moving the cursor reloads the events pane for the newly selected Pod.
Down@600ms 2
Sleep 2.2s
Down@600ms 3
Sleep 2.5s

# z folds all cards down to top-level resources, showing folded counts.
Type "z"
Sleep 3s

# z unfolds everything back.
Type "z"
Sleep 2.5s
