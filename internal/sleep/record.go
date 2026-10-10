package sleep

import (
	"sing-box-smart/internal/domain"
	"time"
)

// Record is the sleep package's convenient name for the API-level record.
type Record = domain.SleepRecord

// Recorder keeps a bounded in-memory history of automatic sleep intervals.
// It is intentionally owned by the application loop, so it does not need a
// mutex and cannot introduce another concurrent state owner.
type Recorder struct {
	limit   int
	records []Record
	active  *Record
}

func NewRecorder(limit int) *Recorder {
	if limit <= 0 {
		limit = 3
	}
	return &Recorder{limit: limit}
}

// Start begins a new interval. Duplicate starts are ignored so a repeated
// sleep transition cannot create phantom records.
func (r *Recorder) Start(at time.Time, reason string) {
	if r.active != nil {
		return
	}
	if r.limit <= 0 {
		r.limit = 3
	}
	if at.IsZero() {
		at = time.Now()
	}
	// The active interval counts toward the same limit as completed records.
	// Evict before opening it so Snapshot never exposes more than the limit.
	if len(r.records) >= r.limit {
		r.records = r.records[len(r.records)-r.limit+1:]
	}
	r.active = &Record{StartedAt: at, Reason: reason, Active: true}
}

// End closes the current interval. Ending without an active interval is a
// no-op, which keeps wake paths safe when state changes race at a boundary.
func (r *Recorder) End(at time.Time, wakeReason string) {
	if r.active == nil {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	if at.Before(r.active.StartedAt) {
		at = r.active.StartedAt
	}
	record := *r.active
	record.EndedAt = &at
	record.DurationSeconds = at.Sub(record.StartedAt).Seconds()
	record.WakeReason = wakeReason
	record.Active = false
	r.records = append(r.records, record)
	if len(r.records) > r.limit {
		r.records = r.records[len(r.records)-r.limit:]
	}
	r.active = nil
}

// Snapshot returns oldest-to-newest records. The active record, when present,
// is appended last so the UI can present the newest interval first while its
// duration continues to advance without mutating stored history.
func (r *Recorder) Snapshot(now time.Time) []Record {
	if now.IsZero() {
		now = time.Now()
	}
	result := make([]Record, 0, len(r.records)+1)
	result = append(result, r.records...)
	if r.active != nil {
		active := *r.active
		active.DurationSeconds = now.Sub(active.StartedAt).Seconds()
		if active.DurationSeconds < 0 {
			active.DurationSeconds = 0
		}
		result = append(result, active)
	}
	return result
}
