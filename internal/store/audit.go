package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type auditEvent struct {
	EventID    string         `json:"event_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Actor      string         `json:"actor"`
	Backend    string         `json:"backend"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type auditLogger struct {
	path string
	mu   sync.Mutex
}

func newAuditLogger(dataFile string) *auditLogger {
	return &auditLogger{path: dataFile + ".audit.log"}
}

func (a *auditLogger) Log(event auditEvent) error {
	if a == nil {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	event.EventID = newID("ev")
	event.OccurredAt = time.Now().UTC()

	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("no se pudo serializar evento de auditoria: %w", err)
	}

	dir := filepath.Dir(a.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("no se pudo crear directorio de auditoria %s: %w", dir, err)
	}

	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("no se pudo abrir archivo de auditoria %s: %w", a.path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	writer := bufio.NewWriter(f)
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("no se pudo escribir evento de auditoria: %w", err)
	}
	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("no se pudo escribir salto de linea de auditoria: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("no se pudo vaciar buffer de auditoria: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("no se pudo sincronizar archivo de auditoria: %w", err)
	}

	return nil
}
