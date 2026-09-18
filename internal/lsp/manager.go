package lsp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/arkiifr/afferr/internal/config"
)

type Manager struct { cfg map[string]config.LSPConfig; mu sync.Mutex; servers map[string]*exec.Cmd }
type Request struct { Action string; File string; Line int; Symbol string }
type Response struct { Action string; File string; Diagnostics []string; Content string }
func NewManager(cfg map[string]config.LSPConfig) *Manager { return &Manager{cfg: cfg, servers: map[string]*exec.Cmd{}} }
func (m *Manager) Query(_ context.Context, request Request) (Response, error) { if request.Action == "" { return Response{}, fmt.Errorf("LSP action is required") }; return Response{Action: request.Action, File: request.File, Content: "LSP transport is ready; language server protocol wiring is not enabled for this language"}, nil }
func (m *Manager) Close() { m.mu.Lock(); defer m.mu.Unlock(); for name, cmd := range m.servers { _ = cmd.Process.Kill(); delete(m.servers, name) } }
