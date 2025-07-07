package granular_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

func TestArtifactCache(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheRoot := ".artifact-cache"
	cache, err := granular.New(cacheRoot, granular.WithFs(memFs))
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	schemaFile := "schema.proto"
	err = afero.WriteFile(memFs, schemaFile, []byte("fake schema.proto content"), 0o644)
	if err != nil {
		log.Fatalf("Failed to write schema file: %v", err)
	}

	key := granular.Key{
		Inputs: []granular.Input{
			granular.FileInput{
				Path: schemaFile,
				Fs:   memFs,
			},
		},
		Extra: map[string]string{
			"generator_version": "2.0.1",
			"language":          "go",
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// "Generate" the artifact
	var outputFile string
	if outputFile, err = generateArtifact(memFs, schemaFile); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	// Store the artifact in cache
	result := granular.Result{
		Path: outputFile,
		Metadata: map[string]string{
			"generation_time": fixedNowFunc().Format(time.RFC3339), // fixedNowFunc to keep the test results deterministic
		},
	}

	if isDebug {
		spew.Dump(result)
	}

	if err := cache.Store(key, result); err != nil {
		log.Printf("Warning: Failed to cache generated artifact: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheRoot)
	}

	res, found, err := cache.Get(key)
	if err != nil || !found {
		log.Fatalf("Failed to fetch artifact: %v", err)
	}

	if isDebug {
		spew.Dump(res)
	}

	expectedResultPath := ".artifact-cache/objects/68/6832ec325639264c/output.go"
	if res.Path != expectedResultPath {
		log.Fatalf("Unexpected artifact output file. Expected %q, but found %q", expectedResultPath, res.Path)
	}
	expectedGenerationTime := fixedNowFunc().Format(time.RFC3339)
	gotGenerationTime := res.Metadata["generation_time"]
	if gotGenerationTime != expectedGenerationTime {
		log.Fatalf("Unexpected generation time metadata. Expected %q, but found %q", expectedGenerationTime, gotGenerationTime)
	}
}

func TestContentBasedFileCache(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".file-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test file
	filePath := "large-dataset.csv"
	fileContent := "id,name,value\n1,item1,100\n2,item2,200\n3,item3,300\n"
	err = afero.WriteFile(memFs, filePath, []byte(fileContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a key based solely on the file content
	key := granular.Key{
		Inputs: []granular.Input{
			granular.FileInput{Path: filePath, Fs: memFs},
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// First get should be a miss
	processedFilePath := filePath + ".processed"
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss, but got a hit")
	}

	// Process the file (just create a mock processed file)
	processedContent := "processed:" + fileContent
	err = afero.WriteFile(memFs, processedFilePath, []byte(processedContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write processed file: %v", err)
	}

	// Store the processed file in cache
	processResult := granular.Result{
		Path: processedFilePath,
	}

	if isDebug {
		spew.Dump(processResult)
	}

	if err := cache.Store(key, processResult); err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheDir)
	}

	// Second get should be a hit
	result, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, but got a miss")
	}

	// Verify the cached file exists
	cachedContent, err := afero.ReadFile(memFs, result.Path)
	if err != nil {
		t.Fatalf("Failed to read cached file: %v", err)
	}
	if string(cachedContent) != processedContent {
		t.Fatalf("Cached content doesn't match. Expected %q, got %q", processedContent, string(cachedContent))
	}

	// Modify the original file
	newFileContent := "id,name,value\n1,item1,100\n2,item2,200\n3,item3,300\n4,item4,400\n"
	err = afero.WriteFile(memFs, filePath, []byte(newFileContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	// Third get should be a miss due to modified content
	_, hit, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after file modification, but got a hit")
	}
}

func TestIncrementalComputation(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".computation-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test directory and files
	inputDir := "data"
	err = memFs.MkdirAll(inputDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create input directory: %v", err)
	}

	// Create some data files
	dataFiles := []struct {
		name    string
		content string
	}{
		{"data1.txt", "This is data file 1"},
		{"data2.txt", "This is data file 2"},
		{"data3.txt", "This is data file 3"},
		{"temp.tmp", "This is a temporary file that should be excluded"},
		{"debug.log", "This is a log file that should be excluded"},
	}

	for _, df := range dataFiles {
		filePath := filepath.Join(inputDir, df.name)
		err = afero.WriteFile(memFs, filePath, []byte(df.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write data file %s: %v", df.name, err)
		}
	}

	// Create config file
	configFile := "config.json"
	configContent := `{"parameter1": "value1", "parameter2": "value2"}`
	err = afero.WriteFile(memFs, configFile, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create a key based on all input files and configuration
	key := granular.Key{
		Inputs: []granular.Input{
			granular.DirectoryInput{
				Path:    inputDir,
				Exclude: []string{"*.tmp", "*.log"},
				Fs:      memFs,
			},
			granular.FileInput{Path: configFile, Fs: memFs},
		},
		Extra: map[string]string{
			"computation_version": "1.2.3",
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// First get should be a miss
	outputFile := "results.json"
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss, but got a hit")
	}

	// Perform the computation (just create a mock result file)
	resultContent := `{"result": "computed data", "count": 3}`
	err = afero.WriteFile(memFs, outputFile, []byte(resultContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write result file: %v", err)
	}

	// Store the result in cache
	computationResult := granular.Result{
		Path: outputFile,
		Metadata: map[string]string{
			"completion_time": fixedNowFunc().Format(time.RFC3339),
			"input_count":     "3", // Excluding .tmp and .log files
		},
	}

	if isDebug {
		spew.Dump(computationResult)
	}

	if err := cache.Store(key, computationResult); err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheDir)
	}

	// Second get should be a hit
	result, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, but got a miss")
	}

	// Verify the cached metadata
	expectedCompletionTime := fixedNowFunc().Format(time.RFC3339)
	gotCompletionTime := result.Metadata["completion_time"]
	if gotCompletionTime != expectedCompletionTime {
		t.Fatalf("Unexpected completion time metadata. Expected %q, but got %q", expectedCompletionTime, gotCompletionTime)
	}

	expectedInputCount := "3"
	gotInputCount := result.Metadata["input_count"]
	if gotInputCount != expectedInputCount {
		t.Fatalf("Unexpected input count metadata. Expected %q, but got %q", expectedInputCount, gotInputCount)
	}

	// Modify one of the input files
	modifiedDataPath := filepath.Join(inputDir, "data1.txt")
	modifiedContent := "This is modified data file 1"
	err = afero.WriteFile(memFs, modifiedDataPath, []byte(modifiedContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update data file: %v", err)
	}

	// Third get should be a miss due to modified input
	_, hit, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after input modification, but got a hit")
	}
}

func TestLocalDevOptimization(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".dev-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test directory structure
	srcDir := "src"
	err = memFs.MkdirAll(srcDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create nested directories
	nestedDirs := []string{
		filepath.Join(srcDir, "api"),
		filepath.Join(srcDir, "models"),
		filepath.Join(srcDir, "utils"),
	}
	for _, dir := range nestedDirs {
		err = memFs.MkdirAll(dir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create some source files
	sourceFiles := []struct {
		path    string
		content string
	}{
		{filepath.Join(srcDir, "api", "handlers.go"), "package api\n\nfunc Handler() {}\n"},
		{filepath.Join(srcDir, "models", "user.go"), "package models\n\ntype User struct {}\n"},
		{filepath.Join(srcDir, "utils", "helpers.go"), "package utils\n\nfunc Helper() {}\n"},
	}

	for _, sf := range sourceFiles {
		err = afero.WriteFile(memFs, sf.path, []byte(sf.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write source file %s: %v", sf.path, err)
		}
	}

	// Create config directory and files
	configDir := "config"
	err = memFs.MkdirAll(configDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configFiles := []struct {
		path    string
		content string
	}{
		{filepath.Join(configDir, "app.yaml"), "environment: development\n"},
		{filepath.Join(configDir, "database.yaml"), "driver: sqlite\n"},
	}

	for _, cf := range configFiles {
		err = afero.WriteFile(memFs, cf.path, []byte(cf.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file %s: %v", cf.path, err)
		}
	}

	// Create dependency files
	err = afero.WriteFile(memFs, "go.mod", []byte("module example.com/myapp\n\ngo 1.16\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod file: %v", err)
	}
	err = afero.WriteFile(memFs, "go.sum", []byte("example.com/dependency v1.0.0 h1:hash\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.sum file: %v", err)
	}

	// Define inputs for the development task
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source files
			granular.GlobInput{Pattern: filepath.Join(srcDir, "**", "*.go"), Fs: memFs},
			// Track configuration files
			granular.GlobInput{Pattern: filepath.Join(configDir, "*.yaml"), Fs: memFs},
			// Track dependencies
			granular.FileInput{Path: "go.mod", Fs: memFs},
			granular.FileInput{Path: "go.sum", Fs: memFs},
		},
		Extra: map[string]string{
			"task": "lint",
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// First get should be a miss
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss, but got a hit")
	}

	// Run the linter (simulate by creating an output file)
	lintOutputFile := "lint-output.txt"
	lintOutput := "src/api/handlers.go:3:1: exported function Handler should have comment\n"
	err = afero.WriteFile(memFs, lintOutputFile, []byte(lintOutput), 0o644)
	if err != nil {
		t.Fatalf("Failed to write lint output file: %v", err)
	}

	// Store the result in cache
	lintResult := granular.Result{
		Path: lintOutputFile,
		Metadata: map[string]string{
			"lint_status": "fail",
			"exit_code":   "1",
		},
	}

	if isDebug {
		spew.Dump(lintResult)
	}

	if err := cache.Store(key, lintResult); err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheDir)
	}

	// Second get should be a hit
	result, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, but got a miss")
	}

	// Verify the cached metadata
	expectedLintStatus := "fail"
	gotLintStatus := result.Metadata["lint_status"]
	if gotLintStatus != expectedLintStatus {
		t.Fatalf("Unexpected lint status metadata. Expected %q, but got %q", expectedLintStatus, gotLintStatus)
	}

	expectedExitCode := "1"
	gotExitCode := result.Metadata["exit_code"]
	if gotExitCode != expectedExitCode {
		t.Fatalf("Unexpected exit code metadata. Expected %q, but got %q", expectedExitCode, gotExitCode)
	}

	// Verify the cached file content
	cachedContent, err := afero.ReadFile(memFs, result.Path)
	if err != nil {
		t.Fatalf("Failed to read cached file: %v", err)
	}
	if string(cachedContent) != lintOutput {
		t.Fatalf("Cached content doesn't match. Expected %q, got %q", lintOutput, string(cachedContent))
	}

	// Fix the lint issue
	fixedHandlerContent := "package api\n\n// Handler is an API handler\nfunc Handler() {}\n"
	err = afero.WriteFile(memFs, filepath.Join(srcDir, "api", "handlers.go"), []byte(fixedHandlerContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update source file: %v", err)
	}

	// Third get should be a miss due to modified source
	_, hit, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after source modification, but got a hit")
	}
}

func TestCIOptimization(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".ci-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test directory structure
	err = memFs.MkdirAll(".github/workflows", 0o755)
	if err != nil {
		t.Fatalf("Failed to create .github/workflows directory: %v", err)
	}

	// Create CI configuration
	ciConfig := `name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.16
      - run: go test ./...
`
	err = afero.WriteFile(memFs, ".github/workflows/ci.yml", []byte(ciConfig), 0o644)
	if err != nil {
		t.Fatalf("Failed to write CI config file: %v", err)
	}

	// Create source and test files
	sourceFiles := []struct {
		path    string
		content string
	}{
		{"main.go", "package main\n\nfunc main() {}\n"},
		{"utils.go", "package main\n\nfunc util() string { return \"util\" }\n"},
		{"main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) { t.Log(\"test passed\") }\n"},
		{"utils_test.go", "package main\n\nimport \"testing\"\n\nfunc TestUtil(t *testing.T) { t.Log(\"test passed\") }\n"},
	}

	for _, sf := range sourceFiles {
		err = afero.WriteFile(memFs, sf.path, []byte(sf.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write file %s: %v", sf.path, err)
		}
	}

	// Create dependency files
	err = afero.WriteFile(memFs, "go.mod", []byte("module example.com/citest\n\ngo 1.16\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod file: %v", err)
	}
	err = afero.WriteFile(memFs, "go.sum", []byte("example.com/dependency v1.0.0 h1:hash\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.sum file: %v", err)
	}

	// Define inputs for the CI task
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source and test files
			granular.GlobInput{Pattern: "**/*.go", Fs: memFs},
			// Track configuration files
			granular.FileInput{Path: ".github/workflows/ci.yml", Fs: memFs},
			granular.FileInput{Path: "go.mod", Fs: memFs},
			granular.FileInput{Path: "go.sum", Fs: memFs},
		},
		Extra: map[string]string{
			"go_version": "1.16",
			"os":         "linux",
			"task":       "test",
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// First get should be a miss
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss, but got a hit")
	}

	// Run the tests (simulate by creating an output file)
	testOutputFile := "test-output.txt"
	testOutput := "=== RUN   TestMain\n--- PASS: TestMain (0.00s)\n=== RUN   TestUtil\n--- PASS: TestUtil (0.00s)\nPASS\n"
	err = afero.WriteFile(memFs, testOutputFile, []byte(testOutput), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test output file: %v", err)
	}

	// Store the result in cache
	testResult := granular.Result{
		Path: testOutputFile,
		Metadata: map[string]string{
			"test_status": "pass",
			"exit_code":   "0",
			"test_count":  "2",
		},
	}

	if isDebug {
		spew.Dump(testResult)
	}

	if err := cache.Store(key, testResult); err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheDir)
	}

	// Second get should be a hit
	result, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, but got a miss")
	}

	// Verify the cached metadata
	expectedTestStatus := "pass"
	gotTestStatus := result.Metadata["test_status"]
	if gotTestStatus != expectedTestStatus {
		t.Fatalf("Unexpected test status metadata. Expected %q, but got %q", expectedTestStatus, gotTestStatus)
	}

	expectedTestCount := "2"
	gotTestCount := result.Metadata["test_count"]
	if gotTestCount != expectedTestCount {
		t.Fatalf("Unexpected test count metadata. Expected %q, but got %q", expectedTestCount, gotTestCount)
	}

	// Verify the cached file content
	cachedContent, err := afero.ReadFile(memFs, result.Path)
	if err != nil {
		t.Fatalf("Failed to read cached file: %v", err)
	}
	if string(cachedContent) != testOutput {
		t.Fatalf("Cached content doesn't match. Expected %q, got %q", testOutput, string(cachedContent))
	}

	// Add a new test
	newTestFile := "new_test.go"
	newTestContent := "package main\n\nimport \"testing\"\n\nfunc TestNew(t *testing.T) { t.Log(\"new test passed\") }\n"
	err = afero.WriteFile(memFs, newTestFile, []byte(newTestContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write new test file: %v", err)
	}

	// Third get should be a miss due to the new test file
	_, hit, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after adding new test, but got a hit")
	}
}

func TestDataPipeline(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".pipeline-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create raw data file
	rawDataFile := "raw-data.csv"
	rawDataContent := "id,name,value\n1,item1,100\n2,item2,200\n3,item3,300\n"
	err = afero.WriteFile(memFs, rawDataFile, []byte(rawDataContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write raw data file: %v", err)
	}

	// Create config directory and files
	configDir := "config"
	err = memFs.MkdirAll(configDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configFiles := []struct {
		name    string
		content string
	}{
		{"extract.yaml", "source: csv\nformat: json\n"},
		{"transform.yaml", "operations: [normalize, validate]\n"},
		{"load.yaml", "destination: database\nformat: json\n"},
	}

	for _, cf := range configFiles {
		filePath := filepath.Join(configDir, cf.name)
		err = afero.WriteFile(memFs, filePath, []byte(cf.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file %s: %v", cf.name, err)
		}
	}

	// Define pipeline stages
	stages := []struct {
		name    string
		input   string
		output  string
		content string
	}{
		{
			name:    "extract",
			input:   rawDataFile,
			output:  "extracted-data.json",
			content: `{"extracted": [{"id": 1, "name": "item1", "value": 100}, {"id": 2, "name": "item2", "value": 200}, {"id": 3, "name": "item3", "value": 300}]}`,
		},
		{
			name:    "transform",
			input:   "extracted-data.json",
			output:  "transformed-data.json",
			content: `{"transformed": [{"id": 1, "name": "item1", "value": 100}, {"id": 2, "name": "item2", "value": 200}, {"id": 3, "name": "item3", "value": 300}]}`,
		},
		{
			name:    "load",
			input:   "transformed-data.json",
			output:  "final-output.json",
			content: `{"loaded": true, "count": 3, "status": "success"}`,
		},
	}

	// Process each stage
	for i, stage := range stages {
		// Create a key for this stage
		var inputs []granular.Input

		// If this is the first stage, use the raw input file
		if i == 0 {
			inputs = []granular.Input{
				granular.FileInput{Path: stage.input, Fs: memFs},
			}
		} else {
			// Otherwise, use the output from the previous stage
			inputs = []granular.Input{
				granular.FileInput{Path: stages[i-1].output, Fs: memFs},
			}
		}

		// Add configuration file
		configFile := filepath.Join(configDir, stage.name+".yaml")
		inputs = append(inputs, granular.FileInput{Path: configFile, Fs: memFs})

		key := granular.Key{
			Inputs: inputs,
			Extra: map[string]string{
				"stage":   stage.name,
				"version": "1.0.0",
			},
		}

		if isDebug {
			spew.Dump(key)
		}

		// First get should be a miss
		_, hit, err := cache.Get(key)
		if err != nil {
			t.Fatalf("Failed to get from cache for stage %s: %v", stage.name, err)
		}
		if hit {
			t.Fatalf("Expected cache miss for stage %s, but got a hit", stage.name)
		}

		// Process this stage (create the output file)
		err = afero.WriteFile(memFs, stage.output, []byte(stage.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write output file for stage %s: %v", stage.name, err)
		}

		// Store the result in cache
		stageResult := granular.Result{
			Path: stage.output,
			Metadata: map[string]string{
				"stage":        stage.name,
				"process_time": fixedNowFunc().Format(time.RFC3339),
			},
		}

		if isDebug {
			spew.Dump(stageResult)
		}

		if err := cache.Store(key, stageResult); err != nil {
			t.Fatalf("Failed to store in cache for stage %s: %v", stage.name, err)
		}

		if isDebug {
			printDirTree(memFs, cacheDir)
		}

		// Second get should be a hit
		result, hit, err := cache.Get(key)
		if err != nil {
			t.Fatalf("Failed to get from cache for stage %s: %v", stage.name, err)
		}
		if !hit {
			t.Fatalf("Expected cache hit for stage %s, but got a miss", stage.name)
		}

		// Verify the cached metadata
		expectedStage := stage.name
		gotStage := result.Metadata["stage"]
		if gotStage != expectedStage {
			t.Fatalf("Unexpected stage metadata. Expected %q, but got %q", expectedStage, gotStage)
		}

		expectedProcessTime := fixedNowFunc().Format(time.RFC3339)
		gotProcessTime := result.Metadata["process_time"]
		if gotProcessTime != expectedProcessTime {
			t.Fatalf("Unexpected process time metadata. Expected %q, but got %q", expectedProcessTime, gotProcessTime)
		}

		// Verify the cached file content
		cachedContent, err := afero.ReadFile(memFs, result.Path)
		if err != nil {
			t.Fatalf("Failed to read cached file for stage %s: %v", stage.name, err)
		}
		if string(cachedContent) != stage.content {
			t.Fatalf("Cached content doesn't match for stage %s. Expected %q, got %q", stage.name, stage.content, string(cachedContent))
		}
	}

	// Modify the raw data file
	newRawDataContent := "id,name,value\n1,item1,100\n2,item2,200\n3,item3,300\n4,item4,400\n"
	err = afero.WriteFile(memFs, rawDataFile, []byte(newRawDataContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update raw data file: %v", err)
	}

	// First stage should be a miss due to modified input
	firstStage := stages[0]
	firstStageKey := granular.Key{
		Inputs: []granular.Input{
			granular.FileInput{Path: firstStage.input, Fs: memFs},
			granular.FileInput{Path: filepath.Join(configDir, firstStage.name+".yaml"), Fs: memFs},
		},
		Extra: map[string]string{
			"stage":   firstStage.name,
			"version": "1.0.0",
		},
	}

	_, firstStageHit, firstStageErr := cache.Get(firstStageKey)
	if firstStageErr != nil {
		t.Fatalf("Failed to get from cache for first stage after modification: %v", firstStageErr)
	}
	if firstStageHit {
		t.Fatalf("Expected cache miss for first stage after input modification, but got a hit")
	}
}

func TestBuildSystemCache(t *testing.T) {
	isDebug := false // Set to true when you want to troubleshoot issues visually.
	memFs := afero.NewMemMapFs()

	cacheDir := ".build-cache"
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test files
	goModContent := "module example.com/myapp\n\ngo 1.16\n"
	goSumContent := "example.com/dependency v1.0.0 h1:hash\n"
	mainGoContent := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, world!\")\n}\n"

	err = afero.WriteFile(memFs, "go.mod", []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod file: %v", err)
	}
	err = afero.WriteFile(memFs, "go.sum", []byte(goSumContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.sum file: %v", err)
	}
	err = afero.WriteFile(memFs, "main.go", []byte(mainGoContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write main.go file: %v", err)
	}

	// Define cache key based on source files
	key := granular.Key{
		Inputs: []granular.Input{
			// Track all source files
			granular.GlobInput{Pattern: "*.go", Fs: memFs},
			// Track build configuration
			granular.FileInput{Path: "go.mod", Fs: memFs},
			granular.FileInput{Path: "go.sum", Fs: memFs},
		},
		Extra: map[string]string{
			"go_version": "1.16",
			"os":         "linux",
			"arch":       "amd64",
		},
	}

	if isDebug {
		spew.Dump(key)
	}

	// First get should be a miss
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss, but got a hit")
	}

	// "Build" the application
	binaryContent := []byte("mock binary content")
	binaryPath := "myapp"
	err = afero.WriteFile(memFs, binaryPath, binaryContent, 0o755)
	if err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	// Store the build artifact in cache
	buildResult := granular.Result{
		Path: binaryPath,
		Metadata: map[string]string{
			"build_time": fixedNowFunc().Format(time.RFC3339),
		},
	}

	if isDebug {
		spew.Dump(buildResult)
	}

	if err := cache.Store(key, buildResult); err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	if isDebug {
		printDirTree(memFs, cacheDir)
	}

	// Second get should be a hit
	result, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, but got a miss")
	}

	// Verify the cached data
	expectedBuildTime := fixedNowFunc().Format(time.RFC3339)
	gotBuildTime := result.Metadata["build_time"]
	if gotBuildTime != expectedBuildTime {
		t.Fatalf("Unexpected build time metadata. Expected %q, but got %q", expectedBuildTime, gotBuildTime)
	}

	// Modify a source file
	newMainGoContent := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, updated world!\")\n}\n"
	err = afero.WriteFile(memFs, "main.go", []byte(newMainGoContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to update main.go file: %v", err)
	}

	// Third get should be a miss due to modified source
	_, hit, err = cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after source modification, but got a hit")
	}
}

func generateArtifact(fs afero.Fs, schemaFilePath string) (string, error) {
	// we're not really creating the output with the protoc tooling, so let's ignore the schema and fake the output file
	_ = schemaFilePath
	outputFile := "output.go"
	err := afero.WriteFile(fs, outputFile, []byte("output from schema proto"), 0o644)
	if err != nil {
		return "", err
	}
	return outputFile, nil
}

func printDirTree(fs afero.Fs, path string) error {
	err := afero.Walk(fs, path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if p == path {
			return nil
		}

		depth := strings.Count(p, string(os.PathSeparator))
		indent := strings.Repeat("│   ", depth-1)

		name := info.Name()
		if info.IsDir() {
			fmt.Printf("%s├── 📁 %s\n", indent, name)
		} else {
			fmt.Printf("%s├── 📄 %s\n", indent, name)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Failed to inspect the folder: %v", err)
	}

	return nil
}

func fixedNowFunc() time.Time {
	return time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
}
