package runner

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type EventStream struct {
	enc *json.Encoder
	mu  sync.Mutex
}

func NewEventStream(w io.Writer) *EventStream {
	if w == nil {
		return nil
	}
	return &EventStream{enc: json.NewEncoder(w)}
}

func (s *EventStream) emit(payload map[string]any) {
	if s == nil {
		return
	}
	payload["ts"] = eventTS()
	s.mu.Lock()
	_ = s.enc.Encode(payload)
	s.mu.Unlock()
}

func (s *EventStream) EmitPlan(rootTask string) {
	s.emit(map[string]any{
		"type": "plan",
		"graph": map[string]any{
			"root": rootTask,
		},
	})
}

func (s *EventStream) EmitStart(task string) {
	s.emit(map[string]any{
		"type": "start",
		"task": task,
	})
}

func (s *EventStream) EmitOutput(task, stream, line string) {
	s.emit(map[string]any{
		"type":   "output",
		"task":   task,
		"stream": stream,
		"line":   line,
	})
}

func (s *EventStream) EmitDone(task, status string, durationMS int64) {
	s.emit(map[string]any{
		"type":        "done",
		"task":        task,
		"status":      status,
		"duration_ms": durationMS,
	})
}

func (s *EventStream) EmitSkipped(task, reason string) {
	s.emit(map[string]any{
		"type":   "skipped",
		"task":   task,
		"reason": reason,
	})
}

func (s *EventStream) EmitComplete(status string, durationMS int64) {
	s.emit(map[string]any{
		"type":        "complete",
		"status":      status,
		"duration_ms": durationMS,
	})
}

func eventTS() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
