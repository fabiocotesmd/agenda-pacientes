package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"agenda-pacientes/internal/model"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) AddPatient(name, phone, email string) (model.Patient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadUnlocked()
	if err != nil {
		return model.Patient{}, err
	}

	patient := model.Patient{
		ID:        newID("p"),
		Name:      strings.TrimSpace(name),
		Phone:     strings.TrimSpace(phone),
		Email:     strings.TrimSpace(email),
		CreatedAt: time.Now().UTC(),
	}
	data.Patients = append(data.Patients, patient)

	if err := s.saveUnlocked(data); err != nil {
		return model.Patient{}, err
	}
	return patient, nil
}

func (s *Store) ListPatients() ([]model.Patient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}

	patients := append([]model.Patient(nil), data.Patients...)
	sort.Slice(patients, func(i, j int) bool {
		return patients[i].CreatedAt.Before(patients[j].CreatedAt)
	})
	return patients, nil
}

func (s *Store) ScheduleAppointment(patientID string, at time.Time, reason string) (model.Appointment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadUnlocked()
	if err != nil {
		return model.Appointment{}, err
	}

	trimmedPatientID := strings.TrimSpace(patientID)
	if !existsPatient(data.Patients, trimmedPatientID) {
		return model.Appointment{}, fmt.Errorf("no existe el paciente con id %q", trimmedPatientID)
	}

	normalized := at.UTC().Truncate(time.Minute)
	for _, appt := range data.Appointments {
		if appt.Status == model.AppointmentStatusCanceled {
			continue
		}
		if appt.DateTime.UTC().Truncate(time.Minute).Equal(normalized) {
			return model.Appointment{}, errors.New("ya existe una cita en ese horario")
		}
	}

	appointment := model.Appointment{
		ID:        newID("a"),
		PatientID: trimmedPatientID,
		DateTime:  normalized,
		Reason:    strings.TrimSpace(reason),
		Status:    model.AppointmentStatusScheduled,
		CreatedAt: time.Now().UTC(),
	}
	data.Appointments = append(data.Appointments, appointment)

	if err := s.saveUnlocked(data); err != nil {
		return model.Appointment{}, err
	}
	return appointment, nil
}

func (s *Store) ListAppointments(includeCanceled bool) ([]model.Appointment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}

	filtered := make([]model.Appointment, 0, len(data.Appointments))
	for _, appt := range data.Appointments {
		if !includeCanceled && appt.Status == model.AppointmentStatusCanceled {
			continue
		}
		filtered = append(filtered, appt)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DateTime.Before(filtered[j].DateTime)
	})

	return filtered, nil
}

func (s *Store) CancelAppointment(appointmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadUnlocked()
	if err != nil {
		return err
	}

	target := strings.TrimSpace(appointmentID)
	for i := range data.Appointments {
		if data.Appointments[i].ID != target {
			continue
		}
		if data.Appointments[i].Status == model.AppointmentStatusCanceled {
			return fmt.Errorf("la cita %q ya estaba cancelada", target)
		}
		data.Appointments[i].Status = model.AppointmentStatusCanceled
		return s.saveUnlocked(data)
	}

	return fmt.Errorf("no se encontro la cita con id %q", target)
}

func existsPatient(patients []model.Patient, id string) bool {
	for _, p := range patients {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) loadUnlocked() (model.Data, error) {
	bytes, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Data{}, nil
		}
		return model.Data{}, fmt.Errorf("no se pudo leer %s: %w", s.path, err)
	}

	if len(strings.TrimSpace(string(bytes))) == 0 {
		return model.Data{}, nil
	}

	var data model.Data
	if err := json.Unmarshal(bytes, &data); err != nil {
		return model.Data{}, fmt.Errorf("JSON invalido en %s: %w", s.path, err)
	}
	return data, nil
}

func (s *Store) saveUnlocked(data model.Data) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("no se pudo serializar datos: %w", err)
	}

	if err := os.WriteFile(s.path, bytes, 0o644); err != nil {
		return fmt.Errorf("no se pudo guardar %s: %w", s.path, err)
	}

	return nil
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%x", prefix, time.Now().UTC().UnixNano())
}
