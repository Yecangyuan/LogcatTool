package adb

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

type Device struct {
	Serial string
	Model  string
	State  string
}

func (d Device) DisplayName() string {
	if d.Model != "" {
		return fmt.Sprintf("%s (%s)", d.Serial, d.Model)
	}
	return d.Serial
}

func FindADB() (string, error) {
	path, err := exec.LookPath("adb")
	if err != nil {
		return "", fmt.Errorf("未找到 adb: %w", err)
	}
	return path, nil
}

func ListDevices(adbPath string) ([]Device, error) {
	out, err := exec.Command(adbPath, "devices", "-l").Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices 失败: %w", err)
	}

	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "List of") || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		d := Device{
			Serial: parts[0],
			State:  parts[1],
		}
		for _, p := range parts[2:] {
			if strings.HasPrefix(p, "model:") {
				d.Model = strings.TrimPrefix(p, "model:")
			}
		}
		if d.State == "device" {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func GetPackagePIDs(adbPath, serial string) (map[string][]int, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "ps", "-A", "-o", "PID,NAME")

	out, err := exec.Command(adbPath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("adb shell ps 失败: %w", err)
	}

	result := make(map[string][]int)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(fields[0], "%d", &pid); err != nil {
			continue
		}
		name := fields[1]
		result[name] = append(result[name], pid)
	}
	return result, nil
}
