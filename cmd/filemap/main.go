package main

import (
	"fmt"
	"os"

	"github.com/hayeah/fork2"
)

func main() {
	// Create a CLI app for filemap
	app, err := fork2.InitFileMapCLI()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing filemap CLI: %v\n", err)
		os.Exit(1)
	}

	// Run the app
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
