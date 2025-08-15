package managers

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

type SystemdManager struct{}

func NewSystemdManager() *SystemdManager {
	return &SystemdManager{}
}

func (sm *SystemdManager) Start(serviceName string) error {
	cmd := exec.Command("systemctl", "start", serviceName)
	return cmd.Run()
}

func (sm *SystemdManager) Stop(serviceName string) error {
	cmd := exec.Command("systemctl", "stop", serviceName)
	return cmd.Run()
}

func (sm *SystemdManager) Restart(serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	return cmd.Run()
}

func (sm *SystemdManager) Enable(serviceName string) error {
	cmd := exec.Command("systemctl", "enable", serviceName)
	return cmd.Run()
}

func (sm *SystemdManager) Disable(serviceName string) error {
	cmd := exec.Command("systemctl", "disable", serviceName)
	return cmd.Run()
}

func (sm *SystemdManager) GetStatus(serviceName string) (types.ServiceInfo, error) {
	info := types.ServiceInfo{
		Name: serviceName,
		Type: types.ServiceTypeSystemd,
	}

	// Get basic status
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		info.Status = types.StatusFailed
	} else {
		status := strings.TrimSpace(string(output))
		switch status {
		case "active":
			info.Status = types.StatusActive
		case "inactive":
			info.Status = types.StatusInactive
		case "failed":
			info.Status = types.StatusFailed
		default:
			info.Status = types.StatusUnknown
		}
	}

	// Get detailed information
	cmd = exec.Command("systemctl", "show", serviceName, "--property=MainPID,Description,ActiveEnterTimestamp")
	output, err = cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, value := parts[0], parts[1]

			switch key {
			case "MainPID":
				if pid, err := strconv.Atoi(value); err == nil && pid > 0 {
					info.PID = pid
				}
			case "Description":
				info.Description = value
			case "ActiveEnterTimestamp":
				if value != "" && value != "n/a" {
					if startTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", value); err == nil {
						info.LastChanged = startTime
						info.Uptime = time.Since(startTime)
					}
				}
			}
		}
	}

	return info, nil
}

func (sm *SystemdManager) ListServices() ([]types.ServiceInfo, error) {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-pager", "--plain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var services []types.ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ".service") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				serviceName := strings.TrimSuffix(fields[0], ".service")
				var status types.ServiceStatus
				switch fields[3] {
				case "running":
					status = types.StatusActive
				case "dead", "exited":
					status = types.StatusInactive
				case "failed":
					status = types.StatusFailed
				default:
					status = types.StatusUnknown
				}

				services = append(services, types.ServiceInfo{
					Name:   serviceName,
					Type:   types.ServiceTypeSystemd,
					Status: status,
				})
			}
		}
	}

	return services, nil
}

func IsSystemdAvailable() bool {
	cmd := exec.Command("systemctl", "--version")
	return cmd.Run() == nil
}