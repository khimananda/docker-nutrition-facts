# Render with: vhs demo/demo.tape  (https://github.com/charmbracelet/vhs)
# Build the scary image first: docker build -t nutrition-demo:scary demo/scary
Output demo/demo.gif
Set Shell zsh
Set FontSize 16
Set Width 1000
Set Height 820
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Type "docker nutrition nginx:latest"
Enter
Sleep 5s
Type "clear" Enter
Type "docker nutrition nutrition-demo:scary --local"
Enter
Sleep 7s
