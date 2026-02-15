package main

import (
	"fmt"
	"strings"
)

type Progress struct {
	Total   int
	Width   int
	Current int
}

func NewProgress(total, width int) *Progress {
	return &Progress{
		Total:   total,
		Width:   width,
		Current: 0,
	}
}

func (p *Progress) Increment() {
	p.Current++
}

func (p *Progress) IsComplete() bool {
	return p.Current >= p.Total
}

func (p *Progress) Render() string {
	if p.Total == 0 {
		return ""
	}

	percentage := (p.Current * 100) / p.Total
	fillLength := (p.Current * p.Width) / p.Total
	if fillLength > p.Width {
		fillLength = p.Width
	}

	bar := strings.Repeat("=", fillLength)
	empty := strings.Repeat(" ", p.Width-fillLength)

	return fmt.Sprintf("\r[%s%s] %d%% (%d/%d files)", bar, empty, percentage, p.Current, p.Total)
}

func (p *Progress) Update() {
	if !isTerminal() {
		return
	}
	fmt.Print(p.Render())
}

func (p *Progress) Done() {
	if !isTerminal() {
		fmt.Println()
		return
	}
	fmt.Print(p.Render())
	fmt.Println()
}

func shouldUseProgress(total int, batch bool) bool {
	return batch && total > 1
}

func confirmBatchOperation(total int, operation string, force bool) error {
	if force {
		return nil
	}

	var prompt string
	switch operation {
	case "add":
		prompt = fmt.Sprintf("Adding %d files. Proceed? [Y/n]: ", total)
	case "remove":
		prompt = fmt.Sprintf("Removing %d files. Proceed? [Y/n]: ", total)
	default:
		prompt = fmt.Sprintf("Processing %d files. Proceed? [Y/n]: ", total)
	}

	fmt.Printf("%s", prompt)

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "n" || input == "no" {
		fmt.Println("Cancelled.")
		return fmt.Errorf("operation cancelled")
	}

	return nil
}
