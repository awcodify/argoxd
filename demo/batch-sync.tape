# Records demo/batch-sync.gif from the built-in sample data: `make demo-batch-sync`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs

Output demo/batch-sync.gif

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

# Filter Applications by sync status: OutOfSync.
Type "S"
Sleep 1s
Type "OutOfSync"
Sleep 900ms
Enter
Sleep 2.5s

# Space marks the selected Application.
Space
Sleep 1.2s

# Mark another Application.
Down
Sleep 800ms
Space
Sleep 1.2s

# Mark a third Application.
Down
Sleep 800ms
Space
Sleep 1.8s

# s triggers batch sync: the confirmation dialog lists every marked Application.
Type "s"
Sleep 3s

# Enter confirms and runs the sync on all marked Applications together.
Enter
Sleep 4s
