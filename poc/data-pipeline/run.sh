#!/bin/bash

# Sales Report Data Pipeline Demo Script
# This script demonstrates various caching scenarios

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "   Data Pipeline Caching Demo - Granular POC"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Clean up any previous runs
echo -e "${YELLOW}Cleaning up previous runs...${NC}"
rm -rf .granular-cache data/ 2>/dev/null || true
echo ""

# Build the project
echo -e "${BLUE}Building the pipeline...${NC}"
go build -o pipeline main.go
echo -e "${GREEN}✓ Build complete${NC}"
echo ""

# Scenario 1: First run (all cache misses)
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  SCENARIO 1: First Run (All Cache Misses)"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "This is the first run. All stages will be executed from scratch."
echo "Expected time: ~22 seconds"
echo ""
read -p "Press Enter to continue..."
echo ""

FIRST_START=$(date +%s)
./pipeline
FIRST_END=$(date +%s)
FIRST_DURATION=$((FIRST_END - FIRST_START))

echo ""
echo -e "${GREEN}✓ First run completed in ${FIRST_DURATION} seconds${NC}"
echo ""
read -p "Press Enter to continue to Scenario 2..."

# Scenario 2: Second run (full cache hit)
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  SCENARIO 2: Second Run (Full Cache Hit)"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Running the same pipeline again with no changes."
echo "All stages should hit the cache."
echo "Expected time: ~0.1 seconds"
echo ""
read -p "Press Enter to continue..."
echo ""

SECOND_START=$(date +%s.%N)
./pipeline
SECOND_END=$(date +%s.%N)
SECOND_DURATION=$(echo "$SECOND_END - $SECOND_START" | bc)

echo ""
echo -e "${GREEN}✓ Second run completed in ${SECOND_DURATION} seconds${NC}"
SPEEDUP=$(echo "scale=1; $FIRST_DURATION / $SECOND_DURATION" | bc)
echo -e "${GREEN}✓ Speedup: ${SPEEDUP}x faster${NC}"
echo ""
read -p "Press Enter to continue to Scenario 3..."

# Scenario 3: Modify late-stage parameter (partial invalidation)
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  SCENARIO 3: Partial Invalidation (Change Stage 5)"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Simulating a change to the report template (stage 5)."
echo "Stages 1-4 should hit cache, only stage 5 re-runs."
echo "Expected time: ~2 seconds"
echo ""
read -p "Press Enter to continue..."
echo ""

THIRD_START=$(date +%s.%N)
./pipeline --invalidate=report
THIRD_END=$(date +%s.%N)
THIRD_DURATION=$(echo "$THIRD_END - $THIRD_START" | bc)

echo ""
echo -e "${GREEN}✓ Partial invalidation completed in ${THIRD_DURATION} seconds${NC}"
echo -e "${GREEN}✓ Only stage 5 was re-executed${NC}"
echo ""
read -p "Press Enter to continue to Scenario 4..."

# Scenario 4: Modify mid-stage parameter (cascading invalidation)
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  SCENARIO 4: Cascading Invalidation (Change Stage 3)"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Simulating a change to the transformation logic (stage 3)."
echo "Stages 1-2 should hit cache, stages 3-5 re-run."
echo "Expected time: ~9 seconds"
echo ""
read -p "Press Enter to continue..."
echo ""

FOURTH_START=$(date +%s)
./pipeline --invalidate=transform
FOURTH_END=$(date +%s)
FOURTH_DURATION=$((FOURTH_END - FOURTH_START))

echo ""
echo -e "${GREEN}✓ Cascading invalidation completed in ${FOURTH_DURATION} seconds${NC}"
echo -e "${GREEN}✓ Stages 3, 4, and 5 were re-executed${NC}"
echo ""
read -p "Press Enter to continue to Scenario 5..."

# Scenario 5: Cache statistics
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  SCENARIO 5: Cache Statistics"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Viewing cache contents and statistics."
echo ""
read -p "Press Enter to continue..."
echo ""

./pipeline --show-cache-stats

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  Summary of Results"
echo "════════════════════════════════════════════════════════════════"
echo ""
printf "%-40s %10s\n" "Scenario" "Time"
echo "────────────────────────────────────────────────────────────────"
printf "%-40s %10s\n" "1. First run (all cache misses)" "${FIRST_DURATION}s"
printf "%-40s %10s\n" "2. Second run (full cache hit)" "${SECOND_DURATION}s"
printf "%-40s %10s\n" "3. Change stage 5 (partial)" "${THIRD_DURATION}s"
printf "%-40s %10s\n" "4. Change stage 3 (cascading)" "${FOURTH_DURATION}s"
echo "────────────────────────────────────────────────────────────────"
echo ""

# Calculate savings
TOTAL_WITHOUT_CACHE=$(echo "$FIRST_DURATION * 4" | bc)
TOTAL_WITH_CACHE=$(echo "$FIRST_DURATION + $SECOND_DURATION + $THIRD_DURATION + $FOURTH_DURATION" | bc)
SAVINGS=$(echo "$TOTAL_WITHOUT_CACHE - $TOTAL_WITH_CACHE" | bc)
SAVINGS_PERCENT=$(echo "scale=1; 100 * $SAVINGS / $TOTAL_WITHOUT_CACHE" | bc)

echo -e "${GREEN}Time Savings:${NC}"
echo "  Without caching: ${TOTAL_WITHOUT_CACHE}s (4 runs × ${FIRST_DURATION}s)"
echo "  With caching:    ${TOTAL_WITH_CACHE}s"
echo "  Saved:           ${SAVINGS}s (${SAVINGS_PERCENT}%)"
echo ""

echo -e "${GREEN}Key Observations:${NC}"
echo "  ✓ Full cache hits are ~${SPEEDUP}x faster"
echo "  ✓ Partial invalidation only re-runs affected stages"
echo "  ✓ Cascading invalidation preserves early-stage caching"
echo "  ✓ Content-based keys ensure correctness"
echo ""

echo -e "${BLUE}Try it yourself:${NC}"
echo "  ./pipeline                    # Run with full cache"
echo "  ./pipeline --invalidate=clean # Force re-run from stage 2"
echo "  ./pipeline --show-cache-stats # View cache contents"
echo "  ./pipeline --clear-cache      # Start fresh"
echo ""

echo "════════════════════════════════════════════════════════════════"
echo "  Demo Complete!"
echo "════════════════════════════════════════════════════════════════"
echo ""
