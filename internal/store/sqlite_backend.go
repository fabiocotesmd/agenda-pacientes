package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agenda-pacientes/internal/model"
	_ "modernc.org/sqlite"
)

type sqliteBackend struct {
	path string
	db   *sql.DB
	mu   sync.Mutex
}

func newSQLiteBackend(path string) (backend, error) {
	db, err := openSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	if err := ensureSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &sqliteBackend{
		path: path,
		db:   db,
	}, nil
}

func openSQLiteDB(path string) (*sql.DB, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, errors.New("ruta sqlite invalida")
	}

	dir := filepath.Dir(trimmedPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("no se pudo crear directorio sqlite %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir sqlite %s: %w", trimmedPath, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("no se pudo conectar sqlite %s: %w", trimmedPath, err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("no se pudo activar foreign_keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("no se pudo activar WAL: %w", err)
	}

	return db, nil
}

func ensureSQLiteSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS patients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT NOT NULL,
			email TEXT NOT NULL,
			email_norm TEXT NOT NULL,
			phone_norm TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS appointments (
			id TEXT PRIMARY KEY,
			patient_id TEXT NOT NULL,
			date_time TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(patient_id) REFERENCES patients(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_date_time ON appointments(date_time);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_patient_id ON appointments(patient_id);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_status ON appointments(status);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patients_email_norm_unique ON patients(email_norm) WHERE email_norm <> '';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patients_phone_norm_unique ON patients(phone_norm) WHERE phone_norm <> '';`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("no se pudo inicializar esquema sqlite: %w", err)
		}
	}

	return nil
}

func sqliteHasAnyData(db *sql.DB) (bool, error) {
	var total int
	err := db.QueryRow(`SELECT (SELECT COUNT(1) FROM patients) + (SELECT COUNT(1) FROM appointments)`).Scan(&total)
	if err != nil {
		return false, fmt.Errorf("no se pudo consultar contenido sqlite: %w", err)
	}
	return total > 0, nil
}

func sqliteResetSchema(db *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS appointments;`,
		`DROP TABLE IF EXISTS patients;`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("no se pudo reiniciar esquema sqlite: %w", err)
		}
	}

	return ensureSQLiteSchema(db)
}

func (b *sqliteBackend) AddPatient(name, phone, email string) (model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return model.Patient{}, errors.New("el nombre es obligatorio")
	}

	trimmedPhone := strings.TrimSpace(phone)
	trimmedEmail := strings.TrimSpace(email)
	normPhone := normalizePhoneDigits(trimmedPhone)
	normEmail := normalizeEmail(trimmedEmail)

	tx, err := b.db.Begin()
	if err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	dupID, err := findDuplicatePatientSQLite(tx, normEmail, normPhone, "")
	if err != nil {
		return model.Patient{}, err
	}
	if dupID != "" {
		return model.Patient{}, fmt.Errorf("paciente duplicado: coincide con %q", dupID)
	}

	patient := model.Patient{
		ID:        newID("p"),
		Name:      trimmedName,
		Phone:     trimmedPhone,
		Email:     trimmedEmail,
		CreatedAt: time.Now().UTC(),
	}

	_, err = tx.Exec(
		`INSERT INTO patients(id, name, phone, email, email_norm, phone_norm, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		patient.ID,
		patient.Name,
		patient.Phone,
		patient.Email,
		normEmail,
		normPhone,
		formatStoredTime(patient.CreatedAt),
	)
	if err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo crear paciente: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo confirmar paciente: %w", err)
	}

	return patient, nil
}

func (b *sqliteBackend) ListPatients() ([]model.Patient, error) {
	rows, err := b.db.Query(`SELECT id, name, phone, email, created_at FROM patients ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar pacientes: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	patients := []model.Patient{}
	for rows.Next() {
		var patient model.Patient
		var createdAt string
		if err := rows.Scan(&patient.ID, &patient.Name, &patient.Phone, &patient.Email, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer paciente: %w", err)
		}
		patient.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		patients = append(patients, patient)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar pacientes: %w", err)
	}

	return patients, nil
}

func (b *sqliteBackend) GetPatientByID(id string) (model.Patient, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return model.Patient{}, errors.New("id de paciente obligatorio")
	}

	var patient model.Patient
	var createdAt string
	err := b.db.QueryRow(
		`SELECT id, name, phone, email, created_at FROM patients WHERE id = ?`,
		target,
	).Scan(&patient.ID, &patient.Name, &patient.Phone, &patient.Email, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Patient{}, fmt.Errorf("no se encontro el paciente con id %q", target)
		}
		return model.Patient{}, fmt.Errorf("no se pudo obtener paciente %q: %w", target, err)
	}

	patient.CreatedAt, err = parseStoredTime(createdAt)
	if err != nil {
		return model.Patient{}, err
	}

	return patient, nil
}

func (b *sqliteBackend) UpdatePatient(id, name, phone, email string) (model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Patient{}, errors.New("id de paciente obligatorio")
	}

	trimmedName := strings.TrimSpace(name)
	trimmedPhone := strings.TrimSpace(phone)
	trimmedEmail := strings.TrimSpace(email)

	if trimmedName == "" && trimmedPhone == "" && trimmedEmail == "" {
		return model.Patient{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	current, err := b.GetPatientByID(target)
	if err != nil {
		return model.Patient{}, err
	}

	updated := current
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedPhone != "" {
		updated.Phone = trimmedPhone
	}
	if trimmedEmail != "" {
		updated.Email = trimmedEmail
	}

	normPhone := normalizePhoneDigits(updated.Phone)
	normEmail := normalizeEmail(updated.Email)

	tx, err := b.db.Begin()
	if err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	dupID, err := findDuplicatePatientSQLite(tx, normEmail, normPhone, updated.ID)
	if err != nil {
		return model.Patient{}, err
	}
	if dupID != "" {
		return model.Patient{}, fmt.Errorf("paciente duplicado: coincide con %q", dupID)
	}

	_, err = tx.Exec(
		`UPDATE patients SET name = ?, phone = ?, email = ?, email_norm = ?, phone_norm = ? WHERE id = ?`,
		updated.Name,
		updated.Phone,
		updated.Email,
		normEmail,
		normPhone,
		updated.ID,
	)
	if err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo actualizar paciente %q: %w", updated.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return model.Patient{}, fmt.Errorf("no se pudo confirmar actualizacion de paciente: %w", err)
	}

	return updated, nil
}

func (b *sqliteBackend) DeletePatient(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de paciente obligatorio")
	}

	if _, err := b.GetPatientByID(target); err != nil {
		return err
	}

	var activeCount int
	err := b.db.QueryRow(
		`SELECT COUNT(1) FROM appointments WHERE patient_id = ? AND status IN (?, ?)`,
		target,
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
	).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("no se pudo validar citas activas del paciente %q: %w", target, err)
	}
	if activeCount > 0 {
		return fmt.Errorf("no se puede eliminar el paciente %q porque tiene citas activas", target)
	}

	if _, err := b.db.Exec(`DELETE FROM patients WHERE id = ?`, target); err != nil {
		return fmt.Errorf("no se pudo eliminar paciente %q: %w", target, err)
	}
	return nil
}

func (b *sqliteBackend) SearchPatients(query string) ([]model.Patient, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	qPhone := normalizePhoneDigits(query)

	var rows *sql.Rows
	var err error

	if q == "" {
		rows, err = b.db.Query(`SELECT id, name, phone, email, created_at FROM patients ORDER BY created_at ASC`)
	} else {
		likeValue := "%" + q + "%"
		likePhone := "%" + qPhone + "%"
		rows, err = b.db.Query(
			`SELECT id, name, phone, email, created_at
			 FROM patients
			 WHERE lower(name) LIKE ? OR lower(email) LIKE ? OR (? <> '' AND phone_norm LIKE ?)
			 ORDER BY created_at ASC`,
			likeValue,
			likeValue,
			qPhone,
			likePhone,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar pacientes: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	patients := []model.Patient{}
	for rows.Next() {
		var patient model.Patient
		var createdAt string
		if err := rows.Scan(&patient.ID, &patient.Name, &patient.Phone, &patient.Email, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer paciente de busqueda: %w", err)
		}
		patient.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		patients = append(patients, patient)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar pacientes de busqueda: %w", err)
	}

	return patients, nil
}

func (b *sqliteBackend) ScheduleAppointment(patientID string, at time.Time, reason string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	trimmedPatientID := strings.TrimSpace(patientID)
	if trimmedPatientID == "" {
		return model.Appointment{}, errors.New("patient-id es obligatorio")
	}

	trimmedReason, err := validateRequiredReason(reason)
	if err != nil {
		return model.Appointment{}, err
	}

	normalizedDateTime, err := normalizeAppointmentDateTime(at)
	if err != nil {
		return model.Appointment{}, err
	}

	tx, err := b.db.Begin()
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var patientExists int
	err = tx.QueryRow(`SELECT COUNT(1) FROM patients WHERE id = ?`, trimmedPatientID).Scan(&patientExists)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo verificar paciente %q: %w", trimmedPatientID, err)
	}
	if patientExists == 0 {
		return model.Appointment{}, fmt.Errorf("no existe el paciente con id %q", trimmedPatientID)
	}

	var conflict int
	err = tx.QueryRow(
		`SELECT COUNT(1) FROM appointments WHERE date_time = ? AND status IN (?, ?)`,
		formatAppointmentTime(normalizedDateTime),
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
	).Scan(&conflict)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo verificar conflicto de citas: %w", err)
	}
	if conflict > 0 {
		return model.Appointment{}, errors.New("ya existe una cita en ese horario")
	}

	appointment := model.Appointment{
		ID:        newID("a"),
		PatientID: trimmedPatientID,
		DateTime:  normalizedDateTime,
		Reason:    trimmedReason,
		Status:    model.AppointmentStatusScheduled,
		CreatedAt: time.Now().UTC(),
	}

	_, err = tx.Exec(
		`INSERT INTO appointments(id, patient_id, date_time, reason, status, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		appointment.ID,
		appointment.PatientID,
		formatAppointmentTime(appointment.DateTime),
		appointment.Reason,
		appointment.Status,
		formatStoredTime(appointment.CreatedAt),
	)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo crear cita: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo confirmar cita: %w", err)
	}

	return appointment, nil
}

func (b *sqliteBackend) RescheduleAppointment(id string, at time.Time) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Appointment{}, errors.New("id de cita obligatorio")
	}

	normalizedDateTime, err := normalizeAppointmentDateTime(at)
	if err != nil {
		return model.Appointment{}, err
	}

	tx, err := b.db.Begin()
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	appointment, err := getAppointmentByIDSQLite(tx, target)
	if err != nil {
		return model.Appointment{}, err
	}

	if !canRescheduleStatus(appointment.Status) {
		return model.Appointment{}, fmt.Errorf("no se puede reprogramar una cita en estado %q", appointment.Status)
	}

	var conflict int
	err = tx.QueryRow(
		`SELECT COUNT(1) FROM appointments WHERE id <> ? AND date_time = ? AND status IN (?, ?)`,
		target,
		formatAppointmentTime(normalizedDateTime),
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
	).Scan(&conflict)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo verificar conflicto al reprogramar: %w", err)
	}
	if conflict > 0 {
		return model.Appointment{}, errors.New("ya existe una cita en ese horario")
	}

	_, err = tx.Exec(`UPDATE appointments SET date_time = ? WHERE id = ?`, formatAppointmentTime(normalizedDateTime), target)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo reprogramar cita %q: %w", target, err)
	}

	if err := tx.Commit(); err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo confirmar reprogramacion: %w", err)
	}

	appointment.DateTime = normalizedDateTime
	return appointment, nil
}

func (b *sqliteBackend) ListAppointments(filters AppointmentFilters) ([]model.Appointment, error) {
	query := `SELECT id, patient_id, date_time, reason, status, created_at FROM appointments WHERE 1=1`
	args := make([]any, 0, 8)

	if !filters.IncludeCanceled {
		query += ` AND status <> ?`
		args = append(args, model.AppointmentStatusCanceled)
	}

	trimmedPatientID := strings.TrimSpace(filters.PatientID)
	if trimmedPatientID != "" {
		query += ` AND patient_id = ?`
		args = append(args, trimmedPatientID)
	}

	trimmedStatus := strings.ToLower(strings.TrimSpace(filters.Status))
	if trimmedStatus != "" {
		validatedStatus, err := validateStatus(trimmedStatus)
		if err != nil {
			return nil, err
		}
		query += ` AND status = ?`
		args = append(args, validatedStatus)
	}

	if filters.From != nil {
		query += ` AND date_time >= ?`
		args = append(args, formatAppointmentTime(filters.From.UTC().Truncate(time.Minute)))
	}
	if filters.To != nil {
		query += ` AND date_time <= ?`
		args = append(args, formatAppointmentTime(filters.To.UTC().Truncate(time.Minute)))
	}

	query += ` ORDER BY date_time ASC`

	rows, err := b.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar citas: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	appointments := []model.Appointment{}
	for rows.Next() {
		var appointment model.Appointment
		var dateTime string
		var createdAt string

		if err := rows.Scan(
			&appointment.ID,
			&appointment.PatientID,
			&dateTime,
			&appointment.Reason,
			&appointment.Status,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("no se pudo leer cita: %w", err)
		}

		appointment.DateTime, err = parseStoredTime(dateTime)
		if err != nil {
			return nil, err
		}
		appointment.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}

		appointments = append(appointments, appointment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar citas: %w", err)
	}

	return appointments, nil
}

func (b *sqliteBackend) SetAppointmentStatus(id, status string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	targetID := strings.TrimSpace(id)
	if targetID == "" {
		return model.Appointment{}, errors.New("id de cita obligatorio")
	}

	targetStatus, err := validateStatus(status)
	if err != nil {
		return model.Appointment{}, err
	}

	tx, err := b.db.Begin()
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	appointment, err := getAppointmentByIDSQLite(tx, targetID)
	if err != nil {
		return model.Appointment{}, err
	}

	if err := validateStatusTransition(appointment.Status, targetStatus); err != nil {
		return model.Appointment{}, err
	}

	if strings.ToLower(strings.TrimSpace(appointment.Status)) == targetStatus {
		return appointment, errNoStatusChange
	}

	if _, err := tx.Exec(`UPDATE appointments SET status = ? WHERE id = ?`, targetStatus, targetID); err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo actualizar estado de cita %q: %w", targetID, err)
	}

	if err := tx.Commit(); err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo confirmar cambio de estado de cita: %w", err)
	}

	appointment.Status = targetStatus
	return appointment, nil
}

func findDuplicatePatientSQLite(tx *sql.Tx, normEmail, normPhone, excludeID string) (string, error) {
	if normEmail == "" && normPhone == "" {
		return "", nil
	}

	query := `
		SELECT id
		FROM patients
		WHERE id <> ?
		  AND ((? <> '' AND email_norm = ?) OR (? <> '' AND phone_norm = ?))
		LIMIT 1`

	var duplicateID string
	err := tx.QueryRow(query, excludeID, normEmail, normEmail, normPhone, normPhone).Scan(&duplicateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("no se pudo validar duplicado de paciente: %w", err)
	}
	return duplicateID, nil
}

func getAppointmentByIDSQLite(tx *sql.Tx, id string) (model.Appointment, error) {
	var appointment model.Appointment
	var dateTime string
	var createdAt string

	err := tx.QueryRow(
		`SELECT id, patient_id, date_time, reason, status, created_at FROM appointments WHERE id = ?`,
		id,
	).Scan(&appointment.ID, &appointment.PatientID, &dateTime, &appointment.Reason, &appointment.Status, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Appointment{}, fmt.Errorf("no se encontro la cita con id %q", id)
		}
		return model.Appointment{}, fmt.Errorf("no se pudo obtener cita %q: %w", id, err)
	}

	appointment.DateTime, err = parseStoredTime(dateTime)
	if err != nil {
		return model.Appointment{}, err
	}
	appointment.CreatedAt, err = parseStoredTime(createdAt)
	if err != nil {
		return model.Appointment{}, err
	}

	return appointment, nil
}

func parseStoredTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("timestamp almacenado vacio")
	}

	t, err := time.Parse(time.RFC3339Nano, trimmed)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("timestamp almacenado invalido %q", value)
}

func formatStoredTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatAppointmentTime(t time.Time) string {
	return t.UTC().Truncate(time.Minute).Format(time.RFC3339)
}
