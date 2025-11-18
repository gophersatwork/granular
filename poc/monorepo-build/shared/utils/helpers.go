package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// GenerateID creates a unique identifier
func GenerateID(prefix string) string {
	timestamp := time.Now().UnixNano()
	random := rand.Int63()
	data := fmt.Sprintf("%s-%d-%d", prefix, timestamp, random)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(hash[:])[:16])
}

// FormatJSON converts any value to pretty JSON
func FormatJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseJSON parses JSON string into a map
func ParseJSON(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

// Contains checks if a slice contains a value
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Filter filters a slice based on a predicate
func Filter(slice []string, predicate func(string) bool) []string {
	result := make([]string, 0)
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms each element in a slice
func Map(slice []string, transform func(string) string) []string {
	result := make([]string, len(slice))
	for i, item := range slice {
		result[i] = transform(item)
	}
	return result
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SanitizeEmail normalizes an email address
func SanitizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(fn func() error, maxRetries int, initialDelay time.Duration) error {
	var err error
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

// MergeStringMaps merges multiple string maps
func MergeStringMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// ComputeHash computes SHA256 hash of a string
func ComputeHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
