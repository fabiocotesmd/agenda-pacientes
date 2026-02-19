package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDateTimeLocalAndRFC3339(t *testing.T) {
	localValue := "2030-04-01 10:15"
	localParsed, err := parseDateTime(localValue)
	if err != nil {
		t.Fatalf("unexpected local parse error: %v", err)
	}
	if localParsed.Location() != time.Local {
		t.Fatalf("expected local timezone, got %v", localParsed.Location())
	}
	if localParsed.Format("2006-01-02 15:04") != localValue {
		t.Fatalf("unexpected local parsed value: %s", localParsed.Format("2006-01-02 15:04"))
	}

	rfc := "2030-04-01T10:15:00Z"
	rfcParsed, err := parseDateTime(rfc)
	if err != nil {
		t.Fatalf("unexpected rfc parse error: %v", err)
	}
	if !rfcParsed.Equal(time.Date(2030, 4, 1, 10, 15, 0, 0, time.UTC)) {
		t.Fatalf("unexpected rfc parsed value: %v", rfcParsed)
	}
}

func TestAppointmentsAddRejectsEmptyReason(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente", "--phone", "555-5050", "--email", "paciente@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)

	at := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at,
		"--reason",
		"   ",
	)
	if err == nil {
		t.Fatal("expected reason validation error")
	}
	if !strings.Contains(output, "reason es obligatorio") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsAddRequiresProfessionalAndService(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente X", "--phone", "555-1111", "--email", "paciente-x@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	at := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04")

	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--at",
		at,
		"--reason",
		"Control",
	)
	if err == nil {
		t.Fatal("expected missing professional-id error")
	}
	if !strings.Contains(output, "falta flag requerida: --professional-id") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsListInvalidFromTo(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "appointments", "list", "--from", "invalid-date")
	if err == nil {
		t.Fatal("expected invalid from error")
	}
	if !strings.Contains(output, "from invalido") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsRescheduleRequiresIDAndAt(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "appointments", "reschedule")
	if err == nil {
		t.Fatal("expected missing id error")
	}
	if !strings.Contains(output, "falta flag requerida: --id") {
		t.Fatalf("unexpected output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "reschedule", "--id", "a_dummy")
	if err == nil {
		t.Fatal("expected missing at error")
	}
	if !strings.Contains(output, "falta flag requerida: --at") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsAddWithForm(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente Q", "--phone", "555-6060", "--email", "paciente-q@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)
	at := time.Now().Add(3 * time.Hour).Format("2006-01-02 15:04")

	input := strings.Join([]string{patientID, professionalID, serviceID, at, "Control"}, "\n") + "\n"
	output, err = runAgendaCLIWithInput(t, cacheDir, dataFile, input, "appointments", "add", "-f")
	if err != nil {
		t.Fatalf("unexpected form add error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Cita creada:") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsRescheduleWithForm(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente R", "--phone", "555-7070", "--email", "paciente-r@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)

	at1 := time.Now().Add(3 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at1,
		"--reason",
		"Control",
	)
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v, output: %s", err, output)
	}
	apptID := mustExtractAppointmentID(t, output)

	at2 := time.Now().Add(4 * time.Hour).Format("2006-01-02 15:04")
	input := strings.Join([]string{apptID, at2}, "\n") + "\n"
	output, err = runAgendaCLIWithInput(t, cacheDir, dataFile, input, "appointments", "reschedule", "--form")
	if err != nil {
		t.Fatalf("unexpected form reschedule error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Cita reprogramada:") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsCancelWithForm(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente C", "--phone", "555-8080", "--email", "paciente-c@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)

	at := time.Now().Add(3 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at,
		"--reason",
		"Control",
	)
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v, output: %s", err, output)
	}
	apptID := mustExtractAppointmentID(t, output)

	output, err = runAgendaCLIWithInput(t, cacheDir, dataFile, apptID+"\n", "appointments", "cancel", "-f")
	if err != nil {
		t.Fatalf("unexpected form cancel error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Cita cancelada:") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsFormUsesPresetValues(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente M", "--phone", "555-9090", "--email", "paciente-m@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)
	at := time.Now().Add(5 * time.Hour).Format("2006-01-02 15:04")

	output, err = runAgendaCLIWithInput(
		t,
		cacheDir,
		dataFile,
		"Seguimiento\n",
		"appointments",
		"add",
		"-f",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at,
	)
	if err != nil {
		t.Fatalf("unexpected mixed form add error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Cita creada:") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsQuestionnaireFlagsAreRejected(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "appointments", "add", "-q")
	if err == nil {
		t.Fatal("expected unknown -q flag error")
	}
	if !strings.Contains(output, "flag provided but not defined: -q") {
		t.Fatalf("unexpected output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "add", "--questionnaire")
	if err == nil {
		t.Fatal("expected unknown --questionnaire flag error")
	}
	if !strings.Contains(output, "flag provided but not defined: -questionnaire") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAppointmentsSetStatusStrictFlow(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente S", "--phone", "555-1112", "--email", "paciente-s@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)
	professionalID := mustCreateProfessional(t, cacheDir, dataFile)
	serviceID := mustCreateService(t, cacheDir, dataFile)

	at := time.Now().Add(3 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at,
		"--reason",
		"Control",
	)
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v, output: %s", err, output)
	}
	apptID := mustExtractAppointmentID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "set-status", "--id", apptID, "--status", "confirmada")
	if err != nil {
		t.Fatalf("unexpected set-status confirmada error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "estado: confirmada") {
		t.Fatalf("unexpected set-status output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "set-status", "--id", apptID, "--status", "atendida")
	if err != nil {
		t.Fatalf("unexpected set-status atendida error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "cancel", "--id", apptID)
	if err == nil {
		t.Fatal("expected invalid transition error after terminal status")
	}
	if !strings.Contains(output, "transicion de estado invalida") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestStorageMigrateJSONToSQLite(t *testing.T) {
	cacheDir := t.TempDir()
	jsonFile := filepath.Join(t.TempDir(), "agenda.json")
	sqliteFile := filepath.Join(t.TempDir(), "agenda.db")

	output, err := runAgendaCLI(t, cacheDir, jsonFile, "professionals", "add", "--name", "Profesional M", "--primary-role", "medico", "--secondary-role", "general")
	if err != nil {
		t.Fatalf("unexpected add professional error: %v, output: %s", err, output)
	}
	professionalID := mustExtractProfessionalID(t, output)

	output, err = runAgendaCLI(t, cacheDir, jsonFile, "services", "add", "--name", "Consultorio M", "--kind", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v, output: %s", err, output)
	}
	serviceID := mustExtractServiceID(t, output)

	output, err = runAgendaCLI(t, cacheDir, jsonFile, "patients", "add", "--name", "Paciente M", "--phone", "555-1212", "--email", "paciente-migrate@mail.com")
	if err != nil {
		t.Fatalf("unexpected add patient error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)

	at := time.Now().Add(4 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(
		t,
		cacheDir,
		jsonFile,
		"appointments",
		"add",
		"--patient-id",
		patientID,
		"--professional-id",
		professionalID,
		"--service-id",
		serviceID,
		"--at",
		at,
		"--reason",
		"Control",
	)
	if err != nil {
		t.Fatalf("unexpected add appointment error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, jsonFile, "storage", "migrate", "--from-json", jsonFile, "--to-sqlite", sqliteFile)
	if err != nil {
		t.Fatalf("unexpected migrate error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Migracion completada") {
		t.Fatalf("unexpected migrate output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, sqliteFile, "--storage", "sqlite", "patients", "list")
	if err != nil {
		t.Fatalf("unexpected sqlite list error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Paciente M") {
		t.Fatalf("unexpected sqlite list output: %s", output)
	}
}
