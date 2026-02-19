package store

import (
	"fmt"
	"strings"
	"time"

	"agenda-pacientes/internal/model"
)

type Config struct {
	Storage         string
	DataFile        string
	Actor           string
	BackupRetention int
}

type Store struct {
	cfg     Config
	backend backend
	auditor *auditLogger
	initErr error
}

func New(path string) *Store {
	return NewWithConfig(Config{
		Storage:         "json",
		DataFile:        path,
		BackupRetention: 7,
	})
}

func NewWithConfig(cfg Config) *Store {
	normalized := cfg
	if strings.TrimSpace(normalized.Storage) == "" {
		normalized.Storage = "json"
	}
	normalized.Storage = strings.ToLower(strings.TrimSpace(normalized.Storage))
	if strings.TrimSpace(normalized.DataFile) == "" {
		normalized.DataFile = "agenda_data.json"
	}
	if normalized.BackupRetention <= 0 {
		normalized.BackupRetention = 7
	}

	s := &Store{
		cfg:     normalized,
		auditor: newAuditLogger(normalized.DataFile),
	}

	switch normalized.Storage {
	case "json":
		s.backend = newJSONBackend(normalized.DataFile, normalized.BackupRetention)
	case "sqlite":
		b, err := newSQLiteBackend(normalized.DataFile)
		if err != nil {
			s.initErr = err
			return s
		}
		s.backend = b
	default:
		s.initErr = fmt.Errorf("storage invalido %q: usa json o sqlite", normalized.Storage)
	}

	return s
}

func (s *Store) InitError() error {
	return s.initErr
}

func (s *Store) AddPatient(name, phone, email string) (model.Patient, error) {
	if err := s.ensureReady(); err != nil {
		return model.Patient{}, err
	}

	patient, err := s.backend.AddPatient(name, phone, email)
	if err != nil {
		return model.Patient{}, err
	}
	if err := s.logEvent("patient.add", "patient", patient.ID, map[string]any{"name": patient.Name}); err != nil {
		return model.Patient{}, err
	}
	return patient, nil
}

func (s *Store) ListPatients() ([]model.Patient, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	return s.backend.ListPatients()
}

func (s *Store) GetPatientByID(id string) (model.Patient, error) {
	if err := s.ensureReady(); err != nil {
		return model.Patient{}, err
	}
	return s.backend.GetPatientByID(id)
}

func (s *Store) UpdatePatient(id, name, phone, email string) (model.Patient, error) {
	if err := s.ensureReady(); err != nil {
		return model.Patient{}, err
	}

	patient, err := s.backend.UpdatePatient(id, name, phone, email)
	if err != nil {
		return model.Patient{}, err
	}
	if err := s.logEvent("patient.update", "patient", patient.ID, nil); err != nil {
		return model.Patient{}, err
	}
	return patient, nil
}

func (s *Store) DeletePatient(id string) error {
	if err := s.ensureReady(); err != nil {
		return err
	}
	if err := s.backend.DeletePatient(id); err != nil {
		return err
	}
	if err := s.logEvent("patient.delete", "patient", strings.TrimSpace(id), nil); err != nil {
		return err
	}
	return nil
}

func (s *Store) SearchPatients(query string) ([]model.Patient, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	return s.backend.SearchPatients(query)
}

func (s *Store) ScheduleAppointment(patientID string, at time.Time, reason string) (model.Appointment, error) {
	if err := s.ensureReady(); err != nil {
		return model.Appointment{}, err
	}

	appointment, err := s.backend.ScheduleAppointment(patientID, at, reason)
	if err != nil {
		return model.Appointment{}, err
	}
	if err := s.logEvent("appointment.add", "appointment", appointment.ID, map[string]any{"patient_id": appointment.PatientID}); err != nil {
		return model.Appointment{}, err
	}
	return appointment, nil
}

func (s *Store) RescheduleAppointment(id string, at time.Time) (model.Appointment, error) {
	if err := s.ensureReady(); err != nil {
		return model.Appointment{}, err
	}

	appointment, err := s.backend.RescheduleAppointment(id, at)
	if err != nil {
		return model.Appointment{}, err
	}
	if err := s.logEvent("appointment.reschedule", "appointment", appointment.ID, map[string]any{"new_date_time": appointment.DateTime.Format(time.RFC3339)}); err != nil {
		return model.Appointment{}, err
	}
	return appointment, nil
}

func (s *Store) ListAppointments(filters AppointmentFilters) ([]model.Appointment, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	return s.backend.ListAppointments(filters)
}

func (s *Store) CancelAppointment(appointmentID string) error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	appointment, err := s.backend.SetAppointmentStatus(appointmentID, model.AppointmentStatusCanceled)
	if err != nil {
		if err == errNoStatusChange {
			return nil
		}
		return err
	}
	if err := s.logEvent("appointment.cancel", "appointment", appointment.ID, nil); err != nil {
		return err
	}
	return nil
}

func (s *Store) SetAppointmentStatus(id, status string) (model.Appointment, error) {
	if err := s.ensureReady(); err != nil {
		return model.Appointment{}, err
	}

	appointment, err := s.backend.SetAppointmentStatus(id, status)
	if err != nil {
		if err == errNoStatusChange {
			return appointment, nil
		}
		return model.Appointment{}, err
	}
	if err := s.logEvent("appointment.set_status", "appointment", appointment.ID, map[string]any{"status": appointment.Status}); err != nil {
		return model.Appointment{}, err
	}
	return appointment, nil
}

func (s *Store) ensureReady() error {
	if s.initErr != nil {
		return s.initErr
	}
	if s.backend == nil {
		return fmt.Errorf("store no inicializado")
	}
	return nil
}

func (s *Store) logEvent(action, entityType, entityID string, metadata map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	return s.auditor.Log(auditEvent{
		Actor:      s.cfg.Actor,
		Backend:    s.cfg.Storage,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metadata,
	})
}
