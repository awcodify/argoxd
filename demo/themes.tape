# Records demo/themes.gif from the built-in sample data: `make demo`.
# Requires vhs (https://github.com/charmbracelet/vhs): brew install vhs

Output demo/themes.gif

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

# Open the theme picker: each theme previews instantly as you move through them.
Type ":theme"
Sleep 800ms
Enter
Sleep 2s

# light: paints a light background readable on any terminal.
Tab
Sleep 2.2s

# high-contrast
Tab
Sleep 2s

# colorblind: accessible blue and vermillion status colors.
Tab
Sleep 2s

# nord: cool arctic tones.
Tab
Sleep 2s

# dracula: vibrant dark palette.
Tab
Sleep 2s

# Enter keeps the chosen theme.
Enter
Sleep 2.5s

# Open an Application to see the theme across resource cards and status indicators.
Enter
Sleep 3.5s

# Switch to table view in the new theme.
Type "w"
Sleep 3.5s
