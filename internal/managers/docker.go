package managers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

type DockerManager struct{}

type DockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created int64             `json:"Created"`
	Labels  map[string]string `json:"Labels"`
}

func NewDockerManager() *DockerManager {
	return &DockerManager{}
}

func (dm *DockerManager) Start(containerName string) error {
	cmd := exec.Command("docker", "start", containerName)
	return cmd.Run()
}

func (dm *DockerManager) Stop(containerName string) error {
	cmd := exec.Command("docker", "stop", containerName)
	return cmd.Run()
}

func (dm *DockerManager) Restart(containerName string) error {
	cmd := exec.Command("docker", "restart", containerName)
	return cmd.Run()
}

func (dm *DockerManager) Enable(containerName string) error {
	// For Docker, "enable" means setting restart policy to always
	cmd := exec.Command("docker", "update", "--restart=always", containerName)
	return cmd.Run()
}

func (dm *DockerManager) Disable(containerName string) error {
	// For Docker, "disable" means setting restart policy to no
	cmd := exec.Command("docker", "update", "--restart=no", containerName)
	return cmd.Run()
}

func (dm *DockerManager) GetStatus(containerName string) (types.ServiceInfo, error) {
	info := types.ServiceInfo{
		Name: containerName,
		Type: types.ServiceTypeDocker,
	}

	// Get container information
	cmd := exec.Command("docker", "inspect", containerName)
	output, err := cmd.Output()
	if err != nil {
		return info, fmt.Errorf("container %s not found", containerName)
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(output, &containers); err != nil {
		return info, err
	}

	if len(containers) == 0 {
		return info, fmt.Errorf("container %s not found", containerName)
	}

	container := containers[0]

	// Get state information
	if state, ok := container["State"].(map[string]interface{}); ok {
		if running, ok := state["Running"].(bool); ok {
			if running {
				info.Status = types.StatusActive
			} else {
				info.Status = types.StatusInactive
			}
		}

		// Get PID
		if pid, ok := state["Pid"].(float64); ok && pid > 0 {
			info.PID = int(pid)
		}

		// Get start time
		if startedAt, ok := state["StartedAt"].(string); ok && startedAt != "" {
			if startTime, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
				info.LastChanged = startTime
				if info.Status == types.StatusActive {
					info.Uptime = time.Since(startTime)
				}
			}
		}
	}

	// Get config information for description
	if config, ok := container["Config"].(map[string]interface{}); ok {
		if image, ok := config["Image"].(string); ok {
			info.Description = fmt.Sprintf("Docker container from image: %s", image)
		}
	}

	return info, nil
}

func (dm *DockerManager) ListServices() ([]types.ServiceInfo, error) {
	cmd := exec.Command("docker", "ps", "-a", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var services []types.ServiceInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var container DockerContainer
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		var name string
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		} else {
			name = container.ID[:12]
		}

		var status types.ServiceStatus
		switch container.State {
		case "running":
			status = types.StatusActive
		case "exited", "created":
			status = types.StatusInactive
		case "dead", "restarting":
			status = types.StatusFailed
		default:
			status = types.StatusUnknown
		}

		// Parse creation time
		var lastChanged time.Time
		var uptime time.Duration
		if container.Created > 0 {
			lastChanged = time.Unix(container.Created, 0)
			if status == types.StatusActive {
				uptime = time.Since(lastChanged)
			}
		}

		services = append(services, types.ServiceInfo{
			Name:        name,
			Type:        types.ServiceTypeDocker,
			Status:      status,
			Description: fmt.Sprintf("Docker container from image: %s", container.Image),
			LastChanged: lastChanged,
			Uptime:      uptime,
		})
	}

	return services, nil
}

// Additional Docker-specific methods

func (dm *DockerManager) GetLogs(containerName string, lines int) (string, error) {
	args := []string{"logs"}
	if lines > 0 {
		args = append(args, "--tail", strconv.Itoa(lines))
	}
	args = append(args, containerName)

	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	return string(output), err
}

func (dm *DockerManager) GetStats(containerName string) (map[string]interface{}, error) {
	cmd := exec.Command("docker", "stats", containerName, "--no-stream", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(output, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

func (dm *DockerManager) RemoveContainer(containerName string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerName)

	cmd := exec.Command("docker", args...)
	return cmd.Run()
}

func (dm *DockerManager) CreateContainer(imageName, containerName string, options []string) error {
	args := []string{"run", "-d"}
	if containerName != "" {
		args = append(args, "--name", containerName)
	}
	args = append(args, options...)
	args = append(args, imageName)

	cmd := exec.Command("docker", args...)
	return cmd.Run()
}

func IsDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}