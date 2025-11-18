package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("Running Tests WITHOUT Caching")
	fmt.Println("========================================")
	fmt.Println()

	startTime := time.Now()

	// Run tests with verbose output
	cmd := exec.Command("go", "test", "-v", "./app/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Run()

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Println("========================================")
	if err != nil {
		fmt.Printf("Tests FAILED in %v\n", elapsed)
		fmt.Println("========================================")
		os.Exit(1)
	}

	fmt.Printf("Tests PASSED in %v\n", elapsed)
	fmt.Println("========================================")
}
