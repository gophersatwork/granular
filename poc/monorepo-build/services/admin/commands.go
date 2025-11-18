package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"monorepo-build/shared/models"
	"monorepo-build/shared/utils"
)

// cmdHelp displays available commands
func (c *CLI) cmdHelp(args []string) error {
	help := `
Available Commands:
  help                    Show this help message
  create <email> <name>   Create a new user
  list                    List all users
  get <user_id>          Get user details
  delete <user_id>       Delete a user
  update <user_id>       Update user metadata
  stats                   Show system statistics
  export                  Export users to JSON
  import <file>          Import users from JSON
  clear                   Clear all users
  exit/quit              Exit the CLI

Examples:
  create user@example.com "John Doe"
  get user_abc123
  delete user_abc123
`
	fmt.Println(help)
	return nil
}

// cmdCreate creates a new user
func (c *CLI) cmdCreate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: create <email> <name> [role]")
	}

	email := utils.SanitizeEmail(args[0])
	name := strings.Join(args[1:], " ")
	role := "member"

	if len(args) > 2 {
		role = args[len(args)-1]
	}

	user := &models.User{
		ID:        utils.GenerateID("user"),
		Email:     email,
		Name:      name,
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	if err := user.Validate(); err != nil {
		return err
	}

	c.users[user.ID] = user
	fmt.Printf("User created successfully!\n")
	fmt.Printf("  ID:    %s\n", user.ID)
	fmt.Printf("  Email: %s\n", user.Email)
	fmt.Printf("  Name:  %s\n", user.Name)
	fmt.Printf("  Role:  %s\n", user.Role)

	return nil
}

// cmdList lists all users
func (c *CLI) cmdList(args []string) error {
	if len(c.users) == 0 {
		fmt.Println("No users found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tROLE\tCREATED")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for _, user := range c.users {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			utils.TruncateString(user.ID, 20),
			utils.TruncateString(user.Email, 25),
			utils.TruncateString(user.Name, 20),
			user.Role,
			user.CreatedAt.Format("2006-01-02"))
	}

	w.Flush()
	fmt.Printf("\nTotal users: %d\n", len(c.users))
	return nil
}

// cmdGet retrieves a specific user
func (c *CLI) cmdGet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: get <user_id>")
	}

	userID := args[0]
	user, exists := c.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}

	fmt.Println("\nUser Details:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  ID:         %s\n", user.ID)
	fmt.Printf("  Email:      %s\n", user.Email)
	fmt.Printf("  Name:       %s\n", user.Name)
	fmt.Printf("  Role:       %s\n", user.Role)
	fmt.Printf("  Admin:      %v\n", user.IsAdmin())
	fmt.Printf("  Created:    %s\n", user.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:    %s\n", user.UpdatedAt.Format(time.RFC3339))

	if len(user.Metadata) > 0 {
		fmt.Println("\n  Metadata:")
		for key, value := range user.Metadata {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}

	return nil
}

// cmdDelete removes a user
func (c *CLI) cmdDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: delete <user_id>")
	}

	userID := args[0]
	user, exists := c.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}

	delete(c.users, userID)
	fmt.Printf("User deleted: %s (%s)\n", user.Name, user.Email)
	return nil
}

// cmdUpdate updates user metadata
func (c *CLI) cmdUpdate(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: update <user_id> <key> <value>")
	}

	userID := args[0]
	key := args[1]
	value := strings.Join(args[2:], " ")

	user, exists := c.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}

	user.UpdateMetadata(key, value)
	fmt.Printf("User metadata updated: %s = %s\n", key, value)
	return nil
}

// cmdStats shows system statistics
func (c *CLI) cmdStats(args []string) error {
	totalUsers := len(c.users)
	adminCount := 0
	memberCount := 0

	for _, user := range c.users {
		if user.IsAdmin() {
			adminCount++
		} else {
			memberCount++
		}
	}

	fmt.Println("\nSystem Statistics:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Total Users:   %d\n", totalUsers)
	fmt.Printf("  Admins:        %d\n", adminCount)
	fmt.Printf("  Members:       %d\n", memberCount)
	fmt.Printf("  Timestamp:     %s\n", time.Now().Format(time.RFC3339))

	return nil
}

// cmdExport exports users to JSON
func (c *CLI) cmdExport(args []string) error {
	users := make([]*models.User, 0, len(c.users))
	for _, user := range c.users {
		users = append(users, user)
	}

	jsonData, err := utils.FormatJSON(users)
	if err != nil {
		return fmt.Errorf("failed to export users: %w", err)
	}

	fmt.Printf("\nExported %d users:\n", len(users))
	fmt.Println(jsonData)

	return nil
}

// cmdImport imports users from JSON (placeholder)
func (c *CLI) cmdImport(args []string) error {
	return fmt.Errorf("import functionality not yet implemented")
}

// cmdClear clears all users
func (c *CLI) cmdClear(args []string) error {
	count := len(c.users)
	c.users = make(map[string]*models.User)
	fmt.Printf("Cleared %d users\n", count)
	return nil
}

// cmdExit exits the CLI
func (c *CLI) cmdExit(args []string) error {
	c.running = false
	fmt.Println("\nGoodbye!")
	return nil
}
