package store

import (
	"strings"
	"testing"
	"time"

	"agenda-pacientes/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(t.TempDir() + "/agenda.json")
}

func futureTime(base time.Time, deltaMinutes int) time.Time {
	return base.Add(time.Duration(deltaMinutes) * time.Minute)
}

func scheduleTestAppointment(t *testing.T, s *Store, patientID string, at time.Time, reason string) (model.Appointment, error) {
	t.Helper()
	return s.ScheduleAppointment(patientID, defaultProfessionalID, defaultServiceID, at, reason)
}

func TestAddPatientRejectsDuplicateEmail(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddPatient("Ana", "555-0101", "ana@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	_, err = s.AddPatient("Ana 2", "555-0102", "ANA@mail.com")
	if err == nil {
		t.Fatal("expected duplicate email error")
	}
	if !strings.Contains(err.Error(), "paciente duplicado") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddPatientRejectsDuplicatePhoneNormalized(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddPatient("Laura", "+54 11 1234-5678", "laura@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	_, err = s.AddPatient("Laura 2", "541112345678", "laura2@mail.com")
	if err == nil {
		t.Fatal("expected duplicate phone error")
	}
	if !strings.Contains(err.Error(), "paciente duplicado") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPatientByID(t *testing.T) {
	s := newTestStore(t)
	created, err := s.AddPatient("Pedro", "555-0202", "pedro@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	found, err := s.GetPatientByID(created.ID)
	if err != nil {
		t.Fatalf("expected patient, got error: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("expected %s, got %s", created.ID, found.ID)
	}

	_, err = s.GetPatientByID("p_missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestUpdatePatientPartialAndDuplicateRule(t *testing.T) {
	s := newTestStore(t)
	p1, err := s.AddPatient("Maria", "555-0303", "maria@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	_, err = s.AddPatient("Carlos", "555-0404", "carlos@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	updated, err := s.UpdatePatient(p1.ID, "Maria Gomez", "", "")
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if updated.Name != "Maria Gomez" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	_, err = s.UpdatePatient(p1.ID, "", "", "carlos@mail.com")
	if err == nil {
		t.Fatal("expected duplicate update error")
	}
	if !strings.Contains(err.Error(), "paciente duplicado") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeletePatientRules(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().Add(2 * time.Hour).Truncate(time.Minute)

	p1, err := s.AddPatient("Delete OK", "555-1111", "ok@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	p2, err := s.AddPatient("Delete Block", "555-2222", "block@mail.com")
	if err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}

	appt, err := scheduleTestAppointment(t, s, p2.ID, futureTime(base, 15), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	if err := s.DeletePatient(p1.ID); err != nil {
		t.Fatalf("expected delete success, got error: %v", err)
	}

	err = s.DeletePatient(p2.ID)
	if err == nil {
		t.Fatal("expected delete blocked error")
	}
	if !strings.Contains(err.Error(), "tiene citas activas") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.CancelAppointment(appt.ID); err != nil {
		t.Fatalf("unexpected cancel error: %v", err)
	}
	if err := s.DeletePatient(p2.ID); err != nil {
		t.Fatalf("expected delete after cancel success, got error: %v", err)
	}
}

func TestSearchPatients(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.AddPatient("Carla Diaz", "555-3333", "carla@mail.com")
	_, _ = s.AddPatient("Nora", "555-4444", "nora@mail.com")

	byName, err := s.SearchPatients("carla")
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(byName) != 1 || byName[0].Name != "Carla Diaz" {
		t.Fatalf("unexpected name search result: %+v", byName)
	}

	byPhone, err := s.SearchPatients("(555) 3333")
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(byPhone) != 1 || byPhone[0].Name != "Carla Diaz" {
		t.Fatalf("unexpected phone search result: %+v", byPhone)
	}

	empty, err := s.SearchPatients("zzz")
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no matches, got %+v", empty)
	}
}

func TestScheduleAppointmentValidationAndConflict(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().Add(2 * time.Hour).Truncate(time.Minute)

	p1, _ := s.AddPatient("Paciente 1", "555-5010", "p1@mail.com")
	p2, _ := s.AddPatient("Paciente 2", "555-5011", "p2@mail.com")

	_, err := scheduleTestAppointment(t, s, p1.ID, time.Now().Add(-1*time.Hour), "Control")
	if err == nil || !strings.Contains(err.Error(), "debe ser futura") {
		t.Fatalf("expected past-date error, got: %v", err)
	}

	_, err = scheduleTestAppointment(t, s, p1.ID, futureTime(base, 10), "")
	if err == nil || !strings.Contains(err.Error(), "motivo") {
		t.Fatalf("expected reason-required error, got: %v", err)
	}

	_, err = scheduleTestAppointment(t, s, p1.ID, futureTime(base, 20), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	_, err = scheduleTestAppointment(t, s, p2.ID, futureTime(base, 20), "Consulta")
	if err == nil || !strings.Contains(err.Error(), "ya existe una cita") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestRescheduleAppointment(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().Add(3 * time.Hour).Truncate(time.Minute)

	p1, _ := s.AddPatient("Paciente 1", "555-6010", "r1@mail.com")
	p2, _ := s.AddPatient("Paciente 2", "555-6011", "r2@mail.com")

	a1, err := scheduleTestAppointment(t, s, p1.ID, futureTime(base, 10), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}
	_, err = scheduleTestAppointment(t, s, p2.ID, futureTime(base, 30), "Control")
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	updated, err := s.RescheduleAppointment(a1.ID, futureTime(base, 40))
	if err != nil {
		t.Fatalf("unexpected reschedule error: %v", err)
	}
	if !updated.DateTime.Equal(futureTime(base, 40).UTC().Truncate(time.Minute)) {
		t.Fatalf("unexpected reschedule datetime: %v", updated.DateTime)
	}
	if updated.Status != model.AppointmentStatusScheduled {
		t.Fatalf("expected status programada, got %s", updated.Status)
	}

	_, err = s.RescheduleAppointment(a1.ID, time.Now().Add(-1*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "debe ser futura") {
		t.Fatalf("expected past-date error, got: %v", err)
	}

	_, err = s.RescheduleAppointment(a1.ID, futureTime(base, 30))
	if err == nil || !strings.Contains(err.Error(), "ya existe una cita") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestListAppointmentsWithFilters(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().Add(4 * time.Hour).Truncate(time.Minute)

	p1, _ := s.AddPatient("P1", "555-7010", "f1@mail.com")
	p2, _ := s.AddPatient("P2", "555-7011", "f2@mail.com")

	a1, _ := scheduleTestAppointment(t, s, p1.ID, futureTime(base, 0), "A1")
	a2, _ := scheduleTestAppointment(t, s, p1.ID, futureTime(base, 20), "A2")
	a3, _ := scheduleTestAppointment(t, s, p2.ID, futureTime(base, 40), "A3")
	if err := s.CancelAppointment(a2.ID); err != nil {
		t.Fatalf("unexpected cancel error: %v", err)
	}

	allActive, err := s.ListAppointments(AppointmentFilters{})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(allActive) != 2 {
		t.Fatalf("expected 2 active appointments, got %d", len(allActive))
	}

	allWithCanceled, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(allWithCanceled) != 3 {
		t.Fatalf("expected 3 appointments with canceled, got %d", len(allWithCanceled))
	}

	byStatus, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true, Status: model.AppointmentStatusCanceled})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].ID != a2.ID {
		t.Fatalf("unexpected status filter result: %+v", byStatus)
	}

	byPatient, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true, PatientID: p1.ID})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(byPatient) != 2 {
		t.Fatalf("expected 2 appointments for patient, got %d", len(byPatient))
	}

	from := a1.DateTime
	to := a1.DateTime
	inclusive, err := s.ListAppointments(AppointmentFilters{IncludeCanceled: true, From: &from, To: &to})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(inclusive) != 1 || inclusive[0].ID != a1.ID {
		t.Fatalf("expected inclusive range to return only a1, got %+v", inclusive)
	}

	_ = a3
}
