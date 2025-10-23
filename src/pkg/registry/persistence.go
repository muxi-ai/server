package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Persistence handles saving and loading the registry to/from disk
type Persistence struct {
	registry     *Registry
	filePath     string
	logger       *zerolog.Logger
	autoSave     bool
	saveDebounce time.Duration
	saveChan     chan struct{}
	stopChan     chan struct{}
	mu           sync.Mutex     // Protects autoSave flag
	wg           sync.WaitGroup // Tracks running save operations
	loopWg       sync.WaitGroup // Tracks the auto-save goroutine
}

// NewPersistence creates a new persistence manager
func NewPersistence(registry *Registry, filePath string, logger *zerolog.Logger) *Persistence {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	return &Persistence{
		registry:     registry,
		filePath:     filePath,
		logger:       logger,
		saveDebounce: 2 * time.Second, // Wait 2s after last change before saving
		saveChan:     make(chan struct{}, 1),
		stopChan:     make(chan struct{}),
	}
}

// EnableAutoSave enables automatic saving when the registry changes
func (p *Persistence) EnableAutoSave() {
	p.mu.Lock()
	if p.autoSave {
		p.mu.Unlock()
		return // Already enabled
	}
	p.autoSave = true
	p.mu.Unlock()
	
	// Wait for any previous goroutine to exit before recreating stopChan
	p.loopWg.Wait()
	
	// Recreate stop channel for this new cycle
	p.stopChan = make(chan struct{})

	// Set up onChange callback
	p.registry.OnChange(func() {
		select {
		case p.saveChan <- struct{}{}:
		default:
			// Channel full, save already pending
		}
	})

	// Start auto-save goroutine
	p.loopWg.Add(1)
	go p.autoSaveLoop()
}

// DisableAutoSave disables automatic saving
func (p *Persistence) DisableAutoSave() {
	p.mu.Lock()
	if !p.autoSave {
		p.mu.Unlock()
		return // Already disabled
	}
	p.autoSave = false
	p.mu.Unlock()
	
	// Close stop channel to signal loop to exit
	close(p.stopChan)
	
	// Wait for the goroutine to exit
	p.loopWg.Wait()
	
	// Wait for all pending save operations to complete
	p.wg.Wait()
}

// Save saves the registry to disk
func (p *Persistence) Save() error {
	// Get all formations
	formations := p.registry.List()

	// Create registry snapshot
	snapshot := &RegistrySnapshot{
		Version:    1,
		SavedAt:    time.Now(),
		Formations: formations,
	}

	// Ensure directory exists
	dir := filepath.Dir(p.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	// Write to file (atomic write with temp file)
	tempFile := p.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename (atomic on most filesystems)
	if err := os.Rename(tempFile, p.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	p.logger.Debug().
		Str("path", p.filePath).
		Int("formations", len(formations)).
		Msg("Registry saved")

	return nil
}

// Load loads the registry from disk
func (p *Persistence) Load() error {
	// Check if file exists
	if _, err := os.Stat(p.filePath); os.IsNotExist(err) {
		p.logger.Debug().
			Str("path", p.filePath).
			Msg("Registry file does not exist (starting fresh)")
		return nil
	}

	// Read file
	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	// Unmarshal
	var snapshot RegistrySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal registry: %w", err)
	}

	// Validate version
	if snapshot.Version != 1 {
		return fmt.Errorf("unsupported registry version: %d", snapshot.Version)
	}

	// Register all formations
	for _, formation := range snapshot.Formations {
		if err := p.registry.Register(formation); err != nil {
			p.logger.Warn().
				Err(err).
				Str("formation_id", formation.ID).
				Msg("Failed to register formation from snapshot")
			continue
		}
	}

	// Registry loaded silently (formations count logged in main if needed)

	return nil
}

// autoSaveLoop runs the auto-save loop with debouncing
func (p *Persistence) autoSaveLoop() {
	defer p.loopWg.Done()
	
	var timer *time.Timer
	var timerMu sync.Mutex // Protects timer access

	for {
		select {
		case <-p.saveChan:
			// Reset timer (debounce)
			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(p.saveDebounce, func() {
				p.mu.Lock()
				if !p.autoSave {
					// Auto-save was disabled, don't save
					p.mu.Unlock()
					return
				}
				p.mu.Unlock()
				
				// Track this save operation
				p.wg.Add(1)
				defer p.wg.Done()
				
				if err := p.Save(); err != nil {
					p.logger.Error().
						Err(err).
						Msg("Failed to auto-save registry")
				}
			})
			timerMu.Unlock()

		case <-p.stopChan:
			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timerMu.Unlock()
			return
		}
	}
}

// RegistrySnapshot represents a saved registry state
type RegistrySnapshot struct {
	Version    int          `json:"version"`
	SavedAt    time.Time    `json:"saved_at"`
	Formations []*Formation `json:"formations"`
}
