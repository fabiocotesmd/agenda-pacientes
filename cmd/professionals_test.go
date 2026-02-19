package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfessionalsCRUDAndSearch(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"professionals",
		"add",
		"--name",
		"Dra. Gomez",
		"--primary-role",
		"medico",
		"--secondary-role",
		"pediatra",
	)
	if err != nil {
		t.Fatalf("unexpected add professional error: %v, output: %s", err, output)
	}
	professionalID := mustExtractProfessionalID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "professionals", "get", "--id", professionalID)
	if err != nil {
		t.Fatalf("unexpected get professional error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Dra. Gomez") {
		t.Fatalf("unexpected get output: %s", output)
	}

	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"professionals",
		"update",
		"--id",
		professionalID,
		"--secondary-role",
		"clinica",
	)
	if err != nil {
		t.Fatalf("unexpected update professional error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "professionals", "search", "--query", "gomez")
	if err != nil {
		t.Fatalf("unexpected search professional error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Dra. Gomez") {
		t.Fatalf("unexpected search output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "professionals", "delete", "--id", professionalID)
	if err != nil {
		t.Fatalf("unexpected delete professional error: %v, output: %s", err, output)
	}
}

func TestProfessionalsDeleteBlockedByActiveAppointments(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"professionals",
		"add",
		"--name",
		"Dr. Bloqueo",
		"--primary-role",
		"medico",
		"--secondary-role",
		"general",
	)
	if err != nil {
		t.Fatalf("unexpected add professional error: %v, output: %s", err, output)
	}
	professionalID := mustExtractProfessionalID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "add", "--name", "Consultorio 1", "--kind", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v, output: %s", err, output)
	}
	serviceID := mustExtractServiceID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente P", "--phone", "555-1234", "--email", "paciente-p@mail.com")
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

	output, err = runAgendaCLI(t, cacheDir, dataFile, "professionals", "delete", "--id", professionalID)
	if err == nil {
		t.Fatal("expected delete blocked error")
	}
	if !strings.Contains(output, "tiene citas activas") {
		t.Fatalf("unexpected output: %s", output)
	}
}
