package middleware

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/Patopm/remote-monitor/internal/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (fine for development)
	},
}

// RegisterRoutes sets up all HTTP and WebSocket routes
func RegisterRoutes(mux *http.ServeMux, hub *Hub) {
	// Public routes
	mux.HandleFunc("POST /api/login", loginHandler())
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, protocol.APIResponse{Success: true, Message: "ok"})
	})

	// WebSocket for agents
	mux.HandleFunc("/ws/agent", wsAgentHandler(hub))

	// Protected REST API routes (Wrapped with AuthMiddleware)
	mux.HandleFunc("GET /api/agents", AuthMiddleware(listAgentsHandler(hub)))
	mux.HandleFunc("GET /api/agents/{id}/processes", AuthMiddleware(getProcessesHandler(hub)))
	mux.HandleFunc("POST /api/agents/{id}/kill", AuthMiddleware(killProcessHandler(hub)))
}

func loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req protocol.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, protocol.APIResponse{Success: false, Message: "Bad request"})
			return
		}

		if req.Username == "admin" && req.Password == "redhat2026" {
			token, err := GenerateJWT(req.Username)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, protocol.APIResponse{Success: false, Message: "Token error"})
				return
			}
			writeJSON(w, http.StatusOK, protocol.APIResponse{
				Success: true,
				Data:    protocol.LoginResponse{Token: token},
			})
			return
		}

		writeJSON(w, http.StatusUnauthorized, protocol.APIResponse{Success: false, Message: "Invalid credentials"})
	}
}

func wsAgentHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[API] WebSocket upgrade failed: %v", err)
			return
		}
		// This blocks until the agent disconnects
		hub.HandleAgentConnection(conn)
	}
}

func listAgentsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		agents := hub.ListAgents()
		writeJSON(w, http.StatusOK, protocol.APIResponse{
			Success: true,
			Data:    agents,
		})
	}
}

func getProcessesHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")

		procs, ok := hub.GetProcesses(agentID)
		if !ok {
			writeJSON(w, http.StatusNotFound, protocol.APIResponse{
				Success: false,
				Message: "Agent not found: " + agentID,
			})
			return
		}

		writeJSON(w, http.StatusOK, protocol.APIResponse{
			Success: true,
			Data:    procs,
		})
	}
}

func killProcessHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")

		var req protocol.KillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, protocol.APIResponse{
				Success: false,
				Message: "Invalid request body",
			})
			return
		}

		resp, err := hub.SendCommand(agentID, "STOP", req.PID)
		if err != nil {
			writeJSON(
				w,
				http.StatusInternalServerError,
				protocol.APIResponse{
					Success: false,
					Message: err.Error(),
				},
			)
			return
		}

		writeJSON(w, http.StatusOK, protocol.APIResponse{
			Success: resp.Success,
			Message: resp.Message,
		})
	}
}

// CORSMiddleware adds CORS headers for frontend development
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PUT, DELETE, OPTIONS",
		)
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[API] Failed to write response: %v", err)
	}
}
