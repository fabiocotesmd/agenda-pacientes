package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"agenda-pacientes/internal/model"
)

var errNoStatusChange = errors.New("sin cambio de estado")

type backend interface {
	AddPatient(name, phone, email string) (model.Patient, error)
	ListPatients() ([]model.Patient, error)
	GetPatientByID(id string) (model.Patient, error)
	UpdatePatient(id, name, phone, email string) (model.Patient, error)
	DeletePatient(id string) error
	SearchPatients(query string) ([]model.Patient, error)
	ScheduleAppointment(patientID string, at time.Time, reason string) (model.Appointment, error)
	RescheduleAppointment(id string, at time.Time) (model.Appointment, error)
	ListAppointments(filters AppointmentFilters) ([]model.Appointment, error)
	SetAppointmentStatus(id, status string) (model.Appointment, error)
}

func validateRequiredReason(reason string) (string, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "", errors.New("el motivo de la cita es obligatorio")
	}
	return trimmed, nil
}

func ensureFutureDateTime(at time.Time) error {
	if at.IsZero() {
		return errors.New("fecha invalida")
	}
	if !at.After(time.Now()) {
		return errors.New("la fecha de la cita debe ser futura")
	}
	return nil
}

func normalizeAppointmentDateTime(at time.Time) (time.Time, error) {
	if err := ensureFutureDateTime(at); err != nil {
		return time.Time{}, err
	}
	return at.UTC().Truncate(time.Minute), nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePhoneDigits(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateStatus(status string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case model.AppointmentStatusScheduled,
		model.AppointmentStatusConfirmed,
		model.AppointmentStatusAttended,
		model.AppointmentStatusNoShow,
		model.AppointmentStatusCanceled:
		return normalized, nil
	default:
		return "", fmt.Errorf("estado de cita invalido: %q", status)
	}
}

func validateStatusTransition(current, next string) error {
	currentStatus, err := validateStatus(current)
	if err != nil {
		return err
	}
	nextStatus, err := validateStatus(next)
	if err != nil {
		return err
	}

	if currentStatus == nextStatus {
		return nil
	}

	switch currentStatus {
	case model.AppointmentStatusScheduled:
		if nextStatus == model.AppointmentStatusConfirmed ||
			nextStatus == model.AppointmentStatusNoShow ||
			nextStatus == model.AppointmentStatusCanceled {
			return nil
		}
	case model.AppointmentStatusConfirmed:
		if nextStatus == model.AppointmentStatusAttended ||
			nextStatus == model.AppointmentStatusNoShow ||
			nextStatus == model.AppointmentStatusCanceled {
			return nil
		}
	}

	return fmt.Errorf("transicion de estado invalida: %s -> %s", currentStatus, nextStatus)
}

func isActiveAppointmentStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == model.AppointmentStatusScheduled || normalized == model.AppointmentStatusConfirmed
}

func canRescheduleStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == model.AppointmentStatusScheduled || normalized == model.AppointmentStatusConfirmed
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%x", prefix, time.Now().UTC().UnixNano())
}
