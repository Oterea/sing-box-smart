// Package domain defines data exchanged between modules; it contains no I/O.
package domain

import (
	"sing-box-smart/internal/preferences"
	"time"
)

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

type SelectionMode string

const (
	SelectionAuto   SelectionMode = "auto"
	SelectionManual SelectionMode = "manual"
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
	J          float64
	Successes  int
	HasLatency bool
}
type Sample struct {
	At      time.Time `json:"at"`
	Success bool      `json:"success"`
	DelayMS *float64  `json:"delay_ms"`
	Score   *float64  `json:"score"`
	Error   string    `json:"error,omitempty"`
}
type StabilitySample struct {
	At      time.Time `json:"at"`
	Success bool      `json:"success"`
	DelayMS *float64  `json:"delay_ms,omitempty"`
	Error   string    `json:"error,omitempty"`
}
type StabilitySummary struct {
	WindowSeconds       float64   `json:"window_seconds"`
	StartedAt           time.Time `json:"started_at"`
	Samples             int       `json:"samples"`
	Successes           int       `json:"successes"`
	Failures            int       `json:"failures"`
	SuccessRate         float64   `json:"success_rate"`
	P50MS               *float64  `json:"p50_ms,omitempty"`
	P90MS               *float64  `json:"p90_ms,omitempty"`
	HighLatencyCount    int       `json:"high_latency_count"`
	HighLatencyRate     float64   `json:"high_latency_rate"`
	MaxConsecutiveFails int       `json:"max_consecutive_failures"`
}
type NodeStability struct {
	Summary StabilitySummary  `json:"summary"`
	Samples []StabilitySample `json:"samples"`
}
type StabilityUpdate struct {
	Sequence uint64           `json:"sequence"`
	NodeID   string           `json:"node_id"`
	Sample   StabilitySample  `json:"sample"`
	Summary  StabilitySummary `json:"summary"`
}
type NodeView struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Score             *float64         `json:"score"`
	ScoreOverflow     bool             `json:"score_overflow"`
	DelayMS           *float64         `json:"delay_ms"`
	JitterMS          float64          `json:"jitter_ms"`
	JitterPenalty     float64          `json:"jitter_penalty"`
	LastSuccess       bool             `json:"last_success"`
	LastAt            *time.Time       `json:"last_at"`
	Samples           []Sample         `json:"history"`
	Checks            int              `json:"checks"`
	Current           bool             `json:"current"`
	Probing           bool             `json:"probing"`
	Frequency         string           `json:"frequency"`
	RecoveryRemaining int              `json:"recovery_remaining"`
	Tier              Tier             `json:"tier"`
	Stability         StabilitySummary `json:"stability"`
}
type Event struct {
	At      time.Time      `json:"at"`
	Kind    string         `json:"kind"`
	Level   string         `json:"level,omitempty"`
	Module  string         `json:"module,omitempty"`
	Event   string         `json:"event,omitempty"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// SleepRecord describes one automatic sleep interval exposed by the API.
type SleepRecord struct {
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds float64    `json:"duration_seconds"`
	Reason          string     `json:"reason"`
	WakeReason      string     `json:"wake_reason,omitempty"`
	WakeSource      string     `json:"wake_source,omitempty"`
	Active          bool       `json:"active"`
}
type Snapshot struct {
	Settings           preferences.Settings `json:"settings"`
	InstanceID         string               `json:"instance_id"`
	Revision           uint64               `json:"revision"`
	Paused             bool                 `json:"paused"`
	Sleeping           bool                 `json:"sleeping"`
	SleepEnabled       bool                 `json:"sleep_enabled"`
	LastActivity       *time.Time           `json:"last_activity,omitempty"`
	Mode               string               `json:"mode"`
	SelectionMode      SelectionMode        `json:"selection_mode"`
	RootSelection      string               `json:"root_selection"`
	ControlActive      bool                 `json:"control_active"`
	Phase              string               `json:"phase"`
	AirportID          string               `json:"airport_id"`
	Airports           []Airport            `json:"airports"`
	CurrentID          string               `json:"current_id"`
	PendingID          string               `json:"pending_id"`
	APIAddress         string               `json:"api_address"`
	GroupRoot          string               `json:"group_root"`
	GroupPattern       string               `json:"group_pattern"`
	APIHealthy         bool                 `json:"api_healthy"`
	StartedAt          time.Time            `json:"started_at"`
	Now                time.Time            `json:"now"`
	StartupSeconds     float64              `json:"startup_seconds"`
	ObservationSeconds float64              `json:"observation_seconds"`
	Nodes              []NodeView           `json:"nodes"`
	Events             []Event              `json:"events"`
	SleepRecords       []SleepRecord        `json:"sleep_records"`
}
