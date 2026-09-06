package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SubmissionsStore manages thread-safe local persistence and retrieval of form submissions in a JSON file.
type SubmissionsStore struct {
	mu       sync.RWMutex
	filePath string
}

// NewSubmissionsStore creates a new SubmissionsStore targeting the specified file path.
func NewSubmissionsStore(filePath string) *SubmissionsStore {
	return &SubmissionsStore{
		filePath: filePath,
	}
}

// defaultStore is the default SubmissionsStore targeting submissions.json.
var defaultStore = NewSubmissionsStore("submissions.json")

// GetAll retrieves all stored form submissions from the JSON file.
// If the file does not exist yet or is empty, it returns an empty slice.
func (s *SubmissionsStore) GetAll() ([]FormSubmission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []FormSubmission{}, nil
		}
		return nil, fmt.Errorf("failed to read submissions file %s: %w", s.filePath, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return []FormSubmission{}, nil
	}

	var submissions []FormSubmission
	if err := json.Unmarshal(data, &submissions); err != nil {
		return nil, fmt.Errorf("failed to parse submissions from %s: %w", s.filePath, err)
	}

	if submissions == nil {
		submissions = []FormSubmission{}
	}

	return submissions, nil
}

// Save atomically appends a new form submission to the JSON file in a thread-safe manner.
// If the existing file contains corrupted JSON, it backs it up to preserve data and creates a fresh store.
func (s *SubmissionsStore) Save(sub FormSubmission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var submissions []FormSubmission

	// Read existing submissions if the file already exists
	if data, err := os.ReadFile(s.filePath); err == nil {
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &submissions); err != nil {
				// Existing file is corrupted. Back it up to preserve data and allow service to recover gracefully.
				backupPath := fmt.Sprintf("%s.corrupted-%d", s.filePath, time.Now().UnixNano())
				if renameErr := os.Rename(s.filePath, backupPath); renameErr == nil {
					log.Printf("Warning: existing submissions file %s was corrupted (%v); backed up to %s", s.filePath, err, backupPath)
				} else {
					return fmt.Errorf("failed to parse existing submissions from %s: %w", s.filePath, err)
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read submissions file %s: %w", s.filePath, err)
	}

	submissions = append(submissions, sub)

	formattedJSON, err := json.MarshalIndent(submissions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal submissions: %w", err)
	}
	formattedJSON = append(formattedJSON, '\n')

	dir := filepath.Dir(s.filePath)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create temporary file in the same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, "submissions-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(formattedJSON); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write submissions to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	_ = os.Chmod(tmpPath, 0644)

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("failed to atomically rename temp file to %s: %w", s.filePath, err)
	}
	tmpPath = "" // Success: temporary file successfully renamed

	return nil
}
