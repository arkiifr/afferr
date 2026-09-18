package hooks

import (
	"context"
	"sync"
	"time"
)

type EventName string
const ( SessionStart EventName = "session.start"; ToolPre EventName = "tool.pre"; ToolPost EventName = "tool.post"; PermissionAsk EventName = "permission.ask"; SessionCompact EventName = "session.compact"; SessionIdle EventName = "session.idle" )

type Event struct { Name EventName; Time time.Time; Session string; Tool string; Data map[string]any }
type Handler func(context.Context, Event) error

type Bus struct { mu sync.RWMutex; handlers map[EventName][]Handler }
func NewBus() *Bus { return &Bus{handlers: map[EventName][]Handler{}} }
func (b *Bus) On(name EventName, handler Handler) { if b == nil || handler == nil { return }; b.mu.Lock(); defer b.mu.Unlock(); b.handlers[name] = append(b.handlers[name], handler) }
func (b *Bus) Publish(ctx context.Context, event Event) error { if b == nil { return nil }; if event.Time.IsZero() { event.Time = time.Now().UTC() }; b.mu.RLock(); handlers := append([]Handler(nil), b.handlers[event.Name]...); b.mu.RUnlock(); for _, handler := range handlers { if err := handler(ctx, event); err != nil { return err } }; return nil }
