package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Patopm/remote-monitor/internal/process"
	"github.com/Patopm/remote-monitor/internal/protocol"
)

func main() {
	middlewareURL := flag.String("middleware", "ws://localhost:8080/ws/agent", "Middleware WebSocket URL")
	interval := flag.Duration("interval", 2*time.Second, "Telemetry send interval")
	secretKey := flag.String("secret", "default-agent-secret", "Shared secret for authentication")
	flag.Parse()

	log.Printf("[Agent] Target middleware: %s", *middlewareURL)

	for {
		err := run(*middlewareURL, *interval, *secretKey)
		log.Printf("[Agent] Disconnected: %v", err)
		log.Printf("[Agent] Reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func run(url string, interval time.Duration, secret string) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Error al cerrar el socket: %v", err)
		}
	}()

	var writeMu sync.Mutex

	// Helper to safely write JSON
	safeWrite := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	// --- Registration ---
	hostname, _ := os.Hostname()
	reg := protocol.AgentRegistration{
		Hostname:  hostname,
		OS:        runtime.GOOS,
		SecretKey: secret,
	}
	regData, _ := json.Marshal(reg)
	if err := safeWrite(protocol.WSMessage{
		Type: "register",
		Data: regData,
	}); err != nil {
		return err
	}
	log.Printf("[Agent] Registered as '%s' (%s)", hostname, runtime.GOOS)

	// --- Telemetry goroutine ---
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				procs, err := process.ListProcesses()
				if err != nil {
					log.Printf("[Agent] Error reading processes: %v", err)
					continue
				}

				telemetry := protocol.AgentTelemetry{Processes: procs}
				data, _ := json.Marshal(telemetry)
				msg := protocol.WSMessage{Type: "telemetry", Data: data}

				if err := safeWrite(msg); err != nil {
					log.Printf("[Agent] Error sending telemetry: %v", err)
					return
				}
			}
		}
	}()

	// --- Command read loop ---
	for {
		var msg protocol.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}

		switch msg.Type {
		case "command":
			var cmd protocol.AgentCommand
			if err := json.Unmarshal(msg.Data, &cmd); err != nil {
				log.Printf("[Agent] Bad command payload: %v", err)
				continue
			}

			log.Printf(
				"[Agent] Received command: %s (target: %s)",
				cmd.Action, cmd.Target,
			)
			resp := executeCommand(cmd)

			data, _ := json.Marshal(resp)
			respMsg := protocol.WSMessage{
				Type: "command_response",
				Data: data,
			}
			if err := safeWrite(respMsg); err != nil {
				return err
			}

		default:
			log.Printf("[Agent] Unknown message type: %s", msg.Type)
		}
	}
}

func executeCommand(cmd protocol.AgentCommand) protocol.AgentCommandResponse {
	resp := protocol.AgentCommandResponse{
		CommandID: cmd.CommandID,
	}

	switch cmd.Action {
	case "STOP":
		if err := process.StopProcess(cmd.Target); err != nil {
			resp.Success = false
			resp.Message = err.Error()
		} else {
			resp.Success = true
			resp.Message = "Process stopped successfully"
		}
	default:
		resp.Success = false
		resp.Message = "Unknown action: " + cmd.Action
	}

	return resp
}
