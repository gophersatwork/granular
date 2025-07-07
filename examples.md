# Examples of use cases for `granular`

This is an incremental file cache library for Go applications that provides deterministic, content-based caching.

You might be wondering: What can I do with it? This document sheds some light on this topic.

The main use cases for this codebase include:

## 1. Build System Caching

Cache build artifacts based on source file content to avoid unnecessary recompilation.

```go
func main() {
	// Initialize cache in project directory
	cacheDir := ".build-cache"
	cache, err := granular.New(cacheDir)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define cache key based on source files
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source files
			granular.GlobInput{Pattern: "*.go"},
			// Track build configuration
			granular.FileInput{Path: "go.mod"},
			granular.FileInput{Path: "go.sum"},
		},
		Extra: map[string]string{
			"go_version": os.Getenv("GO_VERSION"),
			"os":         os.Getenv("GOOS"),
			"arch":       os.Getenv("GOARCH"),
		},
	}

	// Check if we have a cached build
	result, hit, err := cache.Get(key)
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached build artifact")
		// Copy the cached binary to the target location
		if err := copyFile(result.Path, "myapp"); err != nil {
			log.Fatalf("Failed to copy cached binary: %v", err)
		}
	} else {
		fmt.Println("Building from source...")

		// Run the build command
		cmd := exec.Command("go", "build", "-o", "myapp")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Build failed: %v", err)
		}

		// Store the build artifact in cache
		result := granular.Result{
			Path: "myapp",
			Metadata: map[string]string{
				"build_time": fmt.Sprintf("%d", time.Now().Unix()),
			},
		}

		if err := cache.Store(key, result); err != nil {
			log.Printf("Warning: Failed to cache build artifact: %v", err)
		}
	}
}
```

## 2. Content-Based File Caching

Store and retrieve files based on their content hash

```go
func main() {
	// Initialize cache
	cache, err := granular.New(".file-cache")
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// File to cache
	filePath := "large-dataset.csv"

	// Create a key based solely on the file content
	key := granular.Key{
		Inputs: []granular.Input{
			granular.FileInput{Path: filePath},
		},
	}

	// Try to get the processed version from cache
	processedFilePath := filePath + ".processed"
	result, hit, err := cache.GetFile(key, filepath.Base(processedFilePath))
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached processed file")
		// Copy the cached file to the target location
		if err := copyFile(result, processedFilePath); err != nil {
			log.Fatalf("Failed to copy cached file: %v", err)
		}
	} else {
		fmt.Println("Processing file...")

		// Process the file (just copy it for demonstration)
		if err := processFile(filePath, processedFilePath); err != nil {
			log.Fatalf("Processing failed: %v", err)
		}

		// Store the processed file in cache
		result := granular.Result{
			Path: processedFilePath,
		}

		if err := cache.Store(key, result); err != nil {
			log.Printf("Warning: Failed to cache processed file: %v", err)
		}
	}
}
```

## 3. Incremental Computation

Only recompute results when inputs have changed

```go
func main() {
	// Initialize cache
	cache, err := granular.New(".computation-cache")
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define computation inputs
	inputDir := "data"
	configFile := "config.json"

	// Create a key based on all input files and configuration
	key := granular.Key{
		Inputs: []granular.Input{
			granular.DirectoryInput{
				Path:    inputDir,
				Exclude: []string{"*.tmp", "*.log"},
			},
			granular.FileInput{Path: configFile},
		},
		Extra: map[string]string{
			"computation_version": "1.2.3",
		},
	}

	// Try to get the result from cache
	outputFile := "results.json"
	result, hit, err := cache.Get(key)
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached computation result")
		// Copy the cached result to the expected location
		if err := copyFile(result.Path, outputFile); err != nil {
			log.Fatalf("Failed to copy cached result: %v", err)
		}

		// Get additional metadata
		if completionTime, found, _ := cache.GetData(key, "completion_time"); found {
			fmt.Printf("Computation was originally completed at: %s\n", string(completionTime))
		}
	} else {
		fmt.Println("Performing computation...")

		// Perform the computation
		if err := performComputation(inputDir, configFile, outputFile); err != nil {
			log.Fatalf("Computation failed: %v", err)
		}

		// Store the result in cache
		result := granular.Result{
			Path: outputFile,
			Metadata: map[string]string{
				"completion_time": time.Now().Format(time.RFC3339),
				"input_count":     fmt.Sprintf("%d", countFiles(inputDir)),
			},
		}

		if err := cache.Store(key, result); err != nil {
			log.Printf("Warning: Failed to cache computation result: %v", err)
		}
	}
}
```

## 4. Artifact Generation

Cache generated files to avoid regeneration when inputs haven't changed.

```go
func main() {
	// Initialize cache
	cache, err := granular.New(".artifact-cache")
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define inputs for artifact generation
	schemaFile := "schema.proto"

	// Create a key based on the schema file and generator version
	key := granular.Key{
		Inputs: []granular.Input{
			granular.FileInput{Path: schemaFile},
		},
		Extra: map[string]string{
			"generator_version": "2.0.1",
			"language":          "go",
		},
	}

	// Try to get the generated code from cache
	outputFile := "generated.go"
	result, hit, err := cache.Get(key)
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached generated artifact")
		// Copy the cached artifact to the expected location
		if err := copyFile(result.Path, outputFile); err != nil {
			log.Fatalf("Failed to copy cached artifact: %v", err)
		}
	} else {
		fmt.Println("Generating artifact...")

		// Generate the artifact
		if err := generateArtifact(schemaFile, outputFile); err != nil {
			log.Fatalf("Generation failed: %v", err)
		}

		// Store the artifact in cache
		result := granular.Result{
			Path: outputFile,
			Metadata: map[string]string{
				"generation_time": time.Now().Format(time.RFC3339),
			},
		}

		if err := cache.Store(key, result); err != nil {
			log.Printf("Warning: Failed to cache generated artifact: %v", err)
		}
	}
}
```


## 5. Local Development Optimization

Speed up local development workflows by caching expensive operations

```go
func main() {
	// Initialize cache
	cache, err := granular.New(".dev-cache")
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define inputs for the development task
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source files
			granular.GlobInput{Pattern: "src/**/*.go"},
			// Track configuration files
			granular.GlobInput{Pattern: "config/*.yaml"},
			// Track dependencies
			granular.FileInput{Path: "go.mod"},
			granular.FileInput{Path: "go.sum"},
		},
		Extra: map[string]string{
			"task": "lint",
		},
	}

	// Check if we have a cached result
	result, hit, err := cache.Get(key)
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached lint results")

		// Get the lint status from metadata
		if lintStatus, found, _ := cache.GetData(key, "lint_status"); found {
			fmt.Printf("Lint status: %s\n", string(lintStatus))
		}

		// Get the lint output from the cached file
		if result.Path != "" {
			data, err := os.ReadFile(result.Path)
			if err == nil {
				fmt.Printf("Lint output:\n%s\n", string(data))
			}
		}
	} else {
		fmt.Println("Running linter...")

		// Run the linter and capture output
		lintOutputFile := "lint-output.txt"
		cmd := exec.Command("golangci-lint", "run", "./...")
		lintOutput, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal("Failed to run linter:", err)
		}
		// Save the output to a file
		if err := os.WriteFile(lintOutputFile, lintOutput, 0644); err != nil {
			log.Fatalf("Failed to write lint output: %v", err)
		}

		// Determine lint status
		lintStatus := "pass"
		if cmd.ProcessState.ExitCode() != 0 {
			lintStatus = "fail"
		}

		// Store the result in cache
		result := granular.Result{
			Path: lintOutputFile,
			Metadata: map[string]string{
				"lint_status": lintStatus,
				"exit_code":   fmt.Sprintf("%d", cmd.ProcessState.ExitCode()),
			},
		}

		if err := cache.Store(key, result); err != nil {
			log.Printf("Warning: Failed to cache lint result: %v", err)
		}

		fmt.Printf("Lint status: %s\n", lintStatus)
		fmt.Printf("Lint output:\n%s\n", string(lintOutput))
	}
}

```

## 6. Testing and CI Optimization

Reduce build and test times in CI environments
```go
    func main() {
    // Initialize cache
    cacheDir := os.Getenv("CI_CACHE_DIR")
    if cacheDir == "" {
    cacheDir = ".ci-cache"
    }

	cache, err := granular.New(cacheDir)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define inputs for the CI task
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source and test files
			granular.GlobInput{Pattern: "**/*.go"},
			// Track configuration files
			granular.FileInput{Path: ".github/workflows/ci.yml"},
			granular.FileInput{Path: "go.mod"},
			granular.FileInput{Path: "go.sum"},
		},
		Extra: map[string]string{
			"go_version": os.Getenv("GO_VERSION"),
			"os":         os.Getenv("RUNNER_OS"),
			"task":       "test",
		},
	}

	// Check if we have a cached result
	_, hit, err := cache.Get(key)
	if err != nil && errors.Is(err, granular.ErrCacheMiss) {
		log.Fatalf("Cache error: %v", err)
	}

	if hit {
		fmt.Println("Using cached test results")

		// Get the test status from metadata
		if testStatus, found, _ := cache.GetData(key, "test_status"); found {
			fmt.Printf("Test status: %s\n", string(testStatus))

			// If tests passed in the cached run, we can skip running them again
			if string(testStatus) == "pass" {
				fmt.Println("Tests passed in previous run, skipping...")
				os.Exit(0)
			}
		}
	}

	// Run the tests
	fmt.Println("Running tests...")
	testOutputFile := "test-output.txt"
	cmd := exec.Command("go", "test", "./...", "-v")
	testOutput, err := cmd.CombinedOutput()

	// Save the output to a file
	if err := os.WriteFile(testOutputFile, testOutput, 0644); err != nil {
		log.Fatalf("Failed to write test output: %v", err)
	}

	// Determine test status
	testStatus := "pass"
	if cmd.ProcessState.ExitCode() != 0 {
		testStatus = "fail"
	}

	// Store the result in cache
	res := granular.Result{
		Path: testOutputFile,
		Metadata: map[string]string{
			"test_status": testStatus,
			"exit_code":   fmt.Sprintf("%d", cmd.ProcessState.ExitCode()),
			"test_count":  countTests(string(testOutput)),
		},
	}

	if err := cache.Store(key, res); err != nil {
		log.Printf("Warning: Failed to cache test result: %v", err)
	}

	fmt.Printf("Test status: %s\n", testStatus)
	fmt.Println(string(testOutput))

	// Exit with the same code as the test command
	os.Exit(cmd.ProcessState.ExitCode())
}


```

## 7. Data Processing Pipelines

Cache intermediate results in data processing pipelines

```go
func main() {
	// Initialize cache
	cache, err := granular.New(".pipeline-cache")
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// Define pipeline stages
	stages := []struct {
		name    string
		input   string
		output  string
		process func(input, output string) error
	}{
		{
			name:    "extract",
			input:   "raw-data.csv",
			output:  "extracted-data.json",
			process: extractData,
		},
		{
			name:    "transform",
			input:   "extracted-data.json",
			output:  "transformed-data.json",
			process: transformData,
		},
		{
			name:    "load",
			input:   "transformed-data.json",
			output:  "final-output.json",
			process: loadData,
		},
	}

	// Process each stage
	for i, stage := range stages {
		fmt.Printf("Stage %d: %s\n", i+1, stage.name)

		// Create a key for this stage
		var inputs []granular.Input

		// If this is the first stage, use the raw input file
		if i == 0 {
			inputs = []granular.Input{
				granular.FileInput{Path: stage.input},
			}
		} else {
			// Otherwise, use the output from the previous stage
			inputs = []granular.Input{
				granular.FileInput{Path: stages[i-1].output},
			}
		}

		// Add any configuration files
		configFile := fmt.Sprintf("config/%s.yaml", stage.name)
		if _, err := os.Stat(configFile); err == nil {
			inputs = append(inputs, granular.FileInput{Path: configFile})
		}

		key := granular.Key{
			Inputs: inputs,
			Extra: map[string]string{
				"stage":   stage.name,
				"version": "1.0.0",
			},
		}

		// Try to get the result from cache
		result, hit, err := cache.Get(key)
		if err != nil && errors.Is(err, granular.ErrCacheMiss) {
			log.Fatalf("Cache error in stage %s: %v", stage.name, err)
		}

		if hit {
			fmt.Printf("  Using cached result for stage: %s\n", stage.name)
			// Copy the cached result to the expected location
			if err := copyFile(result.Path, stage.output); err != nil {
				log.Fatalf("Failed to copy cached result for stage %s: %v", stage.name, err)
			}
		} else {
			fmt.Printf("  Processing stage: %s\n", stage.name)

			// Process this stage
			if err := stage.process(stage.input, stage.output); err != nil {
				log.Fatalf("Failed to process stage %s: %v", stage.name, err)
			}

			// Store the result in cache
			result := granular.Result{
				Path: stage.output,
				Metadata: map[string]string{
					"stage":        stage.name,
					"process_time": time.Now().Format(time.RFC3339),
				},
			}

			if err := cache.Store(key, result); err != nil {
				log.Printf("Warning: Failed to cache result for stage %s: %v", stage.name, err)
			}
		}
	}

	fmt.Println("Pipeline completed successfully!")
}
```


### Play by yourself!
Each example demonstrates a specific use case for the `granular` package, showing how to define cache keys based on inputs, check for cache hits, and store results in the cache when needed.

The testable version of these examples are [here](examples_test.go)