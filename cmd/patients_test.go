package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPatientsGetRequiresIDAndSupportsValidLookup(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "get")
	if err == nil {
		t.Fatal("expected error when missing --id")
	}
	if !strings.Contains(output, "falta flag requerida: --id") {
		t.Fatalf("unexpected output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Ana", "--phone", "555-1010", "--email", "ana@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "get", "--id", patientID)
	if err != nil {
		t.Fatalf("unexpected get error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, patientID) || !strings.Contains(output, "Ana") {
		t.Fatalf("unexpected get output: %s", output)
	}
}

func TestPatientsUpdateWithoutFields(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Mario", "--phone", "555-2020", "--email", "mario@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "update", "--id", patientID)
	if err == nil {
		t.Fatal("expected update validation error")
	}
	if !strings.Contains(output, "debe proporcionar al menos un campo") {
		t.Fatalf("unexpected update output: %s", output)
	}
}

func TestPatientsDeleteBlockedByActiveAppointments(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Carla", "--phone", "555-3030", "--email", "carla@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v, output: %s", err, output)
	}
	patientID := mustExtractPatientID(t, output)

	at := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04")
	output, err = runAgendaCLI(t, cacheDir, dataFile, "appointments", "add", "--patient-id", patientID, "--at", at, "--reason", "Control")
	if err != nil {
		t.Fatalf("unexpected appointment add error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "delete", "--id", patientID)
	if err == nil {
		t.Fatal("expected delete blocked error")
	}
	if !strings.Contains(output, "tiene citas activas") {
		t.Fatalf("unexpected delete output: %s", output)
	}
}

func TestPatientsSearchShowsResultsAndEmptyState(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLI(t, cacheDir, dataFile, "patients", "add", "--name", "Carla Diaz", "--phone", "555-4040", "--email", "carla@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "search", "--query", "Carla")
	if err != nil {
		t.Fatalf("unexpected search error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "Carla Diaz") {
		t.Fatalf("unexpected search output: %s", output)
	}

	output, err = runAgendaCLI(t, cacheDir, dataFile, "patients", "search", "--query", "ZZZ")
	if err != nil {
		t.Fatalf("unexpected empty search error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "No se encontraron pacientes.") {
		t.Fatalf("unexpected empty search output: %s", output)
	}
}

func TestMutatingCommandsRequireActor(t *testing.T) {
	cacheDir := t.TempDir()
	dataFile := filepath.Join(t.TempDir(), "agenda.json")

	output, err := runAgendaCLINoActor(t, cacheDir, dataFile, "patients", "add", "--name", "Sin Actor", "--phone", "555-0000", "--email", "sin-actor@mail.com")
	if err == nil {
		t.Fatal("expected actor-required error")
	}
	if !strings.Contains(output, "actor es obligatorio") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestSQLiteBackendViaCLI(t *testing.T) {
	cacheDir := t.TempDir()
	dbFile := filepath.Join(t.TempDir(), "agenda.db")

	output, err := runAgendaCLI(t, cacheDir, dbFile, "--storage", "sqlite", "patients", "add", "--name", "SQLite User", "--phone", "555-7878", "--email", "sqlite@mail.com")
	if err != nil {
		t.Fatalf("unexpected sqlite add error: %v, output: %s", err, output)
	}

	output, err = runAgendaCLI(t, cacheDir, dbFile, "--storage", "sqlite", "patients", "list")
	if err != nil {
		t.Fatalf("unexpected sqlite list error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "SQLite User") {
		t.Fatalf("unexpected sqlite list output: %s", output)
	}
}
