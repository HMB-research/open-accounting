package plugin

import (
	"time"

	"github.com/google/uuid"
)

// RuntimeLifecycleState describes the operator-visible lifecycle of a plugin backend runtime.
type RuntimeLifecycleState string

const (
	RuntimeStateNotConfigured RuntimeLifecycleState = "not_configured"
	RuntimeStateExternal      RuntimeLifecycleState = "external"
	RuntimeStateNotLoaded     RuntimeLifecycleState = "not_loaded"
	RuntimeStateStarting      RuntimeLifecycleState = "starting"
	RuntimeStateRunning       RuntimeLifecycleState = "running"
	RuntimeStateStopped       RuntimeLifecycleState = "stopped"
	RuntimeStateExited        RuntimeLifecycleState = "exited"
	RuntimeStateBackoff       RuntimeLifecycleState = "backoff"
	RuntimeStateFailed        RuntimeLifecycleState = "failed"
)

// RuntimeHealthState describes the latest known runtime health signal.
type RuntimeHealthState string

const (
	RuntimeHealthNotApplicable RuntimeHealthState = "not_applicable"
	RuntimeHealthUnknown       RuntimeHealthState = "unknown"
	RuntimeHealthHealthy       RuntimeHealthState = "healthy"
	RuntimeHealthUnhealthy     RuntimeHealthState = "unhealthy"
)

// PluginRuntimeStatus is the operator-facing status snapshot for a plugin backend runtime.
type PluginRuntimeStatus struct {
	PluginID          uuid.UUID             `json:"plugin_id"`
	PluginName        string                `json:"plugin_name"`
	DisplayName       string                `json:"display_name,omitempty"`
	Runtime           string                `json:"runtime"`
	State             RuntimeLifecycleState `json:"state"`
	Health            RuntimeHealthState    `json:"health"`
	Message           string                `json:"message,omitempty"`
	BaseURL           string                `json:"base_url,omitempty"`
	PID               *int                  `json:"pid,omitempty"`
	StartedAt         *time.Time            `json:"started_at,omitempty"`
	ReadyAt           *time.Time            `json:"ready_at,omitempty"`
	ExitedAt          *time.Time            `json:"exited_at,omitempty"`
	LastHealthCheckAt *time.Time            `json:"last_health_check_at,omitempty"`
	BackoffUntil      *time.Time            `json:"backoff_until,omitempty"`
	ExitCode          *int                  `json:"exit_code,omitempty"`
	RestartCount      int                   `json:"restart_count"`
	CrashCount        int                   `json:"crash_count"`
	HookCount         int                   `json:"hook_count"`
	RouteCount        int                   `json:"route_count"`
	LastExitError     string                `json:"last_exit_error,omitempty"`
	LastHealthError   string                `json:"last_health_error,omitempty"`
	LastError         string                `json:"last_error,omitempty"`
	LastOutput        string                `json:"last_output,omitempty"`
}
