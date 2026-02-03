// Package protocol: types for process data
package protocol

// ProcessInfo represents the data of a process sent to the server
type ProcessInfo struct {
	PID    int32   `json:"pid"`
	Name   string  `json:"name"`
	CPU    float64 `json:"cpu"`
	Memory float32 `json:"memory"`
}

// CommandRequest data the client sends to the server
type CommandRequest struct {
	Action string `json:"action"` // START, STOP
	Target string `json:"target"` // PID or binary name
}

// CommandResponse generic response of the server
type CommandResponse struct {
	Success   bool          `json:"success"`
	Message   string        `json:"message"`
	Processes []ProcessInfo `json:"processes,omitempty"`
}

type ServerBeacon struct {
	ID      string `json:"id"`
	TCPPort string `json:"tcp_port"`
	Address string `json:"address"`
}
