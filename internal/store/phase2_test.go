package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agenda-pacientes/internal/model"
)

func TestSetAppointmentStatusTransitions(t *testing.T) {
	s := New(t.TempDir() + "/agenda.json")

	patient, err := s.AddPatient("Estado Test", "555-1001", "estado@test.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v", err)
	}

	appt, err := s.ScheduleAppointment(patient.ID, defaultProfessionalID, defaultServiceID, time.Now().Add(2*time.Hour), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	appt, err = s.SetAppointmentStatus(appt.ID, model.AppointmentStatusConfirmed)
	if err != nil {
		t.Fatalf("unexpected set status confirmada error: %v", err)
	}
	if appt.Status != model.AppointmentStatusConfirmed {
		t.Fatalf("expected confirmada, got %q", appt.Status)
	}

	appt, err = s.SetAppointmentStatus(appt.ID, model.AppointmentStatusAttended)
	if err != nil {
		t.Fatalf("unexpected set status atendida error: %v", err)
	}
	if appt.Status != model.AppointmentStatusAttended {
		t.Fatalf("expected atendida, got %q", appt.Status)
	}

	_, err = s.SetAppointmentStatus(appt.ID, model.AppointmentStatusCanceled)
	if err == nil || !strings.Contains(err.Error(), "transicion de estado invalida") {
		t.Fatalf("expected invalid transition error, got: %v", err)
	}
}

func TestTerminalStatusBlocksRescheduleAndCancel(t *testing.T) {
	s := New(t.TempDir() + "/agenda.json")

	patient, _ := s.AddPatient("Terminal Test", "555-1002", "terminal@test.com")
	appt, err := s.ScheduleAppointment(patient.ID, defaultProfessionalID, defaultServiceID, time.Now().Add(2*time.Hour), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	if _, err := s.SetAppointmentStatus(appt.ID, model.AppointmentStatusNoShow); err != nil {
		t.Fatalf("unexpected set status ausente error: %v", err)
	}

	_, err = s.RescheduleAppointment(appt.ID, time.Now().Add(3*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "no se puede reprogramar") {
		t.Fatalf("expected reschedule blocked error, got: %v", err)
	}

	err = s.CancelAppointment(appt.ID)
	if err == nil || !strings.Contains(err.Error(), "transicion de estado invalida") {
		t.Fatalf("expected cancel blocked error, got: %v", err)
	}
}

func TestJSONBackupsRotationAndNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "agenda.json")
	s := NewWithConfig(Config{
		Storage:         "json",
		DataFile:        jsonFile,
		Actor:           "tester",
		BackupRetention: 7,
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	for i := 0; i < 12; i++ {
		_, err := s.AddPatient(
			"Paciente Backup "+time.Now().Add(time.Duration(i)*time.Second).Format("150405"),
			"555-"+time.Now().Add(time.Duration(i)*time.Second).Format("0405"),
			"backup"+time.Now().Add(time.Duration(i)*time.Second).Format("150405")+"@mail.com",
		)
		if err != nil {
			t.Fatalf("unexpected add patient error: %v", err)
		}
	}

	backupDir := filepath.Join(dir, "agenda.backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("expected backup directory, got error: %v", err)
	}
	if len(entries) == 0 || len(entries) > 7 {
		t.Fatalf("expected backup rotation between 1 and 7 files, got %d", len(entries))
	}

	tmpFiles, err := filepath.Glob(filepath.Join(dir, "agenda.json.tmp-*"))
	if err != nil {
		t.Fatalf("unexpected glob error: %v", err)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("expected no temporary files, got %v", tmpFiles)
	}
}

func TestSQLiteDuplicateRules(t *testing.T) {
	s := NewWithConfig(Config{
		Storage:  "sqlite",
		DataFile: filepath.Join(t.TempDir(), "agenda.db"),
		Actor:    "tester",
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	_, err := s.AddPatient("SQLite One", "+54 11 5555-1000", "sqlite@test.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	_, err = s.AddPatient("SQLite Dup Email", "555-1001", "SQLITE@test.com")
	if err == nil || !strings.Contains(err.Error(), "paciente duplicado") {
		t.Fatalf("expected duplicate email error, got: %v", err)
	}

	_, err = s.AddPatient("SQLite Dup Phone", "541155551000", "other@test.com")
	if err == nil || !strings.Contains(err.Error(), "paciente duplicado") {
		t.Fatalf("expected duplicate phone error, got: %v", err)
	}
}

func TestMigrateJSONToSQLite(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "agenda.json")
	sqliteFile := filepath.Join(dir, "agenda.db")

	jsonStore := NewWithConfig(Config{
		Storage:  "json",
		DataFile: jsonFile,
		Actor:    "migrator",
	})
	if err := jsonStore.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	patient, err := jsonStore.AddPatient("Migracion", "555-5555", "migracion@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v", err)
	}
	_, err = jsonStore.ScheduleAppointment(patient.ID, defaultProfessionalID, defaultServiceID, time.Now().Add(2*time.Hour), "Control")
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v", err)
	}

	result, err := MigrateJSONToSQLite(MigrationOptions{
		FromJSON: jsonFile,
		ToSQLite: sqliteFile,
		Actor:    "migrator",
	})
	if err != nil {
		t.Fatalf("unexpected migrate error: %v", err)
	}
	if result.Patients != 1 || result.Appointments != 1 {
		t.Fatalf("unexpected migrate counts: %+v", result)
	}

	sqliteStore := NewWithConfig(Config{
		Storage:  "sqlite",
		DataFile: sqliteFile,
		Actor:    "migrator",
	})
	if err := sqliteStore.InitError(); err != nil {
		t.Fatalf("unexpected sqlite init error: %v", err)
	}

	patients, err := sqliteStore.ListPatients()
	if err != nil {
		t.Fatalf("unexpected list patients error: %v", err)
	}
	if len(patients) != 1 {
		t.Fatalf("expected 1 migrated patient, got %d", len(patients))
	}

	_, err = MigrateJSONToSQLite(MigrationOptions{
		FromJSON: jsonFile,
		ToSQLite: sqliteFile,
		Actor:    "migrator",
	})
	if err == nil || !strings.Contains(err.Error(), "no esta vacio") {
		t.Fatalf("expected non-empty destination error, got: %v", err)
	}

	_, err = MigrateJSONToSQLite(MigrationOptions{
		FromJSON:       jsonFile,
		ToSQLite:       sqliteFile,
		Actor:          "migrator",
		ForceOverwrite: true,
	})
	if err != nil {
		t.Fatalf("unexpected force migrate error: %v", err)
	}
}

func TestSetStatusNoOpDoesNotCreateAuditEvent(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "agenda.json")
	auditFile := jsonFile + ".audit.log"

	s := NewWithConfig(Config{
		Storage:  "json",
		DataFile: jsonFile,
		Actor:    "audit-user",
	})
	if err := s.InitError(); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	patient, err := s.AddPatient("Audit", "555-9999", "audit@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v", err)
	}
	appt, err := s.ScheduleAppointment(patient.ID, defaultProfessionalID, defaultServiceID, time.Now().Add(2*time.Hour), "Control")
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v", err)
	}

	if _, err := s.SetAppointmentStatus(appt.ID, model.AppointmentStatusConfirmed); err != nil {
		t.Fatalf("unexpected set-status error: %v", err)
	}

	before, err := countLines(auditFile)
	if err != nil {
		t.Fatalf("unexpected audit read error: %v", err)
	}

	if _, err := s.SetAppointmentStatus(appt.ID, model.AppointmentStatusConfirmed); err != nil {
		t.Fatalf("unexpected set-status no-op error: %v", err)
	}

	after, err := countLines(auditFile)
	if err != nil {
		t.Fatalf("unexpected audit read error: %v", err)
	}

	if before != after {
		t.Fatalf("expected no-op status change to keep audit line count (%d), got %d", before, after)
	}
}

func countLines(path string) (int, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(bytes))
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}
