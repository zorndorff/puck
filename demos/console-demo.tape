# Puck Console Demo - Shows persistence
# Run: vhs demos/console-demo.tape

Output demos/console-demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 900
Set Height 550
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Framerate 30
Set TypingSpeed 40ms

Type "# Create a sprite for development"
Enter
Sleep 500ms

Type "./puck create devbox"
Enter
Sleep 4s

Type "# Open a console inside"
Enter
Sleep 500ms

Type "./puck console devbox"
Enter
Sleep 2s

Type "# We're inside Fedora now"
Enter
Sleep 300ms

Type "cat /etc/os-release | head -2"
Enter
Sleep 1500ms

Type "# Install htop - it will persist!"
Enter
Sleep 300ms

Type "dnf install -y htop 2>&1 | tail -5"
Enter
Sleep 5s

Type "htop --version"
Enter
Sleep 1s

Type "exit"
Enter
Sleep 1s

Type "# Exit and re-enter - package is still there"
Enter
Sleep 500ms

Type "./puck console devbox"
Enter
Sleep 2s

Type "htop --version"
Enter
Sleep 1500ms

Type "# State persists across sessions!"
Enter
Sleep 500ms

Type "exit"
Enter
Sleep 1s

Type "# Clean up"
Enter
Sleep 300ms

Type "./puck destroy devbox --force"
Enter
Sleep 2s
