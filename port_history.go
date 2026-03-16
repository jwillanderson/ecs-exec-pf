package ecsexecpf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PortMapping struct {
	RemotePort int `json:"remote_port"`
	LocalPort  int `json:"local_port"`
}

type PortHistory struct {
	Services map[string][][]PortMapping `json:"services"`
}

func (pm PortMapping) String() string {
	return fmt.Sprintf("%d:%d", pm.LocalPort, pm.RemotePort)
}

func FormatMappings(mappings []PortMapping) string {
	s := ""
	for i, pm := range mappings {
		if i > 0 {
			s += ", "
		}
		s += pm.String()
	}
	return s
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "ecs-exec-pf")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return dir, nil
}

func historyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

func LoadPortHistory() (PortHistory, error) {
	path, err := historyPath()
	if err != nil {
		return PortHistory{Services: make(map[string][][]PortMapping)}, err
	}
	return loadPortHistoryFromPath(path)
}

func loadPortHistoryFromPath(path string) (PortHistory, error) {
	empty := PortHistory{Services: make(map[string][][]PortMapping)}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("failed to read history file: %w", err)
	}

	var history PortHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return empty, nil
	}
	if history.Services == nil {
		history.Services = make(map[string][][]PortMapping)
	}
	return history, nil
}

func SavePortHistory(history PortHistory) error {
	path, err := historyPath()
	if err != nil {
		return err
	}
	return savePortHistoryToPath(path, history)
}

func savePortHistoryToPath(path string, history PortHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename history file: %w", err)
	}
	return nil
}

func AddMapping(history PortHistory, service string, mappings []PortMapping) PortHistory {
	newServices := make(map[string][][]PortMapping, len(history.Services))
	for k, v := range history.Services {
		newServices[k] = v
	}

	existing := newServices[service]
	filtered := [][]PortMapping{mappings}
	for _, entry := range existing {
		if !mappingsEqual(entry, mappings) {
			filtered = append(filtered, entry)
		}
	}

	const maxEntries = 10
	if len(filtered) > maxEntries {
		filtered = filtered[:maxEntries]
	}

	newServices[service] = filtered
	return PortHistory{Services: newServices}
}

func mappingsEqual(a, b []PortMapping) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
