package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"agenda-pacientes/internal/model"
)

const (
	defaultProfessionalID            = "pr_general"
	defaultProfessionalName          = "General"
	defaultProfessionalPrimaryRole   = "medico"
	defaultProfessionalSecondaryRole = "general"
	defaultServiceID                 = "sv_general"
	defaultServiceName               = "General"
	defaultServiceKind               = "consultorio"
	systemMigrationActor             = "system:migration"
)

var errNoStatusChange = errors.New("sin cambio de estado")

type backend interface {
	AddPatient(name, phone, email string) (model.Patient, error)
	ListPatients() ([]model.Patient, error)
	GetPatientByID(id string) (model.Patient, error)
	UpdatePatient(id, name, phone, email string) (model.Patient, error)
	DeletePatient(id string) error
	SearchPatients(query string) ([]model.Patient, error)

	AddProfessional(name, primaryRole, secondaryRole string) (model.Professional, error)
	ListProfessionals() ([]model.Professional, error)
	GetProfessionalByID(id string) (model.Professional, error)
	UpdateProfessional(id, name, primaryRole, secondaryRole string) (model.Professional, error)
	DeleteProfessional(id string) error
	SearchProfessionals(query string) ([]model.Professional, error)

	AddService(name, kind string) (model.Service, error)
	ListServices() ([]model.Service, error)
	GetServiceByID(id string) (model.Service, error)
	UpdateService(id, name, kind string) (model.Service, error)
	DeleteService(id string) error
	SearchServices(query string) ([]model.Service, error)

	ScheduleAppointment(patientID, professionalID, serviceID string, at time.Time, reason string) (model.Appointment, error)
	RescheduleAppointment(id string, at time.Time) (model.Appointment, error)
	ListAppointments(filters AppointmentFilters) ([]model.Appointment, error)
	SetAppointmentStatus(id, status string) (model.Appointment, error)
}

type phase3BackfillEnsurer interface {
	EnsurePhase3Backfill() (bool, error)
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

func normalizeNameKey(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func normalizeRoleOrKind(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
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

func defaultProfessional() model.Professional {
	return model.Professional{
		ID:            defaultProfessionalID,
		Name:          defaultProfessionalName,
		PrimaryRole:   defaultProfessionalPrimaryRole,
		SecondaryRole: defaultProfessionalSecondaryRole,
		CreatedAt:     time.Now().UTC(),
	}
}

func defaultService() model.Service {
	return model.Service{
		ID:        defaultServiceID,
		Name:      defaultServiceName,
		Kind:      defaultServiceKind,
		CreatedAt: time.Now().UTC(),
	}
}

func ensurePhase3BackfillData(data *model.Data) bool {
	if data == nil {
		return false
	}

	changed := false

	professionalExists := map[string]struct{}{}
	for _, p := range data.Professionals {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		professionalExists[id] = struct{}{}
	}
	if _, ok := professionalExists[defaultProfessionalID]; !ok {
		p := defaultProfessional()
		data.Professionals = append(data.Professionals, p)
		professionalExists[p.ID] = struct{}{}
		changed = true
	}

	serviceExists := map[string]struct{}{}
	for _, s := range data.Services {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		serviceExists[id] = struct{}{}
	}
	if _, ok := serviceExists[defaultServiceID]; !ok {
		s := defaultService()
		data.Services = append(data.Services, s)
		serviceExists[s.ID] = struct{}{}
		changed = true
	}

	for i := range data.Appointments {
		currentProfessionalID := strings.TrimSpace(data.Appointments[i].ProfessionalID)
		if currentProfessionalID == "" {
			data.Appointments[i].ProfessionalID = defaultProfessionalID
			changed = true
		} else if _, ok := professionalExists[currentProfessionalID]; !ok {
			data.Appointments[i].ProfessionalID = defaultProfessionalID
			changed = true
		}

		currentServiceID := strings.TrimSpace(data.Appointments[i].ServiceID)
		if currentServiceID == "" {
			data.Appointments[i].ServiceID = defaultServiceID
			changed = true
		} else if _, ok := serviceExists[currentServiceID]; !ok {
			data.Appointments[i].ServiceID = defaultServiceID
			changed = true
		}
	}

	return changed
}
