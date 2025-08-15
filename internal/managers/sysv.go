package managers

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nucc.com/mcp_srv_mgr/pkg/types"
)

type SysVManager struct {
	initDPath string
}

func NewSysVManager() *SysVManager {
	return &SysVManager{
		initDPath: "/etc/init.d",
	}
}

func (sv *SysVManager) Start(serviceName string) error {
	scriptPath := filepath.Join(sv.initDPath, serviceName)
	if !sv.serviceExists(serviceName) {
		return fmt.Errorf("service %s not found", serviceName)
	}
	cmd := exec.Command(scriptPath, "start")
	return cmd.Run()
}

func (sv *SysVManager) Stop(serviceName string) error {
	scriptPath := filepath.Join(sv.initDPath, serviceName)
	if !sv.serviceExists(serviceName) {
		return fmt.Errorf("service %s not found", serviceName)
	}
	cmd := exec.Command(scriptPath, "stop")
	return cmd.Run()
}

func (sv *SysVManager) Restart(serviceName string) error {
	scriptPath := filepath.Join(sv.initDPath, serviceName)
	if !sv.serviceExists(serviceName) {
		return fmt.Errorf("service %s not found", serviceName)
	}
	cmd := exec.Command(scriptPath, "restart")
	return cmd.Run()
}

func (sv *SysVManager) Enable(serviceName string) error {
	if !sv.serviceExists(serviceName) {
		return fmt.Errorf("service %s not found", serviceName)
	}

	// Use chkconfig if available
	if sv.hasChkconfig() {
		cmd := exec.Command("chkconfig", serviceName, "on")
		return cmd.Run()
	}

	// Use update-rc.d if available (Debian/Ubuntu)
	if sv.hasUpdateRcd() {
		cmd := exec.Command("update-rc.d", serviceName, "enable")
		return cmd.Run()
	}

	return fmt.Errorf("no suitable enable method found")
}

func (sv *SysVManager) Disable(serviceName string) error {
	if !sv.serviceExists(serviceName) {
		return fmt.Errorf("service %s not found", serviceName)
	}

	// Use chkconfig if available
	if sv.hasChkconfig() {
		cmd := exec.Command("chkconfig", serviceName, "off")
		return cmd.Run()
	}

	// Use update-rc.d if available (Debian/Ubuntu)
	if sv.hasUpdateRcd() {
		cmd := exec.Command("update-rc.d", serviceName, "disable")
		return cmd.Run()
	}

	return fmt.Errorf("no suitable disable method found")
}

func (sv *SysVManager) GetStatus(serviceName string) (types.ServiceInfo, error) {
	info := types.ServiceInfo{
		Name: serviceName,
		Type: types.ServiceTypeSysV,
	}

	if !sv.serviceExists(serviceName) {
		return info, fmt.Errorf("service %s not found", serviceName)
	}

	scriptPath := filepath.Join(sv.initDPath, serviceName)

	// Try to get status from the service script
	cmd := exec.Command(scriptPath, "status")
	output, err := cmd.Output()
	if err != nil {
		// If status command fails, assume service is inactive
		info.Status = types.StatusInactive
	} else {
		statusOutput := strings.ToLower(string(output))
		if strings.Contains(statusOutput, "running") || strings.Contains(statusOutput, "started") {
			info.Status = types.StatusActive

			// Try to extract PID from status output
			if pid := sv.extractPIDFromStatus(string(output)); pid > 0 {
				info.PID = pid
				info.Uptime = sv.getProcessUptime(pid)
			}
		} else if strings.Contains(statusOutput, "stopped") || strings.Contains(statusOutput, "inactive") {
			info.Status = types.StatusInactive
		} else if strings.Contains(statusOutput, "failed") {
			info.Status = types.StatusFailed
		} else {
			info.Status = types.StatusUnknown
		}
	}

	// Try to get description from LSB header
	info.Description = sv.getServiceDescription(scriptPath)

	return info, nil
}

func (sv *SysVManager) ListServices() ([]types.ServiceInfo, error) {
	var services []types.ServiceInfo

	files, err := os.ReadDir(sv.initDPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		// Skip common non-service files
		if sv.isNonServiceFile(file.Name()) {
			continue
		}

		serviceName := file.Name()
		info, err := sv.GetStatus(serviceName)
		if err == nil {
			services = append(services, info)
		}
	}

	return services, nil
}

func (sv *SysVManager) serviceExists(serviceName string) bool {
	scriptPath := filepath.Join(sv.initDPath, serviceName)
	info, err := os.Stat(scriptPath)
	return err == nil && !info.IsDir()
}

func (sv *SysVManager) hasChkconfig() bool {
	cmd := exec.Command("which", "chkconfig")
	return cmd.Run() == nil
}

func (sv *SysVManager) hasUpdateRcd() bool {
	cmd := exec.Command("which", "update-rc.d")
	return cmd.Run() == nil
}

func (sv *SysVManager) extractPIDFromStatus(output string) int {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "pid") {
			// Look for patterns like "PID: 1234" or "pid 1234" or "(pid 1234)"
			words := strings.Fields(line)
			for i, word := range words {
				if strings.ToLower(word) == "pid" || strings.ToLower(word) == "pid:" {
					if i+1 < len(words) {
						if pid, err := strconv.Atoi(strings.Trim(words[i+1], "(),:")); err == nil {
							return pid
						}
					}
				}
				// Check for (pid pattern - could be "(pid" followed by number in next word
				if strings.ToLower(word) == "(pid" && i+1 < len(words) {
					nextWord := strings.TrimSuffix(words[i+1], ")")
					if pid, err := strconv.Atoi(nextWord); err == nil {
						return pid
					}
				}
				// Check for (pid XXXX) pattern in single word - rare but possible
				if strings.HasPrefix(word, "(pid") && strings.HasSuffix(word, ")") {
					// Extract number from "(pidXXXX)" - no space case
					pidStr := strings.TrimPrefix(word, "(pid")
					pidStr = strings.TrimSuffix(pidStr, ")")
					pidStr = strings.TrimSpace(pidStr)
					if pidStr != "" {
						if pid, err := strconv.Atoi(pidStr); err == nil {
							return pid
						}
					}
				}
			}
		}
	}
	return 0
}

func (sv *SysVManager) getProcessUptime(pid int) time.Duration {
	cmd := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	etimeStr := strings.TrimSpace(string(output))
	return sv.parseEtime(etimeStr)
}

func (sv *SysVManager) parseEtime(etime string) time.Duration {
	// etime format can be: "MM:SS", "HH:MM:SS", "DD-HH:MM:SS"
	parts := strings.Split(etime, "-")
	var days int
	var timeStr string

	if len(parts) == 2 {
		days, _ = strconv.Atoi(parts[0])
		timeStr = parts[1]
	} else {
		timeStr = etime
	}

	timeParts := strings.Split(timeStr, ":")
	var duration time.Duration

	if len(timeParts) == 2 {
		// MM:SS
		minutes, _ := strconv.Atoi(timeParts[0])
		seconds, _ := strconv.Atoi(timeParts[1])
		duration = time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	} else if len(timeParts) == 3 {
		// HH:MM:SS
		hours, _ := strconv.Atoi(timeParts[0])
		minutes, _ := strconv.Atoi(timeParts[1])
		seconds, _ := strconv.Atoi(timeParts[2])
		duration = time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	}

	if days > 0 {
		duration += time.Duration(days) * 24 * time.Hour
	}

	return duration
}

func (sv *SysVManager) getServiceDescription(scriptPath string) string {
	file, err := os.Open(scriptPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for LSB header description
		if strings.HasPrefix(line, "# Short-Description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# Short-Description:"))
		}
		if strings.HasPrefix(line, "# Description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# Description:"))
		}
	}

	return ""
}

func (sv *SysVManager) isNonServiceFile(filename string) bool {
	nonServiceFiles := []string{
		"README", "skeleton", "rcS", "rc", "functions", "halt", "killall",
		"single", "reboot", "bootmisc", "checkroot", "hostname", "keymap",
		"localmount", "mtab", "procfs", "urandom", "hwclock",
	}

	for _, nonService := range nonServiceFiles {
		if filename == nonService {
			return true
		}
	}

	return false
}

func IsSysVAvailable() bool {
	_, err := os.Stat("/etc/init.d")
	return err == nil
}