package agent

import (
	"context"
	"fmt"
)

type SubagentRequest struct { Model string `json:"model"`; Prompt string `json:"prompt"`; MaxSteps int `json:"maxSteps"` }
type SubagentResult struct { Summary string; Steps int }
type SubagentRunner func(context.Context, SubagentRequest) (SubagentResult, error)

// RunSubagent is the bounded child-loop hook used by the task tool. The parent
// receives only the summary, never the child's full transcript.
func RunSubagent(ctx context.Context, runner SubagentRunner, request SubagentRequest) (SubagentResult, error) { if runner == nil { return SubagentResult{}, fmt.Errorf("subagent runner is not configured") }; if request.MaxSteps <= 0 || request.MaxSteps > 32 { request.MaxSteps = 32 }; return runner(ctx, request) }
