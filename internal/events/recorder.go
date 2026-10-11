package events

import (
	"encoding/json"
	"fmt"
	"log"
	"sing-box-smart/internal/domain"
	"time"
)

type Level uint8

const (
	Debug Level = iota
	Info
	Warn
	Error
)

// Recorder is owned by the app loop. Trace writes detailed logs without flooding the UI.
type Recorder struct {
	Logger   *log.Logger
	MinLevel Level
	Recent   []domain.Event
}

func (r *Recorder) Debug(module, event, message string, fields map[string]any) {
	r.Log(Debug, module, event, message, fields)
}

func (r *Recorder) Info(module, event, message string, fields map[string]any) {
	r.Log(Info, module, event, message, fields)
}

func (r *Recorder) Warn(module, event, message string, fields map[string]any) {
	r.Log(Warn, module, event, message, fields)
}

func (r *Recorder) Error(module, event, message string, fields map[string]any) {
	r.Log(Error, module, event, message, fields)
}

func (r *Recorder) Record(kind, message string) {
	r.Recent = append(r.Recent, domain.Event{At: time.Now(), Kind: kind, Message: message})
	if len(r.Recent) > 100 {
		r.Recent = r.Recent[len(r.Recent)-100:]
	}
	r.Log(levelForKind(kind), kind, "state", message, nil)
}

func levelForKind(kind string) Level {
	switch kind {
	case "debug", "trace":
		return Debug
	case "warn", "warning":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

// Log writes a structured JSON record. Fields keep their native JSON types.
func (r *Recorder) Log(level Level, module, event, message string, fields map[string]any) {
	if r.Logger == nil || level < r.MinLevel {
		return
	}
	name := map[Level]string{Debug: "DEBUG", Info: "INFO", Warn: "WARN", Error: "ERROR"}[level]
	entry := struct {
		Time    time.Time      `json:"time"`
		Level   string         `json:"level"`
		Module  string         `json:"module,omitempty"`
		Event   string         `json:"event,omitempty"`
		Message string         `json:"message,omitempty"`
		Fields  map[string]any `json:"fields,omitempty"`
	}{Time: time.Now(), Level: name, Module: module, Event: event, Message: message, Fields: fields}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	r.Logger.Print(string(b))
}

func (r *Recorder) Trace(message string, args ...any) {
	fields := make(map[string]any, (len(args)+1)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			key = fmt.Sprintf("field_%d", i/2)
		}
		fields[key] = args[i+1]
	}
	r.Log(Debug, message, "trace", message, fields)
}
