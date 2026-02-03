package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Patopm/remote-monitor/internal/protocol"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	table    table.Model
	servers  map[string]string
	mu       *sync.Mutex
	selected string
	sortBy   string
}

type (
	tickMsg        time.Time
	serverFoundMsg protocol.ServerBeacon
)

func (m *model) Init() tea.Cmd {
	return tea.Batch(tick(), listenForServers())
}

func tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func listenForServers() tea.Cmd {
	return func() tea.Msg {
		addr, _ := net.ResolveUDPAddr("udp", ":9999")
		conn, _ := net.ListenUDP("udp", addr)
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Fatalf("Error al cerrar la conexión: %v", err)
			}
		}()

		buf := make([]byte, 1024)
		for {
			n, rAddr, _ := conn.ReadFromUDP(buf)
			var b protocol.ServerBeacon
			err := json.Unmarshal(buf[:n], &b)
			if err != nil {
				log.Fatalf("Error al deserializar conexiones: %v", err)
				continue
			}
			b.Address = rAddr.IP.String() + b.TCPPort
			return serverFoundMsg(b)
		}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.sortBy = "pid"
		case "2":
			m.sortBy = "name"
		case "3":
			m.sortBy = "cpu"
		case "4":
			m.sortBy = "mem"
		case "s":
			for _, addr := range m.servers {
				m.selected = addr
				break
			}
		case "k":
			if m.selected != "" {
				s := m.table.SelectedRow()
				m.sendCommand("STOP", s[0])
			}
		}
	case serverFoundMsg:
		m.mu.Lock()
		m.servers[msg.ID] = msg.Address
		m.mu.Unlock()
		return m, listenForServers()
	case tickMsg:
		if m.selected != "" {
			m.updateProcessList()
		}
		return m, tick()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) updateProcessList() {
	conn, err := net.DialTimeout("tcp", m.selected, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Error al cerrar la conexión: %v", err)
		}
	}()

	if err := json.NewEncoder(conn).Encode(protocol.CommandRequest{Action: "LIST"}); err != nil {
		return
	}

	var resp protocol.CommandResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return
	}

	sort.Slice(resp.Processes, func(i, j int) bool {
		switch m.sortBy {
		case "cpu":
			return resp.Processes[i].CPU > resp.Processes[j].CPU
		case "mem":
			return resp.Processes[i].Memory > resp.Processes[j].Memory
		case "name":
			return resp.Processes[i].Name < resp.Processes[j].Name
		default:
			return resp.Processes[i].PID < resp.Processes[j].PID
		}
	})

	var rows []table.Row
	for _, p := range resp.Processes {
		rows = append(rows, table.Row{
			strconv.Itoa(int(p.PID)),
			p.Name,
			fmt.Sprintf("%.1f%%", p.CPU),
			fmt.Sprintf("%.1f%%", p.Memory),
		})
	}
	m.table.SetRows(rows)
}

func (m *model) sendCommand(action, value string) {
	conn, _ := net.Dial("tcp", m.selected)
	if conn != nil {
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Fatalf("Error al cerrar la conexión: %v", err)
			}
		}()
		err := json.NewEncoder(conn).Encode(protocol.CommandRequest{
			Action: action,
			Target: value,
		})
		if err != nil {
			log.Fatalf("Error al serializar respuesta: %v", err)
		}
	}
}

func (m *model) View() string {
	s := "Remote Process Monitor\n"
	m.mu.Lock()
	serverCount := len(m.servers)
	m.mu.Unlock()
	s += fmt.Sprintf("Ordenado por: %s | Servidores: %d | Seleccionado: %s\n\n", m.sortBy, serverCount, m.selected)
	s += baseStyle.Render(m.table.View()) + "\n"
	s += "\n[1-4] Ordenar (PID, Nom, CPU, Mem) | [s] Conectar | [k] Kill Process | [q] Quit\n"
	return s
}

func main() {
	columns := []table.Column{
		{Title: "PID", Width: 10},
		{Title: "Nombre", Width: 20},
		{Title: "CPU", Width: 10},
		{Title: "MEM", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	m := &model{
		table:   t,
		servers: make(map[string]string),
		mu:      &sync.Mutex{},
		sortBy:  "pid",
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error:", err)
	}
}
