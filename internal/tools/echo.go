package tools

import (
	"context"
	"encoding/json"
)

type EchoTool struct{}

func (e *EchoTool) Name() string {
	return "echo"
}

func (e *EchoTool) Description() string {
	return "Echoes back the provided arguments as a JSON string"
}

func (e *EchoTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {
				"type": "string",
				"description": "The message to echo back"
			}
		},
		"required": ["message"]
	}`)
}

func (e *EchoTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var parsed any
	if err := json.Unmarshal(args, &parsed); err != nil {
		return "", err
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}
