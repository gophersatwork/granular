package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/gophersatwork/granular"
)

// TestResult stores the cached test execution result
type TestResult struct {
	ExitCode    int       `json:"exit_code"`
	Output      string    `json:"output"`
	Duration    string    `json:"duration"`
	Timestamp   time.Time `json:"timestamp"`
	TotalTests  int       `json:"total_tests"`
	PassedTests int       `json:"passed_tests"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Running Tests WITH Granular Caching")
	fmt.Println("========================================")
	fmt.Println()

	// Initialize cache
	cacheDir := ".granular-test-cache"
	cache, err := granular.Open(cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize cache: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	// Build cache key from all source and test files
	key := cache.Key().
		File("app/calculator.go").
		File("app/database.go").
		File("app/calculator_test.go").
		File("app/database_test.go").
		Version("v1").
		Build()

	keyHash := key.Hash()
	displayHash := keyHash
	if len(keyHash) > 32 {
		displayHash = keyHash[:32] + "..."
	}
	fmt.Printf("Cache key hash: %s\n", displayHash)
	fmt.Println()

	// Check cache for existing results
	result := cache.Get(key)

	if result != nil {
		// Cache hit - restore and display cached results
		fmt.Println("✓ Cache HIT - Restoring previous test results")
		fmt.Println()

		// Retrieve cached test result data
		resultData := result.Bytes("result")
		if resultData == nil {
			fmt.Fprintf(os.Stderr, "Cache corrupted: no result data found\n")
			os.Exit(1)
		}

		var testResult TestResult
		if err := json.Unmarshal(resultData, &testResult); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unmarshal cached results: %v\n", err)
			os.Exit(1)
		}

		// Display cached output
		fmt.Println(testResult.Output)

		fmt.Println()
		fmt.Println("========================================")
		fmt.Printf("Cached Result from: %v\n", testResult.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("Original Duration:  %v\n", testResult.Duration)
		fmt.Printf("Cache Age:          %v\n", result.Age())
		fmt.Printf("Cache retrieval:    ~instant\n")
		fmt.Println("========================================")

		os.Exit(testResult.ExitCode)
	}

	// Cache miss - run tests
	fmt.Println("✗ Cache MISS - Running tests...")
	fmt.Println()

	startTime := time.Now()

	// Run tests with verbose output and capture output
	cmd := exec.Command("go", "test", "-v", "./app/...")
	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outputBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outputBuf)
	cmd.Env = os.Environ()

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	elapsed := time.Since(startTime)

	// Create result object
	testResult := TestResult{
		ExitCode:    exitCode,
		Output:      outputBuf.String(),
		Duration:    elapsed.String(),
		Timestamp:   time.Now(),
		TotalTests:  0, // Could parse from output
		PassedTests: 0, // Could parse from output
	}

	// Store result in cache
	resultData, err := json.Marshal(testResult)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal test results: %v\n", err)
		os.Exit(1)
	}

	// Cache the test result
	if err := cache.Put(key).
		Bytes("result", resultData).
		Meta("exit_code", fmt.Sprintf("%d", exitCode)).
		Meta("duration", elapsed.String()).
		Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cache test results: %v\n", err)
		// Don't exit - tests ran successfully, just caching failed
	}

	fmt.Println()
	fmt.Println("========================================")
	if exitCode != 0 {
		fmt.Printf("Tests FAILED in %v (cached for future runs)\n", elapsed)
		fmt.Println("========================================")
		os.Exit(exitCode)
	}

	fmt.Printf("Tests PASSED in %v (cached for future runs)\n", elapsed)
	fmt.Println("========================================")
}
