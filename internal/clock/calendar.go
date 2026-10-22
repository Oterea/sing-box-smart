// Package clock owns time semantics shared by the application.
//
// Durations and instants are kept as time.Duration/time.Time. Calendar
// operations are deliberately centralized here so a server's local timezone
// can never silently decide which user-facing day a sample belongs to.
package clock

import (
	"fmt"
	"time"
)

const DefaultTimezone = "Asia/Shanghai"

type Calendar struct {
	Location      *time.Location
	Timezone      string
	BucketMinutes int
}

type Bucket struct {
	Day         string
	Index       int
	MinuteOfDay int
	Start       time.Time
}

func NewCalendar(timezone string, bucketMinutes int) (Calendar, error) {
	if timezone == "" {
		timezone = DefaultTimezone
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Calendar{}, fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	if bucketMinutes <= 0 || 24*60%bucketMinutes != 0 {
		return Calendar{}, fmt.Errorf("bucket minutes must divide one day: %d", bucketMinutes)
	}
	return Calendar{Location: location, Timezone: timezone, BucketMinutes: bucketMinutes}, nil
}

func MustCalendar(timezone string, bucketMinutes int) Calendar {
	calendar, err := NewCalendar(timezone, bucketMinutes)
	if err != nil {
		panic(err)
	}
	return calendar
}

func (c Calendar) Day(t time.Time) (time.Time, string) {
	local := t.In(c.Location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, c.Location)
	return day, day.Format("2006-01-02")
}

func (c Calendar) Bucket(t time.Time) Bucket {
	day, date := c.Day(t)
	local := t.In(c.Location)
	minute := local.Hour()*60 + local.Minute()
	index := minute / c.BucketMinutes
	startMinute := index * c.BucketMinutes
	return Bucket{Day: date, Index: index, MinuteOfDay: startMinute, Start: day.Add(time.Duration(startMinute) * time.Minute)}
}

func (c Calendar) Today(t time.Time) string {
	_, date := c.Day(t)
	return date
}
