package app

import (
	"errors"
	"math"
)

// Calculator provides basic arithmetic operations
type Calculator struct {
	Memory float64
}

// NewCalculator creates a new calculator instance
func NewCalculator() *Calculator {
	return &Calculator{Memory: 0}
}

// Add adds two numbers and returns the result
func (c *Calculator) Add(a, b float64) float64 {
	result := a + b
	c.Memory = result
	return result
}

// Subtract subtracts b from a and returns the result
func (c *Calculator) Subtract(a, b float64) float64 {
	result := a - b
	c.Memory = result
	return result
}

// Multiply multiplies two numbers and returns the result
func (c *Calculator) Multiply(a, b float64) float64 {
	result := a * b
	c.Memory = result
	return result
}

// Divide divides a by b and returns the result
// Returns an error if b is zero
func (c *Calculator) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	result := a / b
	c.Memory = result
	return result, nil
}

// Power raises a to the power of b
func (c *Calculator) Power(a, b float64) float64 {
	result := math.Pow(a, b)
	c.Memory = result
	return result
}

// SquareRoot calculates the square root of a number
// Returns an error if the number is negative
func (c *Calculator) SquareRoot(a float64) (float64, error) {
	if a < 0 {
		return 0, errors.New("cannot calculate square root of negative number")
	}
	result := math.Sqrt(a)
	c.Memory = result
	return result, nil
}

// GetMemory returns the last calculated value
func (c *Calculator) GetMemory() float64 {
	return c.Memory
}

// ClearMemory resets the memory to zero
func (c *Calculator) ClearMemory() {
	c.Memory = 0
}
