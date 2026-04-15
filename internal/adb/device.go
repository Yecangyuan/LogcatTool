package adb

import (
	"bufio"
	"fmt"
	"os/exec"
	"sort"
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

	// Try modern format first: ps -A -o PID,NAME
	result := tryPsModern(adbPath, args)
	if len(result) > 0 {
		return result, nil
	}

	// Fallback for older Android: ps (full format)
	return tryPsLegacy(adbPath, args)
}

// tryPsModern tries `ps -A -o PID,NAME` (Android 8+).
func tryPsModern(adbPath string, baseArgs []string) map[string][]int {
	args := append(append([]string{}, baseArgs...), "shell", "ps", "-A", "-o", "PID,NAME")
	out, err := exec.Command(adbPath, args...).Output()
	if err != nil {
		return nil
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
	return result
}

// tryPsLegacy parses `ps` output (older Android, 8+ columns: USER PID PPID ... NAME).
func tryPsLegacy(adbPath string, baseArgs []string) (map[string][]int, error) {
	args := append(append([]string{}, baseArgs...), "shell", "ps")
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
		// PID is typically the 2nd column (index 1)
		var pid int
		if _, err := fmt.Sscanf(fields[1], "%d", &pid); err != nil {
			continue
		}
		// NAME is the last column
		name := fields[len(fields)-1]
		result[name] = append(result[name], pid)
	}
	return result, nil
}

// ListPackages returns installed package names from the device via pm list packages.
func ListPackages(adbPath, serial string) ([]string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "pm", "list", "packages")

	out, err := exec.Command(adbPath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("adb shell pm list packages 失败: %w", err)
	}

	var packages []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		pkg := strings.TrimPrefix(line, "package:")
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			packages = append(packages, pkg)
		}
	}

	sort.Strings(packages)
	return packages, nil
}
