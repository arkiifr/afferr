package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Client struct { command string; args []string; cmd *exec.Cmd; stdin io.WriteCloser; stdout *bufio.Reader; mu sync.Mutex; nextID int }
type Tool struct { Name string `json:"name"`; Description string `json:"description"`; InputSchema json.RawMessage `json:"inputSchema"` }
func NewStdio(command string, args ...string) (*Client, error) { if command == "" { return nil, fmt.Errorf("MCP command is required") }; cmd := exec.Command(command, args...); stdin, err := cmd.StdinPipe(); if err != nil { return nil, err }; stdout, err := cmd.StdoutPipe(); if err != nil { return nil, err }; if err := cmd.Start(); err != nil { return nil, err }; return &Client{command: command, args: args, cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil }
func (c *Client) Close() error { if c == nil || c.cmd == nil { return nil }; _ = c.stdin.Close(); return c.cmd.Process.Kill() }
func (c *Client) Call(ctx context.Context, method string, params any, result any) error { c.mu.Lock(); defer c.mu.Unlock(); c.nextID++; request := map[string]any{"jsonrpc": "2.0", "id": c.nextID, "method": method, "params": params}; data, _ := json.Marshal(request); data = append(data, '\n'); if _, err := c.stdin.Write(data); err != nil { return err }; response := make(chan []byte, 1); errors := make(chan error, 1); go func() { line, err := c.stdout.ReadBytes('\n'); if err != nil { errors <- err } else { response <- line } }(); select { case <-ctx.Done(): return ctx.Err(); case err := <-errors: return err; case line := <-response: var envelope struct { Result json.RawMessage `json:"result"`; Error *struct { Code int `json:"code"`; Message string `json:"message"` } `json:"error"` }; if err := json.Unmarshal(line, &envelope); err != nil { return err }; if envelope.Error != nil { return fmt.Errorf("MCP %d: %s", envelope.Error.Code, envelope.Error.Message) }; if result != nil { return json.Unmarshal(envelope.Result, result) }; return nil } }
