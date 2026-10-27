# Records demo/inspect.gif from the built-in sample data: `make demo-inspect`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs

Output demo/inspect.gif

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

# Open billing-worker, then read the selected resource's live YAML.
Enter
Sleep 2.5s
Down@600ms 3
Sleep 1.5s
Type "Y"
Sleep 3.5s
Escape
Sleep 1.5s

# d diffs the live resource against Git.
Type "d"
Sleep 4s
Escape
Sleep 1.5s

# l follows the logs of a Pod.
Down@600ms 2
Sleep 1.5s
Type "l"
Sleep 4s
Escape
Sleep 2s
