# Puck Demo - Basic Workflow
# Run: vhs demos/demo.tape

Output demos/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Framerate 30
Set TypingSpeed 50ms

Type "# Puck - Persistent Development Containers"
Enter
Sleep 1s

Type "# Check daemon status"
Enter
Sleep 300ms

Type "./puck daemon status"
Enter
Sleep 1500ms

Type "# Create a new sprite"
Enter
Sleep 300ms

Type "./puck create demo-sprite --image fedora:latest"
Enter
Sleep 4s

Type "# List all sprites"
Enter
Sleep 300ms

Type "./puck list"
Enter
Sleep 2s

Type "# Stop the sprite"
Enter
Sleep 300ms

Type "./puck stop demo-sprite"
Enter
Sleep 2s

Type "# Check status again"
Enter
Sleep 300ms

Type "./puck list"
Enter
Sleep 2s

Type "# Start it back up"
Enter
Sleep 300ms

Type "./puck start demo-sprite"
Enter
Sleep 2s

Type "./puck list"
Enter
Sleep 2s

Type "# Clean up"
Enter
Sleep 300ms

Type "./puck destroy demo-sprite --force"
Enter
Sleep 2s

Type "# Done! Sprites are persistent containers made easy."
Enter
Sleep 2s
