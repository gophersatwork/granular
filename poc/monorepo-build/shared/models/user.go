package models

import (
	"fmt"
	"time"
)

// User represents a user in the system
type User struct {
	ID        string
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]interface{}
}

// Validate checks if the user data is valid
func (u *User) Validate() error {
	if u.ID == "" {
		return fmt.Errorf("user ID is required")
	}
	if u.Email == "" {
		return fmt.Errorf("user email is required")
	}
	if u.Name == "" {
		return fmt.Errorf("user name is required")
	}
	return nil
}

// IsAdmin checks if the user has admin privileges
func (u *User) IsAdmin() bool {
	return u.Role == "admin" || u.Role == "superadmin"
}

// UpdateMetadata updates a metadata field
func (u *User) UpdateMetadata(key string, value interface{}) {
	if u.Metadata == nil {
		u.Metadata = make(map[string]interface{})
	}
	u.Metadata[key] = value
	u.UpdatedAt = time.Now()
}

// GetMetadata retrieves a metadata field
func (u *User) GetMetadata(key string) (interface{}, bool) {
	if u.Metadata == nil {
		return nil, false
	}
	val, ok := u.Metadata[key]
	return val, ok
}

// Clone creates a deep copy of the user
func (u *User) Clone() *User {
	metadata := make(map[string]interface{})
	for k, v := range u.Metadata {
		metadata[k] = v
	}

	return &User{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		Metadata:  metadata,
	}
}

// String returns a string representation of the user
func (u *User) String() string {
	return fmt.Sprintf("User{ID: %s, Email: %s, Name: %s, Role: %s}", u.ID, u.Email, u.Name, u.Role)
}

// Test change at $(date)
