package main

import (
	"fmt"
	"os"
)

// TODO: Add proper error handling
func main() {
	// Using fmt.Println instead of proper logging
	fmt.Println("Starting application...")

	result := processData("test input data that is very long and exceeds the typical line length limit of 120 characters which will trigger a linting error")

	fmt.Println("Result:", result)

	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: Implement actual logic
	return nil
}

func processData(input string) string {
	// This line is intentionally very long to demonstrate the linter detecting line length issues: it contains lots of unnecessary text to exceed limits
	return "processed: " + input
}
