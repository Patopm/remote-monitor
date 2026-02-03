package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Patopm/remote-monitor/internal/process"
	"github.com/Patopm/remote-monitor/internal/protocol"
)

func main() {
	tcpPort, udpPort := ":8080", ":9999"

	go startDiscoveryBeacon(udpPort, tcpPort)

	listener, err := net.Listen("tcp", tcpPort)
	if err != nil {
		log.Fatalf("Error TCP: %v", err)
	}

	defer func() {
		err := listener.Close()
		if err != nil {
			log.Fatalf("Error al cerrar el socket: %v", err)
		}
	}()

	fmt.Printf("Servidor escuchando en TCP %s y anunciando por UDP %s\n", tcpPort, udpPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error conexión: %v", err)
			continue
		}
		go handleClient(conn)
	}
}

func startDiscoveryBeacon(udpPort, tcpPort string) {
	addr, _ := net.ResolveUDPAddr("udp", "255.255.255.255"+udpPort)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Printf("Error UDP: %v", err)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Error al cerrar la conexión: %v", err)
		}
	}()

	hostname, _ := os.Hostname()
	beacon := protocol.ServerBeacon{
		ID:      hostname,
		TCPPort: tcpPort,
	}
	data, _ := json.Marshal(beacon)

	for {
		_, err := conn.Write(data)
		if err != nil {
			log.Printf("Error enviando beacon: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
}

func handleClient(conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Error al cerrar la conexión: %v", err)
		}
	}()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req protocol.CommandRequest
		if err := decoder.Decode(&req); err != nil {
			return
		}

		var resp protocol.CommandResponse
		switch req.Action {
		case "LIST":
			procs, _ := process.ListProcesses()
			resp = protocol.CommandResponse{Success: true, Processes: procs}
		case "STOP":
			err := process.StopProcess(req.Target)
			resp = protocol.CommandResponse{Success: err == nil, Message: "Action Stop"}
		default:
			resp = protocol.CommandResponse{Success: false, Message: "Unknown"}
		}
		if err := encoder.Encode(resp); err != nil {
			log.Fatalf("Error al serializar la respuesta: %v", err)
		}
	}
}
