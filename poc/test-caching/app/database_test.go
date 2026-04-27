package app

import (
	"testing"
	"time"
)

// Simulate expensive test setup (e.g., spinning up test database, migrations)
func setupDatabaseTests() {
	time.Sleep(600 * time.Millisecond)
}

// Simulate expensive test teardown (e.g., cleaning up test database)
func teardownDatabaseTests() {
	time.Sleep(400 * time.Millisecond)
}

func TestDatabase_Connect(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()

	// Initially should not be connected
	if db.IsConnected() {
		t.Error("Database should not be connected initially")
	}

	// Connect should succeed
	err := db.Connect()
	if err != nil {
		t.Errorf("Connect() unexpected error: %v", err)
	}

	if !db.IsConnected() {
		t.Error("Database should be connected after Connect()")
	}

	// Second connect should fail
	err = db.Connect()
	if err == nil {
		t.Error("Connect() should fail when already connected")
	}
}

func TestDatabase_Disconnect(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()

	// Disconnect without connect should fail
	err := db.Disconnect()
	if err == nil {
		t.Error("Disconnect() should fail when not connected")
	}

	// Connect then disconnect should succeed
	db.Connect()
	err = db.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() unexpected error: %v", err)
	}

	if db.IsConnected() {
		t.Error("Database should not be connected after Disconnect()")
	}
}

func TestDatabase_CreateUser(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	tests := []struct {
		name      string
		userName  string
		email     string
		expectErr bool
	}{
		{"valid user", "John Doe", "john@example.com", false},
		{"empty name", "", "test@example.com", true},
		{"empty email", "Jane Doe", "", true},
		{"another valid user", "Alice Smith", "alice@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.CreateUser(tt.userName, tt.email)
			if tt.expectErr {
				if err == nil {
					t.Errorf("CreateUser() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CreateUser() unexpected error: %v", err)
				}
				if user.Name != tt.userName {
					t.Errorf("User name = %v; want %v", user.Name, tt.userName)
				}
				if user.Email != tt.email {
					t.Errorf("User email = %v; want %v", user.Email, tt.email)
				}
				if user.ID <= 0 {
					t.Errorf("User ID should be positive; got %v", user.ID)
				}
			}
		})
	}
}

func TestDatabase_GetUser(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	// Create a test user
	created, _ := db.CreateUser("Test User", "test@example.com")

	// Get existing user
	user, err := db.GetUser(created.ID)
	if err != nil {
		t.Errorf("GetUser() unexpected error: %v", err)
	}
	if user.Name != "Test User" {
		t.Errorf("User name = %v; want Test User", user.Name)
	}

	// Get non-existent user
	_, err = db.GetUser(9999)
	if err == nil {
		t.Error("GetUser() should return error for non-existent user")
	}
}

func TestDatabase_UpdateUser(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	// Create a test user
	created, _ := db.CreateUser("Original Name", "original@example.com")

	// Update user
	updated, err := db.UpdateUser(created.ID, "New Name", "new@example.com")
	if err != nil {
		t.Errorf("UpdateUser() unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Updated name = %v; want New Name", updated.Name)
	}
	if updated.Email != "new@example.com" {
		t.Errorf("Updated email = %v; want new@example.com", updated.Email)
	}

	// Update non-existent user
	_, err = db.UpdateUser(9999, "Test", "test@example.com")
	if err == nil {
		t.Error("UpdateUser() should return error for non-existent user")
	}
}

func TestDatabase_DeleteUser(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	// Create a test user
	created, _ := db.CreateUser("To Delete", "delete@example.com")

	// Delete user
	err := db.DeleteUser(created.ID)
	if err != nil {
		t.Errorf("DeleteUser() unexpected error: %v", err)
	}

	// Verify user is deleted
	_, err = db.GetUser(created.ID)
	if err == nil {
		t.Error("GetUser() should return error for deleted user")
	}

	// Delete non-existent user
	err = db.DeleteUser(9999)
	if err == nil {
		t.Error("DeleteUser() should return error for non-existent user")
	}
}

func TestDatabase_ListUsers(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	// Initially no users
	users, err := db.ListUsers()
	if err != nil {
		t.Errorf("ListUsers() unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("ListUsers() count = %v; want 0", len(users))
	}

	// Create some users
	db.CreateUser("User 1", "user1@example.com")
	db.CreateUser("User 2", "user2@example.com")
	db.CreateUser("User 3", "user3@example.com")

	users, err = db.ListUsers()
	if err != nil {
		t.Errorf("ListUsers() unexpected error: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("ListUsers() count = %v; want 3", len(users))
	}
}

func TestDatabase_Count(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()
	db.Connect()
	defer db.Disconnect()

	// Initially count should be 0
	count, err := db.Count()
	if err != nil {
		t.Errorf("Count() unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %v; want 0", count)
	}

	// Create users and verify count
	db.CreateUser("User 1", "user1@example.com")
	count, _ = db.Count()
	if count != 1 {
		t.Errorf("Count() = %v; want 1", count)
	}

	db.CreateUser("User 2", "user2@example.com")
	count, _ = db.Count()
	if count != 2 {
		t.Errorf("Count() = %v; want 2", count)
	}
}

func TestDatabase_NotConnected(t *testing.T) {
	setupDatabaseTests()
	defer teardownDatabaseTests()

	db := NewDatabase()

	// All operations should fail when not connected
	_, err := db.CreateUser("Test", "test@example.com")
	if err == nil {
		t.Error("CreateUser() should fail when not connected")
	}

	_, err = db.GetUser(1)
	if err == nil {
		t.Error("GetUser() should fail when not connected")
	}

	_, err = db.ListUsers()
	if err == nil {
		t.Error("ListUsers() should fail when not connected")
	}

	_, err = db.Count()
	if err == nil {
		t.Error("Count() should fail when not connected")
	}
}
