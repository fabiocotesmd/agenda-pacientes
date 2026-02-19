package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServicesCRUDAndSearch(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "services", "add", "--name", "Sala A", "--kind", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v, output: %s", err, output)
	}
	serviceID := mustExtractServiceID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "get", "--id", serviceID)
	if err != nil {
		t.Fatalf("unexpected get service error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Sala A") {
		t.Fatalf("unexpected get output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "update", "--id", serviceID, "--kind", "sala quirurgica")
	if err != nil {
		t.Fatalf("unexpected update service error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "search", "--query", "quirurgica")
	if err != nil {
		t.Fatalf("unexpected search service error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Sala A") {
		t.Fatalf("unexpected search output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "delete", "--id", serviceID)
	if err != nil {
		t.Fatalf("unexpected delete service error: %v, output: %s", err, output)
	}
}

func TestServicesDeleteBlockedByActiveAppointments(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "services", "add", "--name", "Sala B", "--kind", "consultorio")
	if err != nil {
		t.Fatalf("unexpected add service error: %v, output: %s", err, output)
	}
	serviceID := mustExtractServiceID(t, output)

	output, err = runAgendaCLI(
		t,
		cacheDir,
		dataFile,
		"professionals",
		"add",
		"--name",
		"Dr. Activo",
		"--primary-role",
		"medico",
		"--secondary-role",
		"general",
	)
	if err != nil {
		t.Fatalf("unexpected add professional error: %v, output: %s", err, output)
	}
	professionalID := mustExtractProfessionalID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Paciente S", "--phone", "555-2222", "--email", "paciente-s@mail.com")
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

	output, err = runAgendaCLI(t, cacheDir, dataFile, "services", "delete", "--id", serviceID)
	if err == nil {
		t.Fatal("expected delete blocked error")
	}
	if !strings.Contains(output, "tiene citas activas") {
		t.Fatalf("unexpected output: %s", output)
	}
}
