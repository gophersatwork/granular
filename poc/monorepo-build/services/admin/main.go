package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"monorepo-build/shared/models"
	"monorepo-build/shared/utils"
)

// CLI represents the admin command-line interface
type CLI struct {
	users   map[string]*models.User
	scanner *bufio.Scanner
	running bool
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{
		users:   make(map[string]*models.User),
		scanner: bufio.NewScanner(os.Stdin),
		running: true,
	}
}

// Run starts the CLI event loop
func (c *CLI) Run() error {
	c.printWelcome()

	for c.running {
		fmt.Print("\nadmin> ")
		if !c.scanner.Scan() {
			break
		}

		input := strings.TrimSpace(c.scanner.Text())
		if input == "" {
			continue
		}

		if err := c.handleCommand(input); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	if err := c.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// printWelcome displays the welcome message
func (c *CLI) printWelcome() {
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println("  Admin CLI - User Management System")
	fmt.Println("  Version: 1.0.0")
	fmt.Println("  Type 'help' for available commands")
	fmt.Println("=" + strings.Repeat("=", 60))
}

// handleCommand processes a user command
func (c *CLI) handleCommand(input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "help":
		return c.cmdHelp(args)
	case "create":
		return c.cmdCreate(args)
	case "list":
		return c.cmdList(args)
	case "get":
		return c.cmdGet(args)
	case "delete":
		return c.cmdDelete(args)
	case "update":
		return c.cmdUpdate(args)
	case "stats":
		return c.cmdStats(args)
	case "export":
		return c.cmdExport(args)
	case "import":
		return c.cmdImport(args)
	case "clear":
		return c.cmdClear(args)
	case "exit", "quit":
		return c.cmdExit(args)
	default:
		return fmt.Errorf("unknown command: %s (type 'help' for available commands)", command)
	}
}

func main() {
	cli := NewCLI()

	// Create some sample users
	sampleUsers := []*models.User{
		{
			ID:        utils.GenerateID("user"),
			Email:     "admin@example.com",
			Name:      "Admin User",
			Role:      "admin",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  map[string]interface{}{"department": "IT"},
		},
		{
			ID:        utils.GenerateID("user"),
			Email:     "john@example.com",
			Name:      "John Doe",
			Role:      "member",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  map[string]interface{}{"department": "Sales"},
		},
	}

	for _, user := range sampleUsers {
		cli.users[user.ID] = user
	}

	if err := cli.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "CLI error: %v\n", err)
		os.Exit(1)
	}
}
