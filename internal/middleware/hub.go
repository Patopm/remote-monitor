// Package middleware implements the central hub and API
package middleware

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Patopm/remote-monitor/internal/protocol"
)

// AgentConnection holds the state of a single connected agent
type AgentConnection struct {
	ID   string
	Info protocol.AgentInfo

	conn    *websocket.Conn
	writeMu sync.Mutex

	processes   []protocol.ProcessInfo
	processesMu sync.RWMutex

	pending   map[string]chan protocol.AgentCommandResponse
	pendingMu sync.Mutex
}

// writeJSON safely writes a JSON message to the agent's WebSocket
func (a *AgentConnection) writeJSON(v any) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.WriteJSON(v)
}

// Hub manages all connected agents
type Hub struct {
	agents map[string]*AgentConnection
	mu     sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		agents: make(map[string]*AgentConnection),
	}
}

// Register adds an agent to the hub
func (h *Hub) Register(agent *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agents[agent.ID] = agent
	log.Printf("[Hub] Agent registered: %s (%s)", agent.ID, agent.Info.Hostname)
}

// Unregister removes an agent and cleans up resources
func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	agent, ok := h.agents[id]
	if !ok {
		return
	}

	// Close all pending command channels so waiters unblock
	agent.pendingMu.Lock()
	for _, ch := range agent.pending {
		close(ch)
	}
	agent.pendingMu.Unlock()

	if err := agent.conn.Close(); err != nil {
		log.Fatalf("[Hub] Error closing agent connection: %v", err)
	}
	delete(h.agents, id)
	log.Printf("[Hub] Agent unregistered: %s", id)
}

// GetAgent returns an agent by ID
func (h *Hub) GetAgent(id string) (*AgentConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	a, ok := h.agents[id]
	return a, ok
}

// ListAgents returns info for all connected agents
func (h *Hub) ListAgents() []protocol.AgentInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]protocol.AgentInfo, 0, len(h.agents))
	for _, a := range h.agents {
		list = append(list, a.Info)
	}
	return list
}

// GetProcesses returns the cached process list for an agent
func (h *Hub) GetProcesses(agentID string) ([]protocol.ProcessInfo, bool) {
	agent, ok := h.GetAgent(agentID)
	if !ok {
		return nil, false
	}
	agent.processesMu.RLock()
	defer agent.processesMu.RUnlock()
	procs := make([]protocol.ProcessInfo, len(agent.processes))
	copy(procs, agent.processes)
	return procs, true
}

// SendCommand sends a command to an agent and waits for the response
func (h *Hub) SendCommand(
	agentID, action, target string,
) (*protocol.AgentCommandResponse, error) {
	agent, ok := h.GetAgent(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	cmdID := generateID()
	cmd := protocol.AgentCommand{
		CommandID: cmdID,
		Action:    action,
		Target:    target,
	}

	// Create a channel to receive the response
	respCh := make(chan protocol.AgentCommandResponse, 1)
	agent.pendingMu.Lock()
	agent.pending[cmdID] = respCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, cmdID)
		agent.pendingMu.Unlock()
	}()

	// Marshal command data and wrap in WSMessage
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	msg := protocol.WSMessage{Type: "command", Data: data}
	if err := agent.writeJSON(msg); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("agent disconnected while waiting")
		}
		return &resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("command timed out after 10s")
	}
}

// HandleAgentConnection handles the full lifecycle of an agent WebSocket
func (h *Hub) HandleAgentConnection(conn *websocket.Conn) {
	// First message must be registration
	var msg protocol.WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("[Hub] Failed to read registration: %v", err)
		if err := conn.Close(); err != nil {
			log.Fatalf("[Hub] Error closing connection: %v", err)
		}
		return
	}

	if msg.Type != "register" {
		log.Printf("[Hub] Expected 'register', got '%s'", msg.Type)
		if err := conn.Close(); err != nil {
			log.Fatalf("[Hub] Error closing connection: %v", err)
		}
		return
	}

	var reg protocol.AgentRegistration
	if err := json.Unmarshal(msg.Data, &reg); err != nil {
		log.Printf("[Hub] Invalid registration payload: %v", err)
		if err := conn.Close(); err != nil {
			log.Fatalf("[Hub] Error closing connection: %v", err)
		}
		return
	}

	// Validate Agent Secret Key
	expectedSecret := os.Getenv("AGENT_SECRET_KEY")
	if expectedSecret == "" {
		expectedSecret = "default-agent-secret" // Para desarrollo
	}

	if reg.SecretKey != expectedSecret {
		log.Printf("[Hub] Unauthorized agent connection attempt from %s", reg.Hostname)
		if err := conn.Close(); err != nil {
			log.Fatalf("[Hub] Error closing connection: %v", err)
		}
		return
	}

	// Build a unique ID: hostname-randomsuffix
	agentID := fmt.Sprintf("%s-%s", reg.Hostname, generateID()[:8])
	now := time.Now()

	agent := &AgentConnection{
		ID: agentID,
		Info: protocol.AgentInfo{
			ID:          agentID,
			Hostname:    reg.Hostname,
			OS:          reg.OS,
			ConnectedAt: now,
			LastSeen:    now,
		},
		conn:    conn,
		pending: make(map[string]chan protocol.AgentCommandResponse),
	}

	h.Register(agent)
	defer h.Unregister(agent.ID)

	// Read loop: process incoming messages from the agent
	for {
		var incoming protocol.WSMessage
		if err := conn.ReadJSON(&incoming); err != nil {
			log.Printf("[Hub] Agent %s read error: %v", agent.ID, err)
			return
		}

		// Update last seen timestamp
		h.mu.Lock()
		agent.Info.LastSeen = time.Now()
		h.mu.Unlock()

		switch incoming.Type {
		case "telemetry":
			var telemetry protocol.AgentTelemetry
			if err := json.Unmarshal(incoming.Data, &telemetry); err != nil {
				log.Printf(
					"[Hub] Bad telemetry from %s: %v",
					agent.ID, err,
				)
				continue
			}
			agent.processesMu.Lock()
			agent.processes = telemetry.Processes
			agent.processesMu.Unlock()

		case "command_response":
			var resp protocol.AgentCommandResponse
			if err := json.Unmarshal(incoming.Data, &resp); err != nil {
				log.Printf(
					"[Hub] Bad command_response from %s: %v",
					agent.ID, err,
				)
				continue
			}
			agent.pendingMu.Lock()
			if ch, ok := agent.pending[resp.CommandID]; ok {
				ch <- resp
			}
			agent.pendingMu.Unlock()

		default:
			log.Printf(
				"[Hub] Unknown message type '%s' from %s",
				incoming.Type, agent.ID,
			)
		}
	}
}

// generateID creates a random hex string for unique IDs
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
