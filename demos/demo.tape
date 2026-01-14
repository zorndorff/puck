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

Type "# Create a new puck"
Enter
Sleep 300ms

Type "./puck create demo-puck --image fedora:latest"
Enter
Sleep 4s

Type "# List all pucks"
Enter
Sleep 300ms

Type "./puck list"
Enter
Sleep 2s

Type "# Stop the puck"
Enter
Sleep 300ms

Type "./puck stop demo-puck"
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

Type "./puck start demo-puck"
Enter
Sleep 2s

Type "./puck list"
Enter
Sleep 2s

Type "# Clean up"
Enter
Sleep 300ms

Type "./puck destroy demo-puck --force"
Enter
Sleep 2s

Type "# Done! Pucks are persistent containers made easy."
Enter
Sleep 2s
