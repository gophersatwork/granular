package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gophersatwork/granular"
)

// Configuration for pipeline stages
const (
	// Stage versions - increment when logic changes
	downloadVersion  = "v1.0.0"
	cleanVersion     = "v1.0.0"
	transformVersion = "v1.0.0"
	analyzeVersion   = "v1.0.0"
	reportVersion    = "v1.0.0"

	// Simulated delays (in seconds)
	downloadDelay  = 5
	cleanDelay     = 8
	transformDelay = 4
	analyzeDelay   = 3
	reportDelay    = 2
)

// SalesRecord represents a single sales transaction
type SalesRecord struct {
	Date     string  `json:"date"`
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Total    float64 `json:"total"`
	Region   string  `json:"region"`
}

// Stats represents summary statistics
type Stats struct {
	TotalSales    float64            `json:"total_sales"`
	TotalUnits    int                `json:"total_units"`
	AveragePrice  float64            `json:"average_price"`
	RecordCount   int                `json:"record_count"`
	TopProducts   []ProductSummary   `json:"top_products"`
	SalesByRegion map[string]float64 `json:"sales_by_region"`
	GeneratedAt   string             `json:"generated_at"`
}

// ProductSummary represents aggregated product data
type ProductSummary struct {
	Product    string  `json:"product"`
	TotalSales float64 `json:"total_sales"`
	Units      int     `json:"units"`
}

func main() {
	// Command line flags
	clearCache := flag.Bool("clear-cache", false, "Clear cache before running")
	showStats := flag.Bool("show-cache-stats", false, "Show cache statistics and exit")
	invalidate := flag.String("invalidate", "", "Force invalidation of specific stage (download, clean, transform, analyze, report)")
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║     Sales Report Data Pipeline - Granular POC         ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Open cache
	cache, err := granular.Open(".granular-cache")
	if err != nil {
		fmt.Printf("Error opening cache: %v\n", err)
		os.Exit(1)
	}

	// Handle clear cache flag
	if *clearCache {
		fmt.Println("🗑️  Clearing cache...")
		cache.Clear()
		fmt.Println("✓ Cache cleared\n")
	}

	// Handle show stats flag
	if *showStats {
		showCacheStats(cache)
		return
	}

	// Track total execution time
	pipelineStart := time.Now()

	// Invalidation tracking
	invalidateStage := *invalidate

	// Stage 1: Download CSV data
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Stage 1: Download Sales Data")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	rawDataPath, err := downloadData(cache, invalidateStage == "download")
	if err != nil {
		fmt.Printf("Error in download stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Stage 2: Clean and validate data
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Stage 2: Clean and Validate Data")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	cleanDataPath, err := cleanData(cache, rawDataPath, invalidateStage == "clean")
	if err != nil {
		fmt.Printf("Error in clean stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Stage 3: Transform to JSON
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Stage 3: Transform to JSON")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	jsonDataPath, err := transformData(cache, cleanDataPath, invalidateStage == "transform")
	if err != nil {
		fmt.Printf("Error in transform stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Stage 4: Generate statistics
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Stage 4: Generate Summary Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	statsPath, err := analyzeData(cache, jsonDataPath, invalidateStage == "analyze")
	if err != nil {
		fmt.Printf("Error in analyze stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Stage 5: Create final report
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Stage 5: Generate Final Report")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	reportPath, err := createReport(cache, statsPath, invalidateStage == "report")
	if err != nil {
		fmt.Printf("Error in report stage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Show final results
	pipelineElapsed := time.Since(pipelineStart)
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Pipeline Complete                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\nTotal execution time: %.2f seconds\n", pipelineElapsed.Seconds())
	fmt.Printf("Final report: %s\n\n", reportPath)

	// Display the report
	reportContent, _ := os.ReadFile(reportPath)
	fmt.Println("┌────────────────────────────────────────────────────────┐")
	fmt.Println("│                     FINAL REPORT                       │")
	fmt.Println("└────────────────────────────────────────────────────────┘")
	fmt.Println(string(reportContent))
	fmt.Println()

	// Show cache statistics
	fmt.Println("💡 Tip: Run the pipeline again to see cache hits!")
	fmt.Println("   Run with --show-cache-stats to see cache contents")
	fmt.Println("   Run with --invalidate=<stage> to force re-run from that stage")
}

// downloadData simulates downloading CSV data from an API
func downloadData(cache *granular.Cache, forceInvalidate bool) (string, error) {
	outputPath := "data/raw_data.csv"

	// Build cache key
	key := cache.Key().
		String("source", "sales-api").
		String("date_range", "2024-01-01:2024-01-31").
		Version(downloadVersion).
		Build()

	// Check cache (unless forcing invalidation)
	if !forceInvalidate {
		result := cache.Get(key)
		if result != nil {
			fmt.Println("✓ Cache HIT - Using cached data")
			fmt.Printf("  Cached: %s\n", result.Meta("timestamp"))
			fmt.Printf("  Records: %s\n", result.Meta("record_count"))

			// Restore file
			os.MkdirAll("data", 0o755)
			result.CopyFile("data", outputPath)

			return outputPath, nil
		}
	}

	// Cache miss - do the work
	fmt.Println("✗ Cache MISS - Downloading data from API...")
	start := time.Now()

	// Simulate network delay
	time.Sleep(downloadDelay * time.Second)

	// Generate sample data
	os.MkdirAll("data", 0o755)
	file, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// Write header
	writer.Write([]string{"Date", "Product", "Quantity", "Price", "Region"})

	// Write sample records
	products := []string{"Laptop", "Mouse", "Keyboard", "Monitor", "Headphones"}
	regions := []string{"North", "South", "East", "West"}
	recordCount := 0

	for day := 1; day <= 31; day++ {
		for i := 0; i < 5; i++ {
			date := fmt.Sprintf("2024-01-%02d", day)
			product := products[i%len(products)]
			quantity := (i+day)%10 + 1
			price := float64((i+1)*100 + day*10)
			region := regions[(i+day)%len(regions)]

			writer.Write([]string{
				date,
				product,
				strconv.Itoa(quantity),
				fmt.Sprintf("%.2f", price),
				region,
			})
			recordCount++
		}
	}

	// Flush before measuring time
	writer.Flush()
	elapsed := time.Since(start)
	fmt.Printf("  Downloaded %d records in %.2f seconds\n", recordCount, elapsed.Seconds())

	// Cache the result
	cache.Put(key).
		File("data", outputPath).
		Meta("record_count", strconv.Itoa(recordCount)).
		Meta("timestamp", time.Now().Format(time.RFC3339)).
		Meta("duration", fmt.Sprintf("%.2fs", elapsed.Seconds())).
		Commit()

	return outputPath, nil
}

// cleanData validates and cleans the raw data
func cleanData(cache *granular.Cache, inputPath string, forceInvalidate bool) (string, error) {
	outputPath := "data/clean_data.csv"

	// Build cache key based on input file content and cleaning configuration
	key := cache.Key().
		File(inputPath).
		String("config", "remove_nulls=true,validate_prices=true").
		Version(cleanVersion).
		Build()

	// Check cache
	if !forceInvalidate {
		result := cache.Get(key)
		if result != nil {
			fmt.Println("✓ Cache HIT - Using cached cleaned data")
			fmt.Printf("  Cached: %s\n", result.Meta("timestamp"))
			fmt.Printf("  Valid records: %s/%s\n", result.Meta("valid_records"), result.Meta("total_records"))

			result.CopyFile("data", outputPath)
			return outputPath, nil
		}
	}

	// Cache miss - do the work
	fmt.Println("✗ Cache MISS - Cleaning and validating data...")
	start := time.Now()

	// Simulate processing delay
	time.Sleep(cleanDelay * time.Second)

	// Read input
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer inputFile.Close()

	reader := csv.NewReader(inputFile)
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	// Write cleaned output
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)

	// Write header
	writer.Write(records[0])

	// Clean records
	validCount := 0
	for _, record := range records[1:] {
		// Validate (in real scenario: remove nulls, check formats, etc.)
		if len(record) == 5 && record[0] != "" {
			writer.Write(record)
			validCount++
		}
	}

	// Flush before measuring time
	writer.Flush()
	elapsed := time.Since(start)
	fmt.Printf("  Validated %d/%d records in %.2f seconds\n", validCount, len(records)-1, elapsed.Seconds())

	// Cache the result
	cache.Put(key).
		File("data", outputPath).
		Meta("valid_records", strconv.Itoa(validCount)).
		Meta("total_records", strconv.Itoa(len(records)-1)).
		Meta("timestamp", time.Now().Format(time.RFC3339)).
		Meta("duration", fmt.Sprintf("%.2fs", elapsed.Seconds())).
		Commit()

	return outputPath, nil
}

// transformData converts CSV to JSON format
func transformData(cache *granular.Cache, inputPath string, forceInvalidate bool) (string, error) {
	outputPath := "data/data.json"

	// Build cache key
	key := cache.Key().
		File(inputPath).
		Version(transformVersion).
		Build()

	// Check cache
	if !forceInvalidate {
		result := cache.Get(key)
		if result != nil {
			fmt.Println("✓ Cache HIT - Using cached JSON data")
			fmt.Printf("  Cached: %s\n", result.Meta("timestamp"))
			fmt.Printf("  Records: %s\n", result.Meta("record_count"))

			result.CopyFile("data", outputPath)
			return outputPath, nil
		}
	}

	// Cache miss - do the work
	fmt.Println("✗ Cache MISS - Transforming CSV to JSON...")
	start := time.Now()

	// Simulate processing delay
	time.Sleep(transformDelay * time.Second)

	// Read CSV
	file, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	// Convert to JSON structs
	var salesRecords []SalesRecord
	for _, record := range records[1:] {
		quantity, _ := strconv.Atoi(record[2])
		price, _ := strconv.ParseFloat(record[3], 64)

		salesRecords = append(salesRecords, SalesRecord{
			Date:     record[0],
			Product:  record[1],
			Quantity: quantity,
			Price:    price,
			Total:    float64(quantity) * price,
			Region:   record[4],
		})
	}

	// Write JSON
	jsonData, err := json.MarshalIndent(salesRecords, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(outputPath, jsonData, 0o644)
	if err != nil {
		return "", err
	}

	elapsed := time.Since(start)
	fmt.Printf("  Transformed %d records to JSON in %.2f seconds\n", len(salesRecords), elapsed.Seconds())

	// Cache the result
	cache.Put(key).
		File("data", outputPath).
		Meta("record_count", strconv.Itoa(len(salesRecords))).
		Meta("timestamp", time.Now().Format(time.RFC3339)).
		Meta("duration", fmt.Sprintf("%.2fs", elapsed.Seconds())).
		Commit()

	return outputPath, nil
}

// analyzeData generates summary statistics
func analyzeData(cache *granular.Cache, inputPath string, forceInvalidate bool) (string, error) {
	outputPath := "data/stats.json"

	// Build cache key
	key := cache.Key().
		File(inputPath).
		String("config", "top_products=5").
		Version(analyzeVersion).
		Build()

	// Check cache
	if !forceInvalidate {
		result := cache.Get(key)
		if result != nil {
			fmt.Println("✓ Cache HIT - Using cached statistics")
			fmt.Printf("  Cached: %s\n", result.Meta("timestamp"))
			fmt.Printf("  Total sales: $%s\n", result.Meta("total_sales"))

			result.CopyFile("data", outputPath)
			return outputPath, nil
		}
	}

	// Cache miss - do the work
	fmt.Println("✗ Cache MISS - Analyzing data and generating statistics...")
	start := time.Now()

	// Simulate processing delay
	time.Sleep(analyzeDelay * time.Second)

	// Read JSON data
	jsonData, err := os.ReadFile(inputPath)
	if err != nil {
		return "", err
	}

	var records []SalesRecord
	err = json.Unmarshal(jsonData, &records)
	if err != nil {
		return "", err
	}

	// Calculate statistics
	var totalSales float64
	var totalUnits int
	productSales := make(map[string]*ProductSummary)
	regionSales := make(map[string]float64)

	for _, record := range records {
		totalSales += record.Total
		totalUnits += record.Quantity

		// Product aggregation
		if ps, exists := productSales[record.Product]; exists {
			ps.TotalSales += record.Total
			ps.Units += record.Quantity
		} else {
			productSales[record.Product] = &ProductSummary{
				Product:    record.Product,
				TotalSales: record.Total,
				Units:      record.Quantity,
			}
		}

		// Region aggregation
		regionSales[record.Region] += record.Total
	}

	// Get top products
	var topProducts []ProductSummary
	for _, ps := range productSales {
		topProducts = append(topProducts, *ps)
	}

	// Simple sort by total sales (descending)
	for i := 0; i < len(topProducts); i++ {
		for j := i + 1; j < len(topProducts); j++ {
			if topProducts[j].TotalSales > topProducts[i].TotalSales {
				topProducts[i], topProducts[j] = topProducts[j], topProducts[i]
			}
		}
	}

	// Take top 5
	if len(topProducts) > 5 {
		topProducts = topProducts[:5]
	}

	stats := Stats{
		TotalSales:    totalSales,
		TotalUnits:    totalUnits,
		AveragePrice:  totalSales / float64(totalUnits),
		RecordCount:   len(records),
		TopProducts:   topProducts,
		SalesByRegion: regionSales,
		GeneratedAt:   time.Now().Format(time.RFC3339),
	}

	// Write stats
	statsData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(outputPath, statsData, 0o644)
	if err != nil {
		return "", err
	}

	elapsed := time.Since(start)
	fmt.Printf("  Analyzed %d records in %.2f seconds\n", len(records), elapsed.Seconds())
	fmt.Printf("  Total sales: $%.2f\n", totalSales)

	// Cache the result
	cache.Put(key).
		File("data", outputPath).
		Meta("total_sales", fmt.Sprintf("%.2f", totalSales)).
		Meta("timestamp", time.Now().Format(time.RFC3339)).
		Meta("duration", fmt.Sprintf("%.2fs", elapsed.Seconds())).
		Commit()

	return outputPath, nil
}

// createReport generates the final formatted report
func createReport(cache *granular.Cache, statsPath string, forceInvalidate bool) (string, error) {
	outputPath := "data/report.txt"

	// Build cache key
	key := cache.Key().
		File(statsPath).
		String("template", "standard").
		Version(reportVersion).
		Build()

	// Check cache
	if !forceInvalidate {
		result := cache.Get(key)
		if result != nil {
			fmt.Println("✓ Cache HIT - Using cached report")
			fmt.Printf("  Cached: %s\n", result.Meta("timestamp"))

			result.CopyFile("report", outputPath)
			return outputPath, nil
		}
	}

	// Cache miss - do the work
	fmt.Println("✗ Cache MISS - Generating final report...")
	start := time.Now()

	// Simulate processing delay
	time.Sleep(reportDelay * time.Second)

	// Read stats
	statsData, err := os.ReadFile(statsPath)
	if err != nil {
		return "", err
	}

	var stats Stats
	err = json.Unmarshal(statsData, &stats)
	if err != nil {
		return "", err
	}

	// Generate report
	var report strings.Builder
	report.WriteString("═══════════════════════════════════════════════════════\n")
	report.WriteString("           MONTHLY SALES REPORT - JANUARY 2024         \n")
	report.WriteString("═══════════════════════════════════════════════════════\n")
	report.WriteString("\n")

	report.WriteString("SUMMARY\n")
	report.WriteString("-------------------------------------------------------\n")
	report.WriteString(fmt.Sprintf("Total Sales:      $%.2f\n", stats.TotalSales))
	report.WriteString(fmt.Sprintf("Total Units:      %d\n", stats.TotalUnits))
	report.WriteString(fmt.Sprintf("Average Price:    $%.2f\n", stats.AveragePrice))
	report.WriteString(fmt.Sprintf("Transactions:     %d\n", stats.RecordCount))
	report.WriteString("\n")

	report.WriteString("TOP PRODUCTS\n")
	report.WriteString("-------------------------------------------------------\n")
	for i, product := range stats.TopProducts {
		report.WriteString(fmt.Sprintf("%d. %-15s $%.2f (%d units)\n",
			i+1, product.Product, product.TotalSales, product.Units))
	}
	report.WriteString("\n")

	report.WriteString("SALES BY REGION\n")
	report.WriteString("-------------------------------------------------------\n")
	for region, sales := range stats.SalesByRegion {
		percentage := (sales / stats.TotalSales) * 100
		report.WriteString(fmt.Sprintf("%-10s $%.2f (%.1f%%)\n", region, sales, percentage))
	}
	report.WriteString("\n")

	report.WriteString("-------------------------------------------------------\n")
	report.WriteString(fmt.Sprintf("Report Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString("═══════════════════════════════════════════════════════\n")

	// Write report
	err = os.WriteFile(outputPath, []byte(report.String()), 0o644)
	if err != nil {
		return "", err
	}

	elapsed := time.Since(start)
	fmt.Printf("  Generated report in %.2f seconds\n", elapsed.Seconds())

	// Cache the result
	cache.Put(key).
		File("report", outputPath).
		Meta("timestamp", time.Now().Format(time.RFC3339)).
		Meta("duration", fmt.Sprintf("%.2fs", elapsed.Seconds())).
		Commit()

	return outputPath, nil
}

// showCacheStats displays cache statistics
func showCacheStats(cache *granular.Cache) {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                  Cache Statistics                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	stats, err := cache.Stats()
	if err != nil {
		fmt.Printf("Error getting stats: %v\n", err)
		return
	}

	fmt.Printf("Total entries: %d\n", stats.Entries)
	fmt.Printf("Total size:    %.2f MB\n\n", float64(stats.TotalSize)/1024/1024)

	entries, err := cache.Entries()
	if err != nil {
		fmt.Printf("Error getting entries: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println("Cache is empty. Run the pipeline to populate it.")
		return
	}

	fmt.Println("Cached stages:")
	fmt.Println("─────────────────────────────────────────────────────────")
	for i, entry := range entries {
		age := time.Since(entry.CreatedAt)
		fmt.Printf("%d. Key: %s...\n", i+1, entry.KeyHash[:16])
		fmt.Printf("   Created: %s (%s ago)\n", entry.CreatedAt.Format("2006-01-02 15:04:05"), age.Round(time.Second))
		fmt.Printf("   Size: %.2f KB\n", float64(entry.Size)/1024)
		fmt.Println()
	}
}
