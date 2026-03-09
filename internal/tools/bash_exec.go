package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type BashExecTool struct {
	timeout time.Duration
}

type bashExecArgs struct {
	Command string `json:"command"`
}

func NewBashExecTool(timeoutSeconds int) *BashExecTool {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	return &BashExecTool{
		timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

func (t *BashExecTool) Name() string { return "bash_exec" }

func (t *BashExecTool) Description() string {
	return "Execute a bash command and return its output. Commands have a configurable timeout."
}

func (t *BashExecTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The bash command to execute"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashExecTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a bashExecArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command) //#nosec G204 -- command execution is the tool's purpose
	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %s", t.timeout)
		}
		return fmt.Sprintf("%s\nexit code: %d", result, exitCode), nil
	}

	return fmt.Sprintf("%s\nexit code: 0", result), nil
}
