package events

import (
	"fmt"
	"log"
	"sing-box-smart/internal/domain"
	"time"
)

// Recorder is owned by the app loop. Trace writes detailed logs without flooding the UI.
type Recorder struct {
	Logger *log.Logger
	Recent []domain.Event
}

func (r *Recorder) Record(kind, message string) {
	r.Recent = append(r.Recent, domain.Event{At: time.Now(), Kind: kind, Message: message})
	if len(r.Recent) > 100 {
		r.Recent = r.Recent[len(r.Recent)-100:]
	}
	r.Logger.Printf("kind=%s %s", kind, message)
}
func (r *Recorder) Trace(message string, args ...any) {
	r.Logger.Printf("%s args=%s", message, fmt.Sprint(args))
}
