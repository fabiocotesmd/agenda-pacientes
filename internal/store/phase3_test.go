package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestAppointmentConflictByProfessionalAndService(t *testing.T) {
	s := New(t.TempDir() + "/agenda.json")

	p1, _ := s.AddPatient("P1", "555-3001", "p1@mail.com")
	p2, _ := s.AddPatient("P2", "555-3002", "p2@mail.com")
	p3, _ := s.AddPatient("P3", "555-3003", "p3@mail.com")

	pr1, err := s.AddProfessional("Dr Uno", "medico", "general")
	if err != nil {
		t.Fatalf("unexpected add professional error: %v", err)
	}
	pr2, err := s.AddProfessional("Dr Dos", "medico", "cardio")
	if err != nil {
		t.Fatalf("unexpected add professional error: %v", err)
	}

	sv1, err := s.AddService("Consultorio A", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v", err)
	}
	sv2, err := s.AddService("Consultorio B", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v", err)
	}

	at := time.Now().Add(3 * time.Hour).Truncate(time.Minute)
	if _, err := s.ScheduleAppointment(p1.ID, pr1.ID, sv1.ID, at, "Control"); err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	_, err = s.ScheduleAppointment(p2.ID, pr1.ID, sv2.ID, at, "Control")
	if err == nil || !strings.Contains(err.Error(), "conflicto") {
		t.Fatalf("expected conflict by professional, got: %v", err)
	}

	_, err = s.ScheduleAppointment(p2.ID, pr2.ID, sv1.ID, at, "Control")
	if err == nil || !strings.Contains(err.Error(), "conflicto") {
		t.Fatalf("expected conflict by service, got: %v", err)
	}

	if _, err := s.ScheduleAppointment(p3.ID, pr2.ID, sv2.ID, at, "Control"); err != nil {
		t.Fatalf("expected schedule success without conflict, got: %v", err)
	}
}

func TestScheduleAppointmentRejectsMissingProfessionalOrService(t *testing.T) {
	s := New(t.TempDir() + "/agenda.json")

	p, _ := s.AddPatient("Paciente", "555-3101", "paciente@mail.com")
	pr, _ := s.AddProfessional("Dr", "medico", "general")
	sv, _ := s.AddService("Consultorio", "consultorio")
	at := time.Now().Add(2 * time.Hour)

	_, err := s.ScheduleAppointment(p.ID, "pr_missing", sv.ID, at, "Control")
	if err == nil || !strings.Contains(err.Error(), "no existe el profesional") {
		t.Fatalf("expected missing professional error, got: %v", err)
	}

	_, err = s.ScheduleAppointment(p.ID, pr.ID, "sv_missing", at, "Control")
	if err == nil || !strings.Contains(err.Error(), "no existe el servicio") {
		t.Fatalf("expected missing service error, got: %v", err)
	}
}

func TestJSONBackfillLegacyDataAddsDefaultsAndAuditEvent(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "agenda.json")
	auditFile := jsonFile + ".audit.log"

	legacy := `{
  "patients": [
    {"id":"p_legacy","name":"Legacy","phone":"555","email":"legacy@mail.com","created_at":"2026-01-01T10:00:00Z"}
  ],
  "appointments": [
    {"id":"a_legacy","patient_id":"p_legacy","date_time":"2030-01-01T10:00:00Z","reason":"Control","status":"programada","created_at":"2026-01-01T10:00:00Z"}
  ]
}`
	if err := os.WriteFile(jsonFile, []byte(legacy), 0o644); err != nil {
		t.Fatalf("unexpected write legacy json error: %v", err)
	}

	s := NewWithConfig(Config{
		Storage:  "json",
		DataFile: jsonFile,
		Actor:    "",
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	appointments, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true})
	if err != nil {
		t.Fatalf("unexpected list appointments error: %v", err)
	}
	if len(appointments) != 1 {
		t.Fatalf("expected 1 appointment after backfill, got %d", len(appointments))
	}
	if appointments[0].ProfessionalID != defaultProfessionalID || appointments[0].ServiceID != defaultServiceID {
		t.Fatalf("expected backfilled default ids, got professional=%s service=%s", appointments[0].ProfessionalID, appointments[0].ServiceID)
	}

	professionals, err := s.ListProfessionals()
	if err != nil {
		t.Fatalf("unexpected list professionals error: %v", err)
	}
	if len(professionals) == 0 {
		t.Fatal("expected default professional after backfill")
	}

	services, err := s.ListServices()
	if err != nil {
		t.Fatalf("unexpected list services error: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("expected default service after backfill")
	}

	bytes, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("unexpected audit read error: %v", err)
	}
	content := string(bytes)
	if !strings.Contains(content, `"action":"storage.backfill_v3"`) {
		t.Fatalf("expected storage.backfill_v3 event, got: %s", content)
	}
	if !strings.Contains(content, `"actor":"system:migration"`) {
		t.Fatalf("expected system:migration actor, got: %s", content)
	}
}

func TestSQLiteLegacyMigrationToV3BackfillsDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("unexpected open sqlite error: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		t.Fatalf("unexpected pragma error: %v", err)
	}
	stmts := []string{
		`CREATE TABLE patients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT NOT NULL,
			email TEXT NOT NULL,
			email_norm TEXT NOT NULL,
			phone_norm TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE appointments (
			id TEXT PRIMARY KEY,
			patient_id TEXT NOT NULL,
			date_time TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(patient_id) REFERENCES patients(id) ON DELETE CASCADE
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("unexpected legacy schema error: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO patients(id, name, phone, email, email_norm, phone_norm, created_at)
		 VALUES('p_legacy', 'Legacy', '555', 'legacy@mail.com', 'legacy@mail.com', '555', '2026-01-01T10:00:00Z')`,
	); err != nil {
		t.Fatalf("unexpected insert legacy patient error: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO appointments(id, patient_id, date_time, reason, status, created_at)
		 VALUES('a_legacy', 'p_legacy', '2030-01-01T10:00:00Z', 'Control', 'programada', '2026-01-01T10:00:00Z')`,
	); err != nil {
		t.Fatalf("unexpected insert legacy appointment error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("unexpected close sqlite error: %v", err)
	}

	s := NewWithConfig(Config{
		Storage:  "sqlite",
		DataFile: dbPath,
		Actor:    "",
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected store init error: %v", err)
	}

	appointments, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true})
	if err != nil {
		t.Fatalf("unexpected list appointments error: %v", err)
	}
	if len(appointments) != 1 {
		t.Fatalf("expected 1 appointment, got %d", len(appointments))
	}
	if appointments[0].ProfessionalID != defaultProfessionalID || appointments[0].ServiceID != defaultServiceID {
		t.Fatalf("expected default ids after sqlite migration, got professional=%s service=%s", appointments[0].ProfessionalID, appointments[0].ServiceID)
	}
}

func TestProfessionalAndServiceUniquenessNormalized(t *testing.T) {
	s := New(t.TempDir() + "/agenda.json")

	if _, err := s.AddProfessional("Dr. Nombre", "medico", "general"); err != nil {
		t.Fatalf("unexpected add professional error: %v", err)
	}
	if _, err := s.AddProfessional("  dr.   nombre ", "medico", "otra"); err == nil || !strings.Contains(err.Error(), "profesional duplicado") {
		t.Fatalf("expected duplicate professional error, got: %v", err)
	}

	if _, err := s.AddService("Sala Uno", "consultorio"); err != nil {
		t.Fatalf("unexpected add service error: %v", err)
	}
	if _, err := s.AddService("  sala   uno ", "quirurgico"); err == nil || !strings.Contains(err.Error(), "servicio duplicado") {
		t.Fatalf("expected duplicate service error, got: %v", err)
	}
}

func TestProfessionalAndServiceAuditEvents(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "agenda.json")
	auditFile := jsonFile + ".audit.log"

	s := NewWithConfig(Config{
		Storage:  "json",
		DataFile: jsonFile,
		Actor:    "auditor-f3",
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	professional, err := s.AddProfessional("Dr Audit", "medico", "general")
	if err != nil {
		t.Fatalf("unexpected add professional error: %v", err)
	}
	if _, err := s.UpdateProfessional(professional.ID, "", "", "clinica"); err != nil {
		t.Fatalf("unexpected update professional error: %v", err)
	}
	if err := s.DeleteProfessional(professional.ID); err != nil {
		t.Fatalf("unexpected delete professional error: %v", err)
	}

	service, err := s.AddService("Sala Audit", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v", err)
	}
	if _, err := s.UpdateService(service.ID, "", "quirurgico"); err != nil {
		t.Fatalf("unexpected update service error: %v", err)
	}
	if err := s.DeleteService(service.ID); err != nil {
		t.Fatalf("unexpected delete service error: %v", err)
	}

	bytes, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("unexpected audit read error: %v", err)
	}
	content := string(bytes)
	for _, action := range []string{
		`"action":"professional.add"`,
		`"action":"professional.update"`,
		`"action":"professional.delete"`,
		`"action":"service.add"`,
		`"action":"service.update"`,
		`"action":"service.delete"`,
	} {
		if !strings.Contains(content, action) {
			t.Fatalf("expected audit action %s, got: %s", action, content)
		}
	}
}
