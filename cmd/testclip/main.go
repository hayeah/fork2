package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CopyTextAndImageToClipboard copies both text and image simultaneously using AppleScript
func CopyTextAndImageToClipboard(text string, imagePath string) error {
	// Escape double quotes in the text for AppleScript
	escapedText := strings.ReplaceAll(text, "\"", "\\\"")

	// Create AppleScript that puts both text and image on the clipboard
	script := `
tell application "System Events"
    set theImage to (POSIX file "` + imagePath + `")
    set theText to "` + escapedText + `"

    set the clipboard to {«class PNGf»:(read theImage as «class PNGf»), text:theText}
end tell
`

	// Execute the AppleScript
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("AppleScript error: %v - %s", err, string(output))
	}

	return nil
}

func main() {
	err := CopyTextAndImageToClipboard(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Printf("Error copying to clipboard: %v\n", err)
	} else {
		fmt.Println("Successfully copied text and image to clipboard")
	}
}
