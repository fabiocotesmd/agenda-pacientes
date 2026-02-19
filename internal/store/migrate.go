package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"agenda-pacientes/internal/model"
)

type MigrationOptions struct {
	FromJSON       string
	ToSQLite       string
	Actor          string
	ForceOverwrite bool
}

type MigrationResult struct {
	Patients     int
	Appointments int
}

func MigrateJSONToSQLite(opts MigrationOptions) (MigrationResult, error) {
	fromJSON := strings.TrimSpace(opts.FromJSON)
	toSQLite := strings.TrimSpace(opts.ToSQLite)
	actor := strings.TrimSpace(opts.Actor)

	if fromJSON == "" {
		return MigrationResult{}, errors.New("from-json es obligatorio")
	}
	if toSQLite == "" {
		return MigrationResult{}, errors.New("to-sqlite es obligatorio")
	}
	if actor == "" {
		return MigrationResult{}, errors.New("actor es obligatorio para migracion")
	}

	data, err := loadJSONDataFile(fromJSON)
	if err != nil {
		return MigrationResult{}, err
	}

	for _, appt := range data.Appointments {
		if _, err := validateStatus(appt.Status); err != nil {
			return MigrationResult{}, fmt.Errorf("estado invalido en cita %q: %w", appt.ID, err)
		}
	}

	db, err := openSQLiteDB(toSQLite)
	if err != nil {
		return MigrationResult{}, err
	}
	defer func() {
		_ = db.Close()
	}()

	if err := ensureSQLiteSchema(db); err != nil {
		return MigrationResult{}, err
	}

	if opts.ForceOverwrite {
		if err := sqliteResetSchema(db); err != nil {
			return MigrationResult{}, err
		}
	} else {
		hasData, err := sqliteHasAnyData(db)
		if err != nil {
			return MigrationResult{}, err
		}
		if hasData {
			return MigrationResult{}, errors.New("el destino sqlite no esta vacio; usa --force-overwrite para sobrescribir")
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return MigrationResult{}, fmt.Errorf("no se pudo iniciar transaccion de migracion: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, patient := range data.Patients {
		_, err := tx.Exec(
			`INSERT INTO patients(id, name, phone, email, email_norm, phone_norm, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			patient.ID,
			patient.Name,
			patient.Phone,
			patient.Email,
			normalizeEmail(patient.Email),
			normalizePhoneDigits(patient.Phone),
			formatStoredTime(patient.CreatedAt),
		)
		if err != nil {
			return MigrationResult{}, fmt.Errorf("no se pudo migrar paciente %q: %w", patient.ID, err)
		}
	}

	for _, appt := range data.Appointments {
		normalizedStatus, _ := validateStatus(appt.Status)
		_, err := tx.Exec(
			`INSERT INTO appointments(id, patient_id, date_time, reason, status, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
			appt.ID,
			appt.PatientID,
			formatAppointmentTime(appt.DateTime),
			appt.Reason,
			normalizedStatus,
			formatStoredTime(appt.CreatedAt),
		)
		if err != nil {
			return MigrationResult{}, fmt.Errorf("no se pudo migrar cita %q: %w", appt.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return MigrationResult{}, fmt.Errorf("no se pudo confirmar migracion: %w", err)
	}

	result := MigrationResult{
		Patients:     len(data.Patients),
		Appointments: len(data.Appointments),
	}

	logger := newAuditLogger(toSQLite)
	if err := logger.Log(auditEvent{
		Actor:      actor,
		Backend:    "sqlite",
		Action:     "storage.migrate",
		EntityType: "storage",
		EntityID:   toSQLite,
		Metadata: map[string]any{
			"from_json":    fromJSON,
			"patients":     result.Patients,
			"appointments": result.Appointments,
		},
	}); err != nil {
		return MigrationResult{}, err
	}

	return result, nil
}

func loadJSONDataFile(path string) (model.Data, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return model.Data{}, fmt.Errorf("no se pudo leer %s: %w", path, err)
	}

	if len(strings.TrimSpace(string(bytes))) == 0 {
		return model.Data{}, nil
	}

	var data model.Data
	if err := json.Unmarshal(bytes, &data); err != nil {
		return model.Data{}, fmt.Errorf("JSON invalido en %s: %w", path, err)
	}
	return data, nil
}
