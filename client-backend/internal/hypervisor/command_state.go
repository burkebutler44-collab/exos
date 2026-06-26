package hypervisor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CommandStateStore interface {
	Get(commandID string) (StoredCommandResult, bool, error)
	Put(commandID string, result StoredCommandResult) error
}

type StoredCommandResult struct {
	CommandID   string    `json:"command_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

type FileCommandStateStore struct {
	Directory string
}

func NewFileCommandStateStore(directory string) *FileCommandStateStore {
	return &FileCommandStateStore{Directory: directory}
}

func (s *FileCommandStateStore) Get(commandID string) (StoredCommandResult, bool, error) {
	path, err := s.path(commandID)
	if err != nil {
		return StoredCommandResult{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return StoredCommandResult{}, false, nil
	}
	if err != nil {
		return StoredCommandResult{}, false, err
	}
	var result StoredCommandResult
	if err := json.Unmarshal(data, &result); err != nil {
		return StoredCommandResult{}, false, err
	}
	return result, true, nil
}

func (s *FileCommandStateStore) Put(commandID string, result StoredCommandResult) error {
	path, err := s.path(commandID)
	if err != nil {
		return err
	}
	if result.CommandID == "" {
		result.CommandID = commandID
	}
	if result.CompletedAt.IsZero() {
		result.CompletedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (s *FileCommandStateStore) path(commandID string) (string, error) {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" || strings.Contains(commandID, "/") || strings.Contains(commandID, "..") {
		return "", errors.New("invalid command id")
	}
	return filepath.Join(s.Directory, commandID+".json"), nil
}
