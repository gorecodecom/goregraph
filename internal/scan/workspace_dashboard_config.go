package scan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const WorkspaceDashboardConfigName = ".goregraph-dashboard.json"
const missingDashboardConfigRevision = "missing"

var ErrDashboardConfigConflict = errors.New("workspace dashboard configuration changed")

type WorkspaceDashboardConfig struct {
	Schema       int                         `json:"schema"`
	Architecture DashboardArchitectureConfig `json:"architecture"`
}

type DashboardArchitectureConfig struct {
	GroupOrder []string                          `json:"groupOrder,omitempty"`
	Groups     map[string]DashboardGroupConfig   `json:"groups,omitempty"`
	Services   map[string]DashboardServiceConfig `json:"services,omitempty"`
}

type DashboardGroupConfig struct {
	Label string `json:"label"`
}

type DashboardServiceConfig struct {
	Group string `json:"group"`
	Order int    `json:"order,omitempty"`
}

func LoadWorkspaceDashboardConfig(root string) (WorkspaceDashboardConfig, string, error) {
	data, err := os.ReadFile(filepath.Join(root, WorkspaceDashboardConfigName))
	if errors.Is(err, os.ErrNotExist) {
		return WorkspaceDashboardConfig{Schema: 1}, missingDashboardConfigRevision, nil
	}
	if err != nil {
		return WorkspaceDashboardConfig{}, "", err
	}

	var config WorkspaceDashboardConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return WorkspaceDashboardConfig{}, "", err
	}
	if err := requireDashboardConfigEOF(decoder); err != nil {
		return WorkspaceDashboardConfig{}, "", err
	}
	if err := ValidateWorkspaceDashboardConfig(config); err != nil {
		return WorkspaceDashboardConfig{}, "", err
	}
	return config, dashboardConfigRevision(data), nil
}

func SaveWorkspaceDashboardConfig(root, expectedRevision string, config WorkspaceDashboardConfig) (string, error) {
	if err := ValidateWorkspaceDashboardConfig(config); err != nil {
		return "", err
	}
	path := filepath.Join(root, WorkspaceDashboardConfigName)
	currentRevision, err := currentDashboardConfigRevision(path)
	if err != nil {
		return "", err
	}
	if expectedRevision != currentRevision {
		return "", ErrDashboardConfigConflict
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(root, ".goregraph-dashboard-*.tmp")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return "", err
	}
	return dashboardConfigRevision(data), nil
}

func ValidateWorkspaceDashboardConfig(config WorkspaceDashboardConfig) error {
	if config.Schema != 1 {
		return fmt.Errorf("unsupported workspace dashboard configuration schema %d", config.Schema)
	}
	for groupID, group := range config.Architecture.Groups {
		if strings.TrimSpace(groupID) == "" {
			return errors.New("workspace dashboard group ID must not be blank")
		}
		if strings.TrimSpace(group.Label) == "" {
			return fmt.Errorf("workspace dashboard group %q label must not be blank", groupID)
		}
	}
	seenGroups := make(map[string]bool, len(config.Architecture.GroupOrder))
	for _, groupID := range config.Architecture.GroupOrder {
		if strings.TrimSpace(groupID) == "" {
			return errors.New("workspace dashboard group ID must not be blank")
		}
		if seenGroups[groupID] {
			return fmt.Errorf("workspace dashboard group order contains duplicate %q", groupID)
		}
		seenGroups[groupID] = true
	}
	for serviceID, service := range config.Architecture.Services {
		if !validDashboardServiceID(serviceID) {
			return fmt.Errorf("workspace dashboard service ID %q must be workspace-relative", serviceID)
		}
		if _, ok := config.Architecture.Groups[service.Group]; !ok {
			return fmt.Errorf("workspace dashboard service %q references unknown group %q", serviceID, service.Group)
		}
	}
	return nil
}

func requireDashboardConfigEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("workspace dashboard configuration contains multiple JSON values")
		}
		return err
	}
	return nil
}

func currentDashboardConfigRevision(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return missingDashboardConfigRevision, nil
	}
	if err != nil {
		return "", err
	}
	return dashboardConfigRevision(data), nil
}

func dashboardConfigRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validDashboardServiceID(serviceID string) bool {
	if strings.TrimSpace(serviceID) == "" || filepath.IsAbs(serviceID) || path.IsAbs(serviceID) {
		return false
	}
	for _, segment := range strings.FieldsFunc(serviceID, func(r rune) bool { return r == '/' || r == '\\' }) {
		if segment == ".." {
			return false
		}
	}
	return true
}
