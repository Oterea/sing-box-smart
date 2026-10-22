package stability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sing-box-smart/internal/clock"
	"sing-box-smart/internal/domain"
	"sort"
	"sync"
	"time"
)

const (
	LongTermRetention = 14
	LongTermBucket    = 15 * time.Minute
	longTermBuckets   = 24 * 60 / 15
)

var histogramEdges = [...]float64{50, 100, 150, 200, 250, 300, 400, 500, 600, 800, 1000, 1500, 2000, 3000, 5000}

type longTermBucket struct {
	At      time.Time                       `json:"at"`
	Samples int                             `json:"samples"`
	Success int                             `json:"success"`
	Failure int                             `json:"failure"`
	MaxFail int                             `json:"max_fail"`
	Bins    [len(histogramEdges) + 1]uint32 `json:"bins"`
}
type longTermDay struct {
	Date    string                 `json:"date"`
	Buckets map[int]longTermBucket `json:"buckets"`
}
type longTermNode struct {
	AirportID  string                  `json:"airport_id"`
	NodeID     string                  `json:"node_id"`
	Name       string                  `json:"name"`
	Days       map[string]*longTermDay `json:"days"`
	FailStreak int                     `json:"fail_streak"`
}
type longTermFile struct {
	Version int                      `json:"version"`
	Nodes   map[string]*longTermNode `json:"nodes"`
}

const longTermStorageVersion = 2

// LongTermPersistence isolates aggregation from the storage format and I/O.
// The app can keep using the in-memory aggregator if persistence is disabled,
// while production uses the atomic JSON implementation below.
type LongTermPersistence interface {
	Load() (longTermFile, error)
	Save(longTermFile) error
}

// LongTerm stores a small, bounded time-of-day history. It never retains raw
// probes: each node has fourteen days of ninety-six fixed fifteen-minute
// buckets and each bucket keeps only counters plus a latency histogram.
type LongTerm struct {
	mu       sync.Mutex
	persist  LongTermPersistence
	calendar clock.Calendar
	nodes    map[string]*longTermNode
	lastSave time.Time
}

func NewLongTerm(path string, calendars ...clock.Calendar) (*LongTerm, error) {
	return NewLongTermWithPersistence(calendarFor(calendars), &jsonLongTermPersistence{path: path})
}

func NewLongTermWithPersistence(calendar clock.Calendar, persist LongTermPersistence) (*LongTerm, error) {
	l := &LongTerm{persist: persist, calendar: calendar, nodes: make(map[string]*longTermNode)}
	if persist == nil {
		return l, nil
	}
	file, err := persist.Load()
	if err != nil {
		return nil, err
	}
	// Version 1 used time.Local on the server. It cannot be migrated safely
	// because the original timezone is not stored, so start a clean v2 window.
	if file.Version == longTermStorageVersion && file.Nodes != nil {
		l.nodes = file.Nodes
	}
	l.pruneLocked(time.Now())
	return l, nil
}

func calendarFor(calendars []clock.Calendar) clock.Calendar {
	if len(calendars) > 0 && calendars[0].Location != nil {
		return calendars[0]
	}
	return clock.MustCalendar(clock.DefaultTimezone, 15)
}

func nodeKey(airportID, nodeID string) string { return airportID + "\x00" + nodeID }

func (l *LongTerm) Record(airportID, nodeID, name string, p domain.Probe) error {
	if p.At.IsZero() {
		p.At = time.Now()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.calendar.Bucket(p.At)
	date := bucket.Day
	key := nodeKey(airportID, nodeID)
	n := l.nodes[key]
	if n == nil {
		n = &longTermNode{AirportID: airportID, NodeID: nodeID, Name: name, Days: make(map[string]*longTermDay)}
		l.nodes[key] = n
	}
	n.Name = name
	day := n.Days[date]
	if day == nil {
		day = &longTermDay{Date: date, Buckets: make(map[int]longTermBucket)}
		n.Days[date] = day
	}
	index := bucket.Index
	b := day.Buckets[index]
	if b.At.IsZero() {
		b.At = bucket.Start
	}
	b.Samples++
	if !p.Success {
		n.FailStreak++
		b.Failure++
		if n.FailStreak > b.MaxFail {
			b.MaxFail = n.FailStreak
		}
	} else {
		n.FailStreak = 0
		b.Success++
		bin := len(histogramEdges)
		for i, edge := range histogramEdges {
			if p.DelayMS <= edge {
				bin = i
				break
			}
		}
		b.Bins[bin]++
	}
	day.Buckets[index] = b
	return l.maybeSaveLocked(p.At)
}

func (l *LongTerm) maybeSaveLocked(now time.Time) error {
	if !l.lastSave.IsZero() && now.Sub(l.lastSave) < 5*time.Minute {
		return nil
	}
	l.pruneLocked(now)
	return l.saveLocked()
}

func (l *LongTerm) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(time.Now())
	return l.saveLocked()
}

func (l *LongTerm) saveLocked() error {
	if l.persist == nil {
		return nil
	}
	if err := l.persist.Save(longTermFile{Version: longTermStorageVersion, Nodes: l.nodes}); err != nil {
		return err
	}
	l.lastSave = time.Now()
	return nil
}

type jsonLongTermPersistence struct{ path string }

func (p *jsonLongTermPersistence) Load() (longTermFile, error) {
	b, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		return longTermFile{}, nil
	}
	if err != nil {
		return longTermFile{}, err
	}
	var file longTermFile
	if err := json.Unmarshal(b, &file); err != nil {
		return longTermFile{}, fmt.Errorf("读取长期稳定性数据: %w", err)
	}
	return file, nil
}

func (p *jsonLongTermPersistence) Save(file longTermFile) error {
	if p.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return err
	}
	tmp := p.path + ".tmp"
	b, err := json.Marshal(file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (l *LongTerm) pruneLocked(now time.Time) {
	today, _ := l.calendar.Day(now)
	cutoff := today.AddDate(0, 0, -(LongTermRetention - 1))
	for key, node := range l.nodes {
		for date := range node.Days {
			parsed, err := time.ParseInLocation("2006-01-02", date, l.calendar.Location)
			if err != nil || parsed.Before(cutoff) {
				delete(node.Days, date)
			}
		}
		if len(node.Days) == 0 {
			delete(l.nodes, key)
		}
	}
}

func (l *LongTerm) Query(airportID, nodeID string, now time.Time, yellow float64) domain.LongTermStability {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	n := l.nodes[nodeKey(airportID, nodeID)]
	result := domain.LongTermStability{Retention: LongTermRetention, BucketMins: l.calendar.BucketMinutes, YellowMS: int(yellow), Timezone: l.calendar.Timezone}
	if n == nil {
		return result
	}
	for date, day := range n.Days {
		out := domain.LongTermDay{Date: date, Buckets: make([]domain.LongTermBucket, 0, longTermBuckets)}
		var total longTermBucket
		indices := make([]int, 0, len(day.Buckets))
		for index := range day.Buckets {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			bucket := day.Buckets[index]
			if bucket.Samples == 0 {
				continue
			}
			p50 := percentileBucket(bucket, .50)
			p90 := percentileBucket(bucket, .90)
			high := highLatency(bucket, yellow)
			out.Buckets = append(out.Buckets, domain.LongTermBucket{At: bucket.At, MinuteOfDay: index * l.calendar.BucketMinutes, P50MS: p50, P90MS: p90, SuccessRate: float64(bucket.Success) / float64(bucket.Samples), HighLatencyRate: float64(high) / float64(bucket.Samples), MaxConsecutiveFail: bucket.MaxFail, HasData: true})
			total.Samples += bucket.Samples
			total.Success += bucket.Success
			total.Failure += bucket.Failure
			if bucket.MaxFail > total.MaxFail {
				total.MaxFail = bucket.MaxFail
			}
			for i, count := range bucket.Bins {
				total.Bins[i] += count
			}
		}
		out.Summary = summarizeLongTermBucket(total, yellow)
		result.Days = append(result.Days, out)
	}
	return result
}

func summarizeLongTermBucket(bucket longTermBucket, yellow float64) domain.StabilitySummary {
	summary := domain.StabilitySummary{WindowSeconds: 24 * 60 * 60, Samples: bucket.Samples, Successes: bucket.Success, Failures: bucket.Failure, MaxConsecutiveFails: bucket.MaxFail}
	if bucket.Samples > 0 {
		summary.SuccessRate = float64(bucket.Success) / float64(bucket.Samples)
	}
	if bucket.Success > 0 {
		summary.P50MS = percentileBucket(bucket, .50)
		summary.P90MS = percentileBucket(bucket, .90)
	}
	summary.HighLatencyCount = highLatency(bucket, yellow)
	if bucket.Samples > 0 {
		summary.HighLatencyRate = float64(summary.HighLatencyCount) / float64(bucket.Samples)
	}
	return summary
}

func percentileBucket(bucket longTermBucket, q float64) *float64 {
	if bucket.Success == 0 {
		return nil
	}
	target := int(float64(bucket.Success-1)*q) + 1
	seen := 0
	for i, count := range bucket.Bins {
		seen += int(count)
		if seen >= target {
			value := float64(5000)
			if i < len(histogramEdges) {
				value = histogramEdges[i]
			}
			return &value
		}
	}
	return nil
}

func highLatency(bucket longTermBucket, threshold float64) int {
	count := 0
	for i, value := range bucket.Bins {
		lower := 0.0
		if i > 0 {
			lower = histogramEdges[i-1]
		}
		if lower >= threshold {
			count += int(value)
		}
	}
	return count
}
