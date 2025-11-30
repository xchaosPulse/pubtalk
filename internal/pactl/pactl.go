package pactl

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Executor runs commands and returns their output.
type Executor interface {
	Run(name string, args ...string) ([]byte, error)
}

// RealExecutor uses exec.Command to run real commands.
type RealExecutor struct{}

func (RealExecutor) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Client wraps pactl command execution.
type Client struct {
	Exec   Executor
	DryRun bool
}

// New creates a new Client. If dry is true, commands are not executed.
func New(dry bool) *Client {
	return &Client{Exec: RealExecutor{}, DryRun: dry}
}

// Pactl runs `pactl <args...>` or returns a dry-run string when DryRun is set.
func (c *Client) Pactl(args ...string) ([]byte, error) {
	if c.DryRun {
		s := "dry-run: pactl " + strings.Join(args, " ")
		fmt.Println(s)
		return []byte(s), nil
	}
	out, err := c.Exec.Run("pactl", args...)
	if err != nil {
		return out, fmt.Errorf("pactl failed: %w, output: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// SinkInput represents a single audio stream playing into a sink.
type SinkInput struct {
	ID              int
	ApplicationName string
}

// ListSinkInputs fetches all current sink inputs and their properties.
func (c *Client) ListSinkInputs() ([]SinkInput, error) {
	// Execute the pactl command to list sink inputs
	output, err := exec.Command("pactl", "list", "sink-inputs").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute pactl command: %v", err)
	}

	// Parse the output to extract sink input information
	var inputs []SinkInput
	var currentInput *SinkInput

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Sink Input #") {
			if currentInput != nil {
				inputs = append(inputs, *currentInput)
			}
			idStr := strings.TrimPrefix(line, "Sink Input #")
			id, _ := strconv.Atoi(idStr)
			currentInput = &SinkInput{ID: id}
		} else if currentInput != nil && strings.HasPrefix(line, "application.name =") {
			// e.g. application.name = "Firefox"
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				currentInput.ApplicationName = strings.Trim(strings.TrimSpace(parts[1]), `"`)
			}
		}
	}
	if currentInput != nil {
		inputs = append(inputs, *currentInput)
	}

	return inputs, nil
}

// MoveSinkInput moves a given sink input to a new sink.
func (c *Client) MoveSinkInput(sinkInputID int, sinkName string) error {
	_, err := c.Pactl("move-sink-input", strconv.Itoa(sinkInputID), sinkName)
	if err != nil {
		return fmt.Errorf("failed to move sink-input %d: %w", sinkInputID, err)
	}
	return nil
}

// GetSinkMonitor finds the monitor source for a given sink by its name.
func (c *Client) GetSinkMonitor(sinkName string) (string, error) {
	out, err := c.Pactl("list", "sinks")
	if err != nil {
		return "", fmt.Errorf("failed to list sinks: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var inTargetSink bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Sink #") {
			inTargetSink = false // Reset when we enter a new sink block
		} else if strings.HasPrefix(line, "Name: "+sinkName) {
			inTargetSink = true
		} else if inTargetSink && strings.HasPrefix(line, "Monitor Source: ") {
			return strings.TrimPrefix(line, "Monitor Source: "), nil
		}
	}

	return "", fmt.Errorf("monitor source for sink '%s' not found", sinkName)
}

// FindModules finds modules by a substring in their arguments and returns all matching IDs.
func (c *Client) FindModules(argumentSubstr string) ([]int, error) {
	out, err := c.Pactl("list", "short", "modules")
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	var ids []int
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 && strings.Contains(parts[2], argumentSubstr) {
			id, err := strconv.Atoi(parts[0])
			if err == nil {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no modules with argument substring '%s' not found", argumentSubstr)
	}
	return ids, nil
}

// FindModulesMatching finds modules whose argument string contains ALL provided substrings.
// This is safer when we want to identify modules created with multiple arguments
// (for example both a specific source and sink). Returns matching module IDs or an error.
func (c *Client) FindModulesMatching(substrs []string) ([]int, error) {
	out, err := c.Pactl("list", "short", "modules")
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	var ids []int
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		args := parts[2]
		matchesAll := true
		for _, s := range substrs {
			if !strings.Contains(args, s) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			id, err := strconv.Atoi(parts[0])
			if err == nil {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no modules matching substrings %v found", substrs)
	}
	return ids, nil
}

// UnloadModule unloads a module by its ID.
func (c *Client) UnloadModule(moduleID int) error {
	_, err := c.Pactl("unload-module", strconv.Itoa(moduleID))
	if err != nil {
		return fmt.Errorf("failed to unload module %d: %w", moduleID, err)
	}
	return nil
}

// SinkExists checks if a sink with the given name already exists.
func (c *Client) SinkExists(sinkName string) (bool, error) {
	out, err := c.Pactl("list", "short", "sinks")
	if err != nil {
		return false, fmt.Errorf("failed to list sinks: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 && parts[1] == sinkName {
			return true, nil
		}
	}
	return false, nil
}
