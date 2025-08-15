package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

// This is a simple wrapper that redirects to the actual main in cmd/server
func main() {
	// Get the directory of this file
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}

	// Path to the actual server binary
	serverPath := filepath.Join(dir, "cmd", "server", "main.go")
	
	// Execute the actual server with all arguments
	cmd := exec.Command("go", append([]string{"run", serverPath}, os.Args[1:]...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	err = cmd.Run()
	if err != nil {
		os.Exit(1)
	}
}