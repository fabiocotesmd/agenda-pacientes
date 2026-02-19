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

	backfillPending bool
}

func newSQLiteBackend(path string) (backend, error) {
	db, err := openSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	backfilled, err := ensureSQLiteSchema(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &sqliteBackend{
		path:            path,
		db:              db,
		backfillPending: backfilled,
	}, nil
}

func (b *sqliteBackend) EnsurePhase3Backfill() (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	changed := b.backfillPending
	b.backfillPending = false
	return changed, nil
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

func ensureSQLiteSchema(db *sql.DB) (bool, error) {
	baseStmts := []string{
		`CREATE TABLE IF NOT EXISTS patients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT NOT NULL,
			email TEXT NOT NULL,
			email_norm TEXT NOT NULL,
			phone_norm TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS professionals (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			name_norm TEXT NOT NULL,
			primary_role TEXT NOT NULL,
			secondary_role TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS services (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			name_norm TEXT NOT NULL,
			kind TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patients_email_norm_unique ON patients(email_norm) WHERE email_norm <> '';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patients_phone_norm_unique ON patients(phone_norm) WHERE phone_norm <> '';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_professionals_name_norm_unique ON professionals(name_norm) WHERE name_norm <> '';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_services_name_norm_unique ON services(name_norm) WHERE name_norm <> '';`,
	}

	for _, stmt := range baseStmts {
		if _, err := db.Exec(stmt); err != nil {
			return false, fmt.Errorf("no se pudo inicializar esquema sqlite: %w", err)
		}
	}

	if _, err := ensureSQLiteDefaults(db); err != nil {
		return false, err
	}

	columns, exists, err := sqliteTableColumns(db, "appointments")
	if err != nil {
		return false, err
	}
	if !exists {
		if err := createAppointmentsV3Table(db); err != nil {
			return false, err
		}
		if err := ensureAppointmentsV3Indexes(db); err != nil {
			return false, err
		}
		return false, nil
	}

	var appointmentsCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM appointments`).Scan(&appointmentsCount); err != nil {
		return false, fmt.Errorf("no se pudo contar citas existentes: %w", err)
	}

	backfilled := false
	needsMigration := !columns["professional_id"] || !columns["service_id"]
	if needsMigration {
		if err := migrateAppointmentsV2ToV3(db, columns); err != nil {
			return false, err
		}
		if appointmentsCount > 0 {
			backfilled = true
		}
	} else {
		updated, err := backfillAppointmentsV3Defaults(db)
		if err != nil {
			return false, err
		}
		if updated > 0 {
			backfilled = true
		}
	}

	if err := ensureAppointmentsV3Indexes(db); err != nil {
		return false, err
	}

	return backfilled, nil
}

func sqliteHasAnyData(db *sql.DB) (bool, error) {
	var total int
	err := db.QueryRow(
		`SELECT
			(SELECT COUNT(1) FROM patients) +
			(SELECT COUNT(1) FROM professionals WHERE id <> 'pr_general') +
			(SELECT COUNT(1) FROM services WHERE id <> 'sv_general') +
			(SELECT COUNT(1) FROM appointments)`,
	).Scan(&total)
	if err != nil {
		return false, fmt.Errorf("no se pudo consultar contenido sqlite: %w", err)
	}
	return total > 0, nil
}

func sqliteResetSchema(db *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS appointments;`,
		`DROP TABLE IF EXISTS patients;`,
		`DROP TABLE IF EXISTS professionals;`,
		`DROP TABLE IF EXISTS services;`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("no se pudo reiniciar esquema sqlite: %w", err)
		}
	}

	_, err := ensureSQLiteSchema(db)
	return err
}

func sqliteTableColumns(db *sql.DB, table string) (map[string]bool, bool, error) {
	var found string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&found)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("no se pudo verificar tabla %s: %w", table, err)
	}

	rows, err := db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return nil, false, fmt.Errorf("no se pudo leer columnas de %s: %w", table, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name      string
			fieldType string
			notNull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &fieldType, &notNull, &dflt, &pk); err != nil {
			return nil, false, fmt.Errorf("no se pudo leer metadata de columna en %s: %w", table, err)
		}
		cols[strings.ToLower(strings.TrimSpace(name))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("error al iterar columnas de %s: %w", table, err)
	}

	return cols, true, nil
}

func ensureSQLiteDefaults(db *sql.DB) (bool, error) {
	changed := false

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM professionals WHERE id = ?`, defaultProfessionalID).Scan(&count); err != nil {
		return false, fmt.Errorf("no se pudo verificar profesional por defecto: %w", err)
	}
	if count == 0 {
		p := defaultProfessional()
		if _, err := db.Exec(
			`INSERT INTO professionals(id, name, name_norm, primary_role, secondary_role, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
			p.ID,
			p.Name,
			normalizeNameKey(p.Name),
			p.PrimaryRole,
			p.SecondaryRole,
			formatStoredTime(p.CreatedAt),
		); err != nil {
			return false, fmt.Errorf("no se pudo crear profesional por defecto: %w", err)
		}
		changed = true
	}

	if err := db.QueryRow(`SELECT COUNT(1) FROM services WHERE id = ?`, defaultServiceID).Scan(&count); err != nil {
		return false, fmt.Errorf("no se pudo verificar servicio por defecto: %w", err)
	}
	if count == 0 {
		s := defaultService()
		if _, err := db.Exec(
			`INSERT INTO services(id, name, name_norm, kind, created_at) VALUES(?, ?, ?, ?, ?)`,
			s.ID,
			s.Name,
			normalizeNameKey(s.Name),
			s.Kind,
			formatStoredTime(s.CreatedAt),
		); err != nil {
			return false, fmt.Errorf("no se pudo crear servicio por defecto: %w", err)
		}
		changed = true
	}

	return changed, nil
}

func createAppointmentsV3Table(db *sql.DB) error {
	stmt := `CREATE TABLE IF NOT EXISTS appointments (
		id TEXT PRIMARY KEY,
		patient_id TEXT NOT NULL,
		professional_id TEXT NOT NULL,
		service_id TEXT NOT NULL,
		date_time TEXT NOT NULL,
		reason TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY(patient_id) REFERENCES patients(id) ON DELETE CASCADE,
		FOREIGN KEY(professional_id) REFERENCES professionals(id) ON DELETE CASCADE,
		FOREIGN KEY(service_id) REFERENCES services(id) ON DELETE CASCADE
	);`
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("no se pudo crear tabla appointments v3: %w", err)
	}
	return nil
}

func migrateAppointmentsV2ToV3(db *sql.DB, oldColumns map[string]bool) error {
	if _, err := ensureSQLiteDefaults(db); err != nil {
		return err
	}

	if _, err := db.Exec(`DROP TABLE IF EXISTS appointments_v3_tmp;`); err != nil {
		return fmt.Errorf("no se pudo limpiar tabla temporal de citas v3: %w", err)
	}

	createTmp := `CREATE TABLE appointments_v3_tmp (
		id TEXT PRIMARY KEY,
		patient_id TEXT NOT NULL,
		professional_id TEXT NOT NULL,
		service_id TEXT NOT NULL,
		date_time TEXT NOT NULL,
		reason TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY(patient_id) REFERENCES patients(id) ON DELETE CASCADE,
		FOREIGN KEY(professional_id) REFERENCES professionals(id) ON DELETE CASCADE,
		FOREIGN KEY(service_id) REFERENCES services(id) ON DELETE CASCADE
	);`
	if _, err := db.Exec(createTmp); err != nil {
		return fmt.Errorf("no se pudo crear tabla temporal de citas v3: %w", err)
	}

	profExpr := fmt.Sprintf("'%s'", defaultProfessionalID)
	if oldColumns["professional_id"] {
		profExpr = fmt.Sprintf("COALESCE(NULLIF(professional_id, ''), '%s')", defaultProfessionalID)
	}
	serviceExpr := fmt.Sprintf("'%s'", defaultServiceID)
	if oldColumns["service_id"] {
		serviceExpr = fmt.Sprintf("COALESCE(NULLIF(service_id, ''), '%s')", defaultServiceID)
	}

	copyQuery := fmt.Sprintf(
		`INSERT INTO appointments_v3_tmp(id, patient_id, professional_id, service_id, date_time, reason, status, created_at)
		 SELECT id, patient_id, %s, %s, date_time, reason, status, created_at
		 FROM appointments`,
		profExpr,
		serviceExpr,
	)
	if _, err := db.Exec(copyQuery); err != nil {
		return fmt.Errorf("no se pudo migrar citas legacy a v3: %w", err)
	}

	if _, err := db.Exec(`DROP TABLE appointments;`); err != nil {
		return fmt.Errorf("no se pudo eliminar tabla appointments legacy: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE appointments_v3_tmp RENAME TO appointments;`); err != nil {
		return fmt.Errorf("no se pudo renombrar tabla appointments v3: %w", err)
	}

	return nil
}

func backfillAppointmentsV3Defaults(db *sql.DB) (int64, error) {
	if _, err := ensureSQLiteDefaults(db); err != nil {
		return 0, err
	}

	total := int64(0)
	result, err := db.Exec(
		`UPDATE appointments
		 SET professional_id = ?
		 WHERE professional_id IS NULL
		   OR professional_id = ''
		   OR NOT EXISTS (SELECT 1 FROM professionals p WHERE p.id = appointments.professional_id)`,
		defaultProfessionalID,
	)
	if err != nil {
		return 0, fmt.Errorf("no se pudo backfillear professional_id en citas: %w", err)
	}
	n, _ := result.RowsAffected()
	total += n

	result, err = db.Exec(
		`UPDATE appointments
		 SET service_id = ?
		 WHERE service_id IS NULL
		   OR service_id = ''
		   OR NOT EXISTS (SELECT 1 FROM services s WHERE s.id = appointments.service_id)`,
		defaultServiceID,
	)
	if err != nil {
		return 0, fmt.Errorf("no se pudo backfillear service_id en citas: %w", err)
	}
	n, _ = result.RowsAffected()
	total += n

	return total, nil
}

func ensureAppointmentsV3Indexes(db *sql.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_appointments_date_time ON appointments(date_time);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_patient_id ON appointments(patient_id);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_professional_id ON appointments(professional_id);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_service_id ON appointments(service_id);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_status ON appointments(status);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_professional_date_time ON appointments(professional_id, date_time);`,
		`CREATE INDEX IF NOT EXISTS idx_appointments_service_date_time ON appointments(service_id, date_time);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("no se pudo crear indice de citas v3: %w", err)
		}
	}
	return nil
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

func (b *sqliteBackend) AddProfessional(name, primaryRole, secondaryRole string) (model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	trimmedName := normalizeRoleOrKind(name)
	if trimmedName == "" {
		return model.Professional{}, errors.New("el nombre del profesional es obligatorio")
	}
	trimmedPrimaryRole := normalizeRoleOrKind(primaryRole)
	if trimmedPrimaryRole == "" {
		return model.Professional{}, errors.New("primary-role es obligatorio")
	}
	trimmedSecondaryRole := normalizeRoleOrKind(secondaryRole)

	normName := normalizeNameKey(trimmedName)
	dupID, err := findDuplicateProfessionalSQLite(b.db, normName, "")
	if err != nil {
		return model.Professional{}, err
	}
	if dupID != "" {
		return model.Professional{}, fmt.Errorf("profesional duplicado: coincide con %q", dupID)
	}

	professional := model.Professional{
		ID:            newID("pr"),
		Name:          trimmedName,
		PrimaryRole:   trimmedPrimaryRole,
		SecondaryRole: trimmedSecondaryRole,
		CreatedAt:     time.Now().UTC(),
	}

	_, err = b.db.Exec(
		`INSERT INTO professionals(id, name, name_norm, primary_role, secondary_role, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		professional.ID,
		professional.Name,
		normName,
		professional.PrimaryRole,
		professional.SecondaryRole,
		formatStoredTime(professional.CreatedAt),
	)
	if err != nil {
		return model.Professional{}, fmt.Errorf("no se pudo crear profesional: %w", err)
	}

	return professional, nil
}

func (b *sqliteBackend) ListProfessionals() ([]model.Professional, error) {
	rows, err := b.db.Query(
		`SELECT id, name, primary_role, secondary_role, created_at FROM professionals ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar profesionales: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := []model.Professional{}
	for rows.Next() {
		var p model.Professional
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.PrimaryRole, &p.SecondaryRole, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer profesional: %w", err)
		}
		p.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar profesionales: %w", err)
	}
	return out, nil
}

func (b *sqliteBackend) GetProfessionalByID(id string) (model.Professional, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return model.Professional{}, errors.New("id de profesional obligatorio")
	}

	var p model.Professional
	var createdAt string
	err := b.db.QueryRow(
		`SELECT id, name, primary_role, secondary_role, created_at FROM professionals WHERE id = ?`,
		target,
	).Scan(&p.ID, &p.Name, &p.PrimaryRole, &p.SecondaryRole, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Professional{}, fmt.Errorf("no se encontro el profesional con id %q", target)
		}
		return model.Professional{}, fmt.Errorf("no se pudo obtener profesional %q: %w", target, err)
	}
	p.CreatedAt, err = parseStoredTime(createdAt)
	if err != nil {
		return model.Professional{}, err
	}
	return p, nil
}

func (b *sqliteBackend) UpdateProfessional(id, name, primaryRole, secondaryRole string) (model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Professional{}, errors.New("id de profesional obligatorio")
	}

	trimmedName := normalizeRoleOrKind(name)
	trimmedPrimaryRole := normalizeRoleOrKind(primaryRole)
	trimmedSecondaryRole := normalizeRoleOrKind(secondaryRole)
	if trimmedName == "" && trimmedPrimaryRole == "" && trimmedSecondaryRole == "" {
		return model.Professional{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	current, err := b.GetProfessionalByID(target)
	if err != nil {
		return model.Professional{}, err
	}

	updated := current
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedPrimaryRole != "" {
		updated.PrimaryRole = trimmedPrimaryRole
	}
	if trimmedSecondaryRole != "" {
		updated.SecondaryRole = trimmedSecondaryRole
	}

	if strings.TrimSpace(updated.Name) == "" {
		return model.Professional{}, errors.New("el nombre del profesional es obligatorio")
	}
	if strings.TrimSpace(updated.PrimaryRole) == "" {
		return model.Professional{}, errors.New("primary-role es obligatorio")
	}

	normName := normalizeNameKey(updated.Name)
	dupID, err := findDuplicateProfessionalSQLite(b.db, normName, updated.ID)
	if err != nil {
		return model.Professional{}, err
	}
	if dupID != "" {
		return model.Professional{}, fmt.Errorf("profesional duplicado: coincide con %q", dupID)
	}

	_, err = b.db.Exec(
		`UPDATE professionals SET name = ?, name_norm = ?, primary_role = ?, secondary_role = ? WHERE id = ?`,
		updated.Name,
		normName,
		updated.PrimaryRole,
		updated.SecondaryRole,
		updated.ID,
	)
	if err != nil {
		return model.Professional{}, fmt.Errorf("no se pudo actualizar profesional %q: %w", updated.ID, err)
	}
	return updated, nil
}

func (b *sqliteBackend) DeleteProfessional(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de profesional obligatorio")
	}
	if target == defaultProfessionalID {
		return errors.New("no se puede eliminar el profesional por defecto")
	}

	if _, err := b.GetProfessionalByID(target); err != nil {
		return err
	}

	var activeCount int
	err := b.db.QueryRow(
		`SELECT COUNT(1) FROM appointments WHERE professional_id = ? AND status IN (?, ?)`,
		target,
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
	).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("no se pudo validar citas activas del profesional %q: %w", target, err)
	}
	if activeCount > 0 {
		return fmt.Errorf("no se puede eliminar el profesional %q porque tiene citas activas", target)
	}

	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`DELETE FROM appointments WHERE professional_id = ?`, target); err != nil {
		return fmt.Errorf("no se pudieron eliminar citas historicas del profesional %q: %w", target, err)
	}
	if _, err := tx.Exec(`DELETE FROM professionals WHERE id = ?`, target); err != nil {
		return fmt.Errorf("no se pudo eliminar profesional %q: %w", target, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("no se pudo confirmar eliminacion de profesional %q: %w", target, err)
	}
	return nil
}

func (b *sqliteBackend) SearchProfessionals(query string) ([]model.Professional, error) {
	q := strings.ToLower(strings.TrimSpace(query))

	var rows *sql.Rows
	var err error

	if q == "" {
		rows, err = b.db.Query(
			`SELECT id, name, primary_role, secondary_role, created_at FROM professionals ORDER BY created_at ASC`,
		)
	} else {
		like := "%" + q + "%"
		rows, err = b.db.Query(
			`SELECT id, name, primary_role, secondary_role, created_at
			 FROM professionals
			 WHERE lower(name) LIKE ? OR lower(primary_role) LIKE ? OR lower(secondary_role) LIKE ?
			 ORDER BY created_at ASC`,
			like,
			like,
			like,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar profesionales: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := []model.Professional{}
	for rows.Next() {
		var p model.Professional
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.PrimaryRole, &p.SecondaryRole, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer profesional de busqueda: %w", err)
		}
		p.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar profesionales de busqueda: %w", err)
	}
	return out, nil
}

func (b *sqliteBackend) AddService(name, kind string) (model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	trimmedName := normalizeRoleOrKind(name)
	if trimmedName == "" {
		return model.Service{}, errors.New("el nombre del servicio es obligatorio")
	}
	trimmedKind := normalizeRoleOrKind(kind)
	if trimmedKind == "" {
		return model.Service{}, errors.New("kind es obligatorio")
	}

	normName := normalizeNameKey(trimmedName)
	dupID, err := findDuplicateServiceSQLite(b.db, normName, "")
	if err != nil {
		return model.Service{}, err
	}
	if dupID != "" {
		return model.Service{}, fmt.Errorf("servicio duplicado: coincide con %q", dupID)
	}

	service := model.Service{
		ID:        newID("sv"),
		Name:      trimmedName,
		Kind:      trimmedKind,
		CreatedAt: time.Now().UTC(),
	}

	_, err = b.db.Exec(
		`INSERT INTO services(id, name, name_norm, kind, created_at) VALUES(?, ?, ?, ?, ?)`,
		service.ID,
		service.Name,
		normName,
		service.Kind,
		formatStoredTime(service.CreatedAt),
	)
	if err != nil {
		return model.Service{}, fmt.Errorf("no se pudo crear servicio: %w", err)
	}
	return service, nil
}

func (b *sqliteBackend) ListServices() ([]model.Service, error) {
	rows, err := b.db.Query(`SELECT id, name, kind, created_at FROM services ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar servicios: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := []model.Service{}
	for rows.Next() {
		var s model.Service
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer servicio: %w", err)
		}
		s.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar servicios: %w", err)
	}
	return out, nil
}

func (b *sqliteBackend) GetServiceByID(id string) (model.Service, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return model.Service{}, errors.New("id de servicio obligatorio")
	}

	var s model.Service
	var createdAt string
	err := b.db.QueryRow(`SELECT id, name, kind, created_at FROM services WHERE id = ?`, target).
		Scan(&s.ID, &s.Name, &s.Kind, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Service{}, fmt.Errorf("no se encontro el servicio con id %q", target)
		}
		return model.Service{}, fmt.Errorf("no se pudo obtener servicio %q: %w", target, err)
	}
	s.CreatedAt, err = parseStoredTime(createdAt)
	if err != nil {
		return model.Service{}, err
	}
	return s, nil
}

func (b *sqliteBackend) UpdateService(id, name, kind string) (model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Service{}, errors.New("id de servicio obligatorio")
	}

	trimmedName := normalizeRoleOrKind(name)
	trimmedKind := normalizeRoleOrKind(kind)
	if trimmedName == "" && trimmedKind == "" {
		return model.Service{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	current, err := b.GetServiceByID(target)
	if err != nil {
		return model.Service{}, err
	}

	updated := current
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedKind != "" {
		updated.Kind = trimmedKind
	}

	if strings.TrimSpace(updated.Name) == "" {
		return model.Service{}, errors.New("el nombre del servicio es obligatorio")
	}
	if strings.TrimSpace(updated.Kind) == "" {
		return model.Service{}, errors.New("kind es obligatorio")
	}

	normName := normalizeNameKey(updated.Name)
	dupID, err := findDuplicateServiceSQLite(b.db, normName, updated.ID)
	if err != nil {
		return model.Service{}, err
	}
	if dupID != "" {
		return model.Service{}, fmt.Errorf("servicio duplicado: coincide con %q", dupID)
	}

	_, err = b.db.Exec(
		`UPDATE services SET name = ?, name_norm = ?, kind = ? WHERE id = ?`,
		updated.Name,
		normName,
		updated.Kind,
		updated.ID,
	)
	if err != nil {
		return model.Service{}, fmt.Errorf("no se pudo actualizar servicio %q: %w", updated.ID, err)
	}
	return updated, nil
}

func (b *sqliteBackend) DeleteService(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de servicio obligatorio")
	}
	if target == defaultServiceID {
		return errors.New("no se puede eliminar el servicio por defecto")
	}

	if _, err := b.GetServiceByID(target); err != nil {
		return err
	}

	var activeCount int
	err := b.db.QueryRow(
		`SELECT COUNT(1) FROM appointments WHERE service_id = ? AND status IN (?, ?)`,
		target,
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
	).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("no se pudo validar citas activas del servicio %q: %w", target, err)
	}
	if activeCount > 0 {
		return fmt.Errorf("no se puede eliminar el servicio %q porque tiene citas activas", target)
	}

	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("no se pudo iniciar transaccion sqlite: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`DELETE FROM appointments WHERE service_id = ?`, target); err != nil {
		return fmt.Errorf("no se pudieron eliminar citas historicas del servicio %q: %w", target, err)
	}
	if _, err := tx.Exec(`DELETE FROM services WHERE id = ?`, target); err != nil {
		return fmt.Errorf("no se pudo eliminar servicio %q: %w", target, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("no se pudo confirmar eliminacion de servicio %q: %w", target, err)
	}
	return nil
}

func (b *sqliteBackend) SearchServices(query string) ([]model.Service, error) {
	q := strings.ToLower(strings.TrimSpace(query))

	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = b.db.Query(`SELECT id, name, kind, created_at FROM services ORDER BY created_at ASC`)
	} else {
		like := "%" + q + "%"
		rows, err = b.db.Query(
			`SELECT id, name, kind, created_at
			 FROM services
			 WHERE lower(name) LIKE ? OR lower(kind) LIKE ?
			 ORDER BY created_at ASC`,
			like,
			like,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar servicios: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := []model.Service{}
	for rows.Next() {
		var s model.Service
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &createdAt); err != nil {
			return nil, fmt.Errorf("no se pudo leer servicio de busqueda: %w", err)
		}
		s.CreatedAt, err = parseStoredTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar servicios de busqueda: %w", err)
	}
	return out, nil
}

func (b *sqliteBackend) ScheduleAppointment(patientID, professionalID, serviceID string, at time.Time, reason string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	trimmedPatientID := strings.TrimSpace(patientID)
	if trimmedPatientID == "" {
		return model.Appointment{}, errors.New("patient-id es obligatorio")
	}
	trimmedProfessionalID := strings.TrimSpace(professionalID)
	if trimmedProfessionalID == "" {
		return model.Appointment{}, errors.New("professional-id es obligatorio")
	}
	trimmedServiceID := strings.TrimSpace(serviceID)
	if trimmedServiceID == "" {
		return model.Appointment{}, errors.New("service-id es obligatorio")
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

	if err := ensureEntityExistsSQLite(tx, "patients", trimmedPatientID, "paciente"); err != nil {
		return model.Appointment{}, err
	}
	if err := ensureEntityExistsSQLite(tx, "professionals", trimmedProfessionalID, "profesional"); err != nil {
		return model.Appointment{}, err
	}
	if err := ensureEntityExistsSQLite(tx, "services", trimmedServiceID, "servicio"); err != nil {
		return model.Appointment{}, err
	}

	var conflict int
	err = tx.QueryRow(
		`SELECT COUNT(1)
		 FROM appointments
		 WHERE date_time = ?
		   AND status IN (?, ?)
		   AND (professional_id = ? OR service_id = ?)`,
		formatAppointmentTime(normalizedDateTime),
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
		trimmedProfessionalID,
		trimmedServiceID,
	).Scan(&conflict)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo verificar conflicto de citas: %w", err)
	}
	if conflict > 0 {
		return model.Appointment{}, errors.New("ya existe una cita en conflicto para ese profesional o servicio")
	}

	appointment := model.Appointment{
		ID:             newID("a"),
		PatientID:      trimmedPatientID,
		ProfessionalID: trimmedProfessionalID,
		ServiceID:      trimmedServiceID,
		DateTime:       normalizedDateTime,
		Reason:         trimmedReason,
		Status:         model.AppointmentStatusScheduled,
		CreatedAt:      time.Now().UTC(),
	}

	_, err = tx.Exec(
		`INSERT INTO appointments(id, patient_id, professional_id, service_id, date_time, reason, status, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		appointment.ID,
		appointment.PatientID,
		appointment.ProfessionalID,
		appointment.ServiceID,
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
		`SELECT COUNT(1)
		 FROM appointments
		 WHERE id <> ?
		   AND date_time = ?
		   AND status IN (?, ?)
		   AND (professional_id = ? OR service_id = ?)`,
		target,
		formatAppointmentTime(normalizedDateTime),
		model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
		appointment.ProfessionalID,
		appointment.ServiceID,
	).Scan(&conflict)
	if err != nil {
		return model.Appointment{}, fmt.Errorf("no se pudo verificar conflicto al reprogramar: %w", err)
	}
	if conflict > 0 {
		return model.Appointment{}, errors.New("ya existe una cita en conflicto para ese profesional o servicio")
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
	query := `SELECT id, patient_id, professional_id, service_id, date_time, reason, status, created_at FROM appointments WHERE 1=1`
	args := make([]any, 0, 12)

	if !filters.IncludeCanceled {
		query += ` AND status <> ?`
		args = append(args, model.AppointmentStatusCanceled)
	}

	trimmedPatientID := strings.TrimSpace(filters.PatientID)
	if trimmedPatientID != "" {
		query += ` AND patient_id = ?`
		args = append(args, trimmedPatientID)
	}

	trimmedProfessionalID := strings.TrimSpace(filters.ProfessionalID)
	if trimmedProfessionalID != "" {
		query += ` AND professional_id = ?`
		args = append(args, trimmedProfessionalID)
	}

	trimmedServiceID := strings.TrimSpace(filters.ServiceID)
	if trimmedServiceID != "" {
		query += ` AND service_id = ?`
		args = append(args, trimmedServiceID)
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
			&appointment.ProfessionalID,
			&appointment.ServiceID,
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

func findDuplicateProfessionalSQLite(db *sql.DB, normName, excludeID string) (string, error) {
	if normName == "" {
		return "", nil
	}
	var duplicateID string
	err := db.QueryRow(
		`SELECT id FROM professionals WHERE id <> ? AND name_norm = ? LIMIT 1`,
		excludeID,
		normName,
	).Scan(&duplicateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("no se pudo validar duplicado de profesional: %w", err)
	}
	return duplicateID, nil
}

func findDuplicateServiceSQLite(db *sql.DB, normName, excludeID string) (string, error) {
	if normName == "" {
		return "", nil
	}
	var duplicateID string
	err := db.QueryRow(
		`SELECT id FROM services WHERE id <> ? AND name_norm = ? LIMIT 1`,
		excludeID,
		normName,
	).Scan(&duplicateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("no se pudo validar duplicado de servicio: %w", err)
	}
	return duplicateID, nil
}

func ensureEntityExistsSQLite(tx *sql.Tx, table, id, entityName string) error {
	var count int
	query := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE id = ?", table)
	if err := tx.QueryRow(query, id).Scan(&count); err != nil {
		return fmt.Errorf("no se pudo verificar %s %q: %w", entityName, id, err)
	}
	if count == 0 {
		return fmt.Errorf("no existe el %s con id %q", entityName, id)
	}
	return nil
}

func getAppointmentByIDSQLite(tx *sql.Tx, id string) (model.Appointment, error) {
	var appointment model.Appointment
	var dateTime string
	var createdAt string

	err := tx.QueryRow(
		`SELECT id, patient_id, professional_id, service_id, date_time, reason, status, created_at FROM appointments WHERE id = ?`,
		id,
	).Scan(
		&appointment.ID,
		&appointment.PatientID,
		&appointment.ProfessionalID,
		&appointment.ServiceID,
		&dateTime,
		&appointment.Reason,
		&appointment.Status,
		&createdAt,
	)
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
