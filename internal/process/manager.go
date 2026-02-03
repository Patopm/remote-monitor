// Package process: Service manager for processes
package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/Patopm/remote-monitor/internal/protocol"

	ps "github.com/shirou/gopsutil/v3/process"
)

func ListProcesses() ([]protocol.ProcessInfo, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, err
	}

	var list []protocol.ProcessInfo
	for _, p := range processes {
		name, _ := p.Name()
		cpu, _ := p.CPUPercent()
		mem, _ := p.MemoryPercent()

		list = append(list, protocol.ProcessInfo{
			PID:    p.Pid,
			Name:   name,
			CPU:    cpu,
			Memory: mem,
		})
	}
	return list, nil
}

func StopProcess(pidStr string) error {
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		return fmt.Errorf("PID inválido: %v", err)
	}

	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}

	return p.Kill()
}

func StartProcess(path string) error {
	cmd := exec.Command(path)
	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("El proceso %s terminó con error: %v\n", path, err)
		}
	}()
	return nil
}
