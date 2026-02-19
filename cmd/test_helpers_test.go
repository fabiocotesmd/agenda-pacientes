package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func runAgendaCLI(t *testing.T, cacheDir, dataFile string, args ...string) (string, error) {
	t.Helper()
	return runAgendaCLIWithInput(t, cacheDir, dataFile, "", args...)
}

func runAgendaCLINoActor(t *testing.T, cacheDir, dataFile string, args ...string) (string, error) {
	t.Helper()
	return runAgendaCLIWithInputAndActor(t, cacheDir, dataFile, "", "", args...)
}

func runAgendaCLIWithInput(t *testing.T, cacheDir, dataFile, input string, args ...string) (string, error) {
	t.Helper()
	return runAgendaCLIWithInputAndActor(t, cacheDir, dataFile, input, "test-actor", args...)
}

func runAgendaCLIWithInputAndActor(t *testing.T, cacheDir, dataFile, input, actor string, args ...string) (string, error) {
	t.Helper()
	fullArgs := []string{"run", ".", "--data-file", dataFile}
	if strings.TrimSpace(actor) != "" {
		fullArgs = append(fullArgs, "--actor", actor)
	}
	fullArgs = append(fullArgs, args...)

	command := exec.Command("go", fullArgs...)
	command.Dir = filepath.Join("..")
	command.Env = append(os.Environ(), "GOCACHE="+cacheDir)
	if input != "" {
		command.Stdin = strings.NewReader(input)
	}

	out, err := command.CombinedOutput()
	return string(out), err
}

func mustExtractPatientID(t *testing.T, output string) string {
	t.Helper()
	re := regexp.MustCompile(`\((p_[^)]+)\)`)
	match := re.FindStringSubmatch(output)
	if len(match) != 2 {
		t.Fatalf("could not parse patient id from output: %s", output)
	}
	return strings.TrimSpace(match[1])
}

func mustExtractAppointmentID(t *testing.T, output string) string {
	t.Helper()
	re := regexp.MustCompile(`Cita creada: (a_[^| ]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) != 2 {
		t.Fatalf("could not parse appointment id from output: %s", output)
	}
	return strings.TrimSpace(match[1])
}
