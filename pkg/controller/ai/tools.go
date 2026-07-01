/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"k8s.io/klog/v2"
)

type toolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     func(ctx context.Context, args map[string]interface{}) (string, error)
}

func (c *controller) buildTools() []toolDefinition {
	tools := []toolDefinition{
		{
			Name:        "exec_shell",
			Description: "Run a restricted read-only shell command for diagnostics such as checking IP, hostname, directory listing, or reading a workspace file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Allowed commands: ipconfig, hostname, whoami, nslookup, ping, tracert, netstat, route, arp, systeminfo, Get-ChildItem, Get-Content.",
					},
					"args": map[string]interface{}{
						"type":        "array",
						"description": "Command arguments. For Get-Content and Get-ChildItem, pass a workspace-relative path as the first argument.",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
			Handler: c.handleExecShell,
		},
	}

	if c.cc.Default.EnableDangerousAITools {
		tools = append(tools, toolDefinition{
			Name:        "exec_shell_dangerous",
			Description: "Run any shell command on the server. This is highly privileged and only available to administrators when dangerous AI tools are enabled.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"shell": map[string]interface{}{
						"type":        "string",
						"description": "Shell to use. Allowed values: powershell, cmd.",
						"enum":        []string{"powershell", "cmd"},
					},
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The raw shell command to execute.",
					},
					"workdir": map[string]interface{}{
						"type":        "string",
						"description": "Optional working directory. Defaults to the current server process directory.",
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
			Handler: c.handleExecShellDangerous,
		})
	}

	return tools
}

func toResponsesTools(tools []toolDefinition) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		items = append(items, map[string]interface{}{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}
	return items
}

func (c *controller) executeTool(ctx context.Context, callID, toolName, rawArgs string) (string, error) {
	var args map[string]interface{}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %v", err)
		}
	}

	start := time.Now()
	var (
		output string
		err    error
	)
	for _, tool := range c.buildTools() {
		if tool.Name == toolName {
			output, err = tool.Handler(ctx, args)
			c.recordToolExecution(ctx, callID, toolName, rawArgs, output, err, time.Since(start))
			return output, err
		}
	}
	err = fmt.Errorf("unknown tool: %s", toolName)
	c.recordToolExecution(ctx, callID, toolName, rawArgs, "", err, time.Since(start))
	return "", err
}

func (c *controller) handleExecShell(ctx context.Context, args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	commandArgs, err := parseStringArgs(args["args"])
	if err != nil {
		return "", err
	}

	execName, execArgs, workdir, err := buildRestrictedCommand(command, commandArgs)
	if err != nil {
		return "", err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, execName, execArgs...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := map[string]interface{}{
		"command": execName,
		"args":    execArgs,
		"stdout":  truncateToolOutput(stdout.String()),
		"stderr":  truncateToolOutput(stderr.String()),
		"success": runErr == nil,
	}
	if workdir != "" {
		result["workdir"] = workdir
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *controller) handleExecShellDangerous(ctx context.Context, args map[string]interface{}) (string, error) {
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return "", err
	}
	if user.Role != model.RoleRoot && user.Role != model.RoleAdmin {
		return "", fmt.Errorf("exec_shell_dangerous requires admin role")
	}
	if !c.cc.Default.EnableDangerousAITools {
		return "", fmt.Errorf("dangerous ai tools are disabled")
	}

	command, _ := args["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	shellName, _ := args["shell"].(string)
	shellName = strings.TrimSpace(strings.ToLower(shellName))
	if shellName == "" {
		shellName = "powershell"
	}

	workdir, err := resolveDangerousWorkdir(args["workdir"])
	if err != nil {
		return "", err
	}

	execName, execArgs, err := buildDangerousCommand(shellName, command)
	if err != nil {
		return "", err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, execName, execArgs...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := map[string]interface{}{
		"shell":   shellName,
		"command": command,
		"stdout":  truncateToolOutput(stdout.String()),
		"stderr":  truncateToolOutput(stderr.String()),
		"success": runErr == nil,
	}
	if workdir != "" {
		result["workdir"] = workdir
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseStringArgs(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}

	rawItems, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("args must be a string array")
	}

	items := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("args must be a string array")
		}
		items = append(items, text)
	}
	return items, nil
}

func buildRestrictedCommand(command string, args []string) (string, []string, string, error) {
	switch strings.ToLower(command) {
	case "ipconfig", "hostname", "whoami", "netstat", "route", "arp", "systeminfo", "ping", "tracert", "nslookup":
		return command, args, "", nil
	case "get-content":
		path, workdir, err := resolveWorkspacePath(args)
		if err != nil {
			return "", nil, "", err
		}
		return "powershell", []string{"-NoProfile", "-Command", "Get-Content -LiteralPath $args[0]", path}, workdir, nil
	case "get-childitem":
		path, workdir, err := resolveWorkspacePath(args)
		if err != nil {
			return "", nil, "", err
		}
		return "powershell", []string{"-NoProfile", "-Command", "Get-ChildItem -LiteralPath $args[0]", path}, workdir, nil
	default:
		return "", nil, "", fmt.Errorf("command %q is not allowed", command)
	}
}

func buildDangerousCommand(shellName, command string) (string, []string, error) {
	switch shellName {
	case "powershell":
		return "powershell", []string{"-NoProfile", "-Command", command}, nil
	case "cmd":
		return "cmd", []string{"/c", command}, nil
	default:
		return "", nil, fmt.Errorf("unsupported shell %q", shellName)
	}
}

func resolveWorkspacePath(args []string) (string, string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", "", fmt.Errorf("path argument is required")
	}

	baseDir, err := filepath.Abs(".")
	if err != nil {
		return "", "", err
	}

	targetPath := args[0]
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(baseDir, targetPath)
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return "", "", err
	}

	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, "..\\") || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("path %q is outside the workspace", args[0])
	}

	return targetPath, baseDir, nil
}

func resolveDangerousWorkdir(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	workdir, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("workdir must be a string")
	}
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return "", nil
	}
	return filepath.Abs(workdir)
}

func truncateToolOutput(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 4000 {
		return text
	}
	return text[:4000]
}

func (c *controller) recordToolExecution(ctx context.Context, callID, toolName, rawArgs, output string, runErr error, duration time.Duration) {
	meta := getToolExecutionMeta(ctx)
	if meta == nil {
		return
	}

	recordCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	record := &model.AIToolExecution{
		RequestId:      meta.RequestId,
		UserId:         meta.UserId,
		UserName:       meta.UserName,
		AIAccountId:    meta.AIAccountId,
		ConversationId: meta.ConversationId,
		Provider:       meta.Provider,
		ModelName:      meta.ModelName,
		ToolName:       toolName,
		CallId:         callID,
		Arguments:      truncateToolOutput(rawArgs),
		Output:         truncateToolOutput(output),
		Success:        runErr == nil,
		Duration:       duration.Milliseconds(),
	}
	if runErr != nil {
		record.ErrorMessage = truncateToolOutput(runErr.Error())
	}

	if _, err := c.factory.AIToolExecution().Create(recordCtx, record); err != nil {
		klog.Errorf("failed to create ai tool execution record for tool(%s): %v", toolName, err)
	}
}
