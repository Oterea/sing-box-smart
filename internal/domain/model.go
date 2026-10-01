// Package domain defines data exchanged between modules; it contains no I/O.
package domain

import "time"

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Tier string

const (
	TierCurrent   Tier = "current"
	TierCandidate Tier = "candidate"
	TierOrdinary  Tier = "ordinary"
)

type Airport struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selector string `json:"selector"`
	Nodes    []Node `json:"-"`
}
type Probe struct {
	Success bool
	DelayMS float64
	Error   string
	At      time.Time
}
type Metrics struct {
	L, S, N, F float64
	HasLatency bool
}
type Sample struct {
	At      time.Time `json:"at"`
	Success bool      `json:"success"`
	DelayMS *float64  `json:"delay_ms"`
	Score   *float64  `json:"score"`
	Error   string    `json:"error,omitempty"`
}
type NodeView struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Score             *float64   `json:"score"`
	ScoreOverflow     bool       `json:"score_overflow"`
	DelayMS           *float64   `json:"delay_ms"`
	LastSuccess       bool       `json:"last_success"`
	LastAt            *time.Time `json:"last_at"`
	Samples           []Sample   `json:"history"`
	Checks            int        `json:"checks"`
	Current           bool       `json:"current"`
	Probing           bool       `json:"probing"`
	Frequency         string     `json:"frequency"`
	RecoveryRemaining int        `json:"recovery_remaining"`
	Tier              Tier       `json:"tier"`
}
type Event struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Message string    `json:"message"`
}
type Snapshot struct {
	Revision       uint64     `json:"revision"`
	Paused         bool       `json:"paused"`
	Mode           string     `json:"mode"`
	Phase          string     `json:"phase"`
	AirportID      string     `json:"airport_id"`
	Airports       []Airport  `json:"airports"`
	CurrentID      string     `json:"current_id"`
	PendingID      string     `json:"pending_id"`
	APIAddress     string     `json:"api_address"`
	GroupRoot      string     `json:"group_root"`
	GroupPattern   string     `json:"group_pattern"`
	APIHealthy     bool       `json:"api_healthy"`
	StartedAt      time.Time  `json:"started_at"`
	Now            time.Time  `json:"now"`
	StartupSeconds float64    `json:"startup_seconds"`
	Nodes          []NodeView `json:"nodes"`
	Events         []Event    `json:"events"`
}
