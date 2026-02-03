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

func getBroadcastAddr(port string) string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() != nil {
				ip := ipnet.IP.To4()
				mask := ipnet.Mask
				broadcast := net.IP(make([]byte, 4))
				for i := range ip {
					broadcast[i] = ip[i] | ^mask[i]
				}
				return broadcast.String() + port
			}
		}
	}
	return "255.255.255.255" + port
}

func startDiscoveryBeacon(udpPort, tcpPort string) {
	broadcastTarget := getBroadcastAddr(udpPort)
	fmt.Printf("Enviando beacons a: %s\n", broadcastTarget)

	addr, _ := net.ResolveUDPAddr("udp", broadcastTarget)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Printf("Error UDP: %v", err)
		return
	}
	hostname, _ := os.Hostname()

	for {
		beacon := protocol.ServerBeacon{ID: hostname, TCPPort: tcpPort}
		data, _ := json.Marshal(beacon)
		if _, err := conn.Write(data); err != nil {
			log.Fatalf("Error al hacer broadcast: %v", err)
		}
		time.Sleep(2 * time.Second)
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
