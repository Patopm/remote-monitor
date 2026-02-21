// Package protocol defines shared types for communication
package protocol

import (
	"encoding/json"
	"time"
)

// ProcessInfo represents the data of a running process
type ProcessInfo struct {
	PID    int32   `json:"pid"`
	Name   string  `json:"name"`
	CPU    float64 `json:"cpu"`
	Memory float32 `json:"memory"`
}

// --- WebSocket Messages (Agent <-> Middleware) ---

// WSMessage is the envelope for all WebSocket communication
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// AgentRegistration is sent by the agent upon connecting
type AgentRegistration struct {
	Hostname  string `json:"hostname"`
	OS        string `json:"os"`
	SecretKey string `json:"secret_key"`
}

// AgentTelemetry contains cached process data sent periodically
type AgentTelemetry struct {
	Processes []ProcessInfo `json:"processes"`
}

// AgentCommand is sent from middleware to agent
type AgentCommand struct {
	CommandID string `json:"command_id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
}

// AgentCommandResponse is the agent's reply to a command
type AgentCommandResponse struct {
	CommandID string `json:"command_id"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
}

// --- REST API Types (Frontend <-> Middleware) ---

// AgentInfo represents a connected agent exposed via the API
type AgentInfo struct {
	ID          string    `json:"id"`
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

// KillRequest is the JSON body for the kill endpoint
type KillRequest struct {
	PID string `json:"pid"`
}

// APIResponse is a generic API response envelope
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// LoginRequest is the JSON body for the login endpoint
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse contains the JWT token
type LoginResponse struct {
	Token string `json:"token"`
}
