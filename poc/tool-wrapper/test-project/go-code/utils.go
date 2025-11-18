package main

import "fmt"

// TODO: Add documentation
type Helper struct {
	Name string
	Data map[string]interface{}
}

func NewHelper(name string) *Helper {
	fmt.Println("Creating new helper:", name)
	return &Helper{
		Name: name,
		Data: make(map[string]interface{}),
	}
}

func (h *Helper) Process() error {
	// TODO: Implement processing logic
	fmt.Println("Processing helper:", h.Name)
	return nil
}

// This function has an intentionally long line that will be flagged by the linter because it exceeds the recommended line length limit
func (h *Helper) VeryLongFunctionNameThatDemonstatesLineLengthIssues(parameter1 string, parameter2 int, parameter3 bool) error {
	return nil
}
