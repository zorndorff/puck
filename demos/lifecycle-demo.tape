# Puck Lifecycle Demo - Start/Stop workflow
# Run: vhs demos/lifecycle-demo.tape

Output demos/lifecycle-demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 900
Set Height 450
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Framerate 30
Set TypingSpeed 50ms

Type "# Create two sprites"
Enter
Sleep 300ms

Type "./puck create app1"
Enter
Sleep 4s

Type "./puck create app2"
Enter
Sleep 4s

Type "# View all sprites"
Enter
Sleep 300ms

Type "./puck list"
Enter
Sleep 2s

Type "# Stop one sprite"
Enter
Sleep 300ms

Type "./puck stop app1"
Enter
Sleep 2s

Type "./puck list"
Enter
Sleep 1500ms

Type "# Start it back"
Enter
Sleep 300ms

Type "./puck start app1"
Enter
Sleep 2s

Type "./puck list"
Enter
Sleep 1500ms

Type "# Destroy all sprites at once"
Enter
Sleep 300ms

Type "./puck destroy --all --force"
Enter
Sleep 2s

Type "./puck list"
Enter
Sleep 2s
