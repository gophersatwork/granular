package app

import (
	"testing"
	"time"
)

// Simulate expensive test setup (e.g., database connections, external services)
func setupCalculatorTests() {
	time.Sleep(500 * time.Millisecond)
}

// Simulate expensive test teardown
func teardownCalculatorTests() {
	time.Sleep(300 * time.Millisecond)
}

func TestCalculator_Add(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"positive numbers", 5, 3, 8},
		{"negative numbers", -5, -3, -8},
		{"mixed signs", 5, -3, 2},
		{"with zero", 5, 0, 5},
		{"decimals", 5.5, 3.2, 8.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Add(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Add(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestCalculator_Subtract(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"positive numbers", 10, 3, 7},
		{"negative numbers", -10, -3, -7},
		{"mixed signs", 10, -3, 13},
		{"with zero", 10, 0, 10},
		{"decimals", 10.5, 3.2, 7.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Subtract(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Subtract(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestCalculator_Multiply(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"positive numbers", 5, 3, 15},
		{"negative numbers", -5, -3, 15},
		{"mixed signs", 5, -3, -15},
		{"with zero", 5, 0, 0},
		{"decimals", 2.5, 4, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Multiply(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Multiply(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestCalculator_Divide(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name      string
		a, b      float64
		expected  float64
		expectErr bool
	}{
		{"positive numbers", 10, 2, 5, false},
		{"negative numbers", -10, -2, 5, false},
		{"mixed signs", 10, -2, -5, false},
		{"with zero dividend", 0, 5, 0, false},
		{"division by zero", 10, 0, 0, true},
		{"decimals", 10, 4, 2.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.Divide(tt.a, tt.b)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Divide(%v, %v) expected error but got nil", tt.a, tt.b)
				}
			} else {
				if err != nil {
					t.Errorf("Divide(%v, %v) unexpected error: %v", tt.a, tt.b, err)
				}
				if result != tt.expected {
					t.Errorf("Divide(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
				}
			}
		})
	}
}

func TestCalculator_Power(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"square", 5, 2, 25},
		{"cube", 3, 3, 27},
		{"power of zero", 5, 0, 1},
		{"power of one", 5, 1, 5},
		{"negative exponent", 2, -2, 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Power(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Power(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestCalculator_SquareRoot(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	tests := []struct {
		name      string
		a         float64
		expected  float64
		expectErr bool
	}{
		{"perfect square", 25, 5, false},
		{"non-perfect square", 10, 3.1622776601683795, false},
		{"zero", 0, 0, false},
		{"negative number", -10, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.SquareRoot(tt.a)
			if tt.expectErr {
				if err == nil {
					t.Errorf("SquareRoot(%v) expected error but got nil", tt.a)
				}
			} else {
				if err != nil {
					t.Errorf("SquareRoot(%v) unexpected error: %v", tt.a, err)
				}
				if result != tt.expected {
					t.Errorf("SquareRoot(%v) = %v; want %v", tt.a, result, tt.expected)
				}
			}
		})
	}
}

func TestCalculator_Memory(t *testing.T) {
	setupCalculatorTests()
	defer teardownCalculatorTests()

	calc := NewCalculator()

	// Initial memory should be 0
	if calc.GetMemory() != 0 {
		t.Errorf("Initial memory = %v; want 0", calc.GetMemory())
	}

	// After addition, memory should be updated
	calc.Add(5, 3)
	if calc.GetMemory() != 8 {
		t.Errorf("Memory after Add = %v; want 8", calc.GetMemory())
	}

	// Clear memory
	calc.ClearMemory()
	if calc.GetMemory() != 0 {
		t.Errorf("Memory after Clear = %v; want 0", calc.GetMemory())
	}
}
