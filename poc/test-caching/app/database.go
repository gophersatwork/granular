package app

import (
	"errors"
	"fmt"
	"time"
)

// User represents a user in the database
type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
}

// Database simulates a database connection and operations
type Database struct {
	connected bool
	users     map[int]*User
	nextID    int
}

// NewDatabase creates a new database instance
func NewDatabase() *Database {
	return &Database{
		connected: false,
		users:     make(map[int]*User),
		nextID:    1,
	}
}

// Connect simulates establishing a database connection
func (db *Database) Connect() error {
	if db.connected {
		return errors.New("already connected")
	}
	// Simulate connection delay
	time.Sleep(100 * time.Millisecond)
	db.connected = true
	return nil
}

// Disconnect closes the database connection
func (db *Database) Disconnect() error {
	if !db.connected {
		return errors.New("not connected")
	}
	db.connected = false
	return nil
}

// IsConnected returns the connection status
func (db *Database) IsConnected() bool {
	return db.connected
}

// CreateUser creates a new user in the database
func (db *Database) CreateUser(name, email string) (*User, error) {
	if !db.connected {
		return nil, errors.New("database not connected")
	}

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	// Simulate database write operation
	time.Sleep(50 * time.Millisecond)

	user := &User{
		ID:        db.nextID,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	db.users[db.nextID] = user
	db.nextID++

	return user, nil
}

// GetUser retrieves a user by ID
func (db *Database) GetUser(id int) (*User, error) {
	if !db.connected {
		return nil, errors.New("database not connected")
	}

	// Simulate database read operation
	time.Sleep(30 * time.Millisecond)

	user, exists := db.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}

	return user, nil
}

// UpdateUser updates an existing user
func (db *Database) UpdateUser(id int, name, email string) (*User, error) {
	if !db.connected {
		return nil, errors.New("database not connected")
	}

	user, exists := db.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}

	if name != "" {
		user.Name = name
	}

	if email != "" {
		user.Email = email
	}

	// Simulate database update operation
	time.Sleep(50 * time.Millisecond)

	return user, nil
}

// DeleteUser removes a user from the database
func (db *Database) DeleteUser(id int) error {
	if !db.connected {
		return errors.New("database not connected")
	}

	if _, exists := db.users[id]; !exists {
		return fmt.Errorf("user with ID %d not found", id)
	}

	// Simulate database delete operation
	time.Sleep(40 * time.Millisecond)

	delete(db.users, id)
	return nil
}

// ListUsers returns all users in the database
func (db *Database) ListUsers() ([]*User, error) {
	if !db.connected {
		return nil, errors.New("database not connected")
	}

	// Simulate database query operation
	time.Sleep(70 * time.Millisecond)

	users := make([]*User, 0, len(db.users))
	for _, user := range db.users {
		users = append(users, user)
	}

	return users, nil
}

// Count returns the number of users in the database
func (db *Database) Count() (int, error) {
	if !db.connected {
		return 0, errors.New("database not connected")
	}

	// Simulate database count operation
	time.Sleep(20 * time.Millisecond)

	return len(db.users), nil
}
