package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agenda-pacientes/internal/model"
)

type jsonBackend struct {
	path            string
	backupRetention int
	mu              sync.Mutex
}

func newJSONBackend(path string, backupRetention int) backend {
	if backupRetention <= 0 {
		backupRetention = 7
	}
	return &jsonBackend{
		path:            path,
		backupRetention: backupRetention,
	}
}

func (b *jsonBackend) EnsurePhase3Backfill() (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, changed, err := b.loadBackfilledUnlocked()
	return changed, err
}

func (b *jsonBackend) AddPatient(name, phone, email string) (model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Patient{}, err
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return model.Patient{}, errors.New("el nombre es obligatorio")
	}

	trimmedPhone := strings.TrimSpace(phone)
	trimmedEmail := strings.TrimSpace(email)

	if dup := findDuplicatePatient(data.Patients, trimmedEmail, trimmedPhone, ""); dup != nil {
		return model.Patient{}, fmt.Errorf("paciente duplicado: coincide con %q", dup.ID)
	}

	patient := model.Patient{
		ID:        newID("p"),
		Name:      trimmedName,
		Phone:     trimmedPhone,
		Email:     trimmedEmail,
		CreatedAt: time.Now().UTC(),
	}
	data.Patients = append(data.Patients, patient)

	if err := b.saveUnlocked(data); err != nil {
		return model.Patient{}, err
	}
	return patient, nil
}

func (b *jsonBackend) ListPatients() ([]model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	patients := append([]model.Patient(nil), data.Patients...)
	sort.Slice(patients, func(i, j int) bool {
		return patients[i].CreatedAt.Before(patients[j].CreatedAt)
	})
	return patients, nil
}

func (b *jsonBackend) GetPatientByID(id string) (model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Patient{}, err
	}

	target := strings.TrimSpace(id)
	for _, patient := range data.Patients {
		if patient.ID == target {
			return patient, nil
		}
	}

	return model.Patient{}, fmt.Errorf("no se encontro el paciente con id %q", target)
}

func (b *jsonBackend) UpdatePatient(id, name, phone, email string) (model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Patient{}, err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Patient{}, errors.New("id de paciente obligatorio")
	}

	idx := -1
	for i := range data.Patients {
		if data.Patients[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return model.Patient{}, fmt.Errorf("no se encontro el paciente con id %q", target)
	}

	trimmedName := strings.TrimSpace(name)
	trimmedPhone := strings.TrimSpace(phone)
	trimmedEmail := strings.TrimSpace(email)

	if trimmedName == "" && trimmedPhone == "" && trimmedEmail == "" {
		return model.Patient{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	updated := data.Patients[idx]
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedPhone != "" {
		updated.Phone = trimmedPhone
	}
	if trimmedEmail != "" {
		updated.Email = trimmedEmail
	}

	if dup := findDuplicatePatient(data.Patients, updated.Email, updated.Phone, updated.ID); dup != nil {
		return model.Patient{}, fmt.Errorf("paciente duplicado: coincide con %q", dup.ID)
	}

	data.Patients[idx] = updated
	if err := b.saveUnlocked(data); err != nil {
		return model.Patient{}, err
	}

	return updated, nil
}

func (b *jsonBackend) DeletePatient(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de paciente obligatorio")
	}

	idx := -1
	for i := range data.Patients {
		if data.Patients[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no se encontro el paciente con id %q", target)
	}

	for _, appt := range data.Appointments {
		if appt.PatientID == target && isActiveAppointmentStatus(appt.Status) {
			return fmt.Errorf("no se puede eliminar el paciente %q porque tiene citas activas", target)
		}
	}

	data.Patients = append(data.Patients[:idx], data.Patients[idx+1:]...)
	filteredAppointments := make([]model.Appointment, 0, len(data.Appointments))
	for _, appt := range data.Appointments {
		if appt.PatientID != target {
			filteredAppointments = append(filteredAppointments, appt)
		}
	}
	data.Appointments = filteredAppointments

	return b.saveUnlocked(data)
}

func (b *jsonBackend) SearchPatients(query string) ([]model.Patient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	qPhone := normalizePhoneDigits(query)

	results := make([]model.Patient, 0, len(data.Patients))
	for _, patient := range data.Patients {
		if q == "" {
			results = append(results, patient)
			continue
		}

		if strings.Contains(strings.ToLower(patient.Name), q) ||
			strings.Contains(strings.ToLower(patient.Email), q) ||
			(qPhone != "" && strings.Contains(normalizePhoneDigits(patient.Phone), qPhone)) {
			results = append(results, patient)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results, nil
}

func (b *jsonBackend) AddProfessional(name, primaryRole, secondaryRole string) (model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Professional{}, err
	}

	trimmedName := normalizeRoleOrKind(name)
	if trimmedName == "" {
		return model.Professional{}, errors.New("el nombre del profesional es obligatorio")
	}
	trimmedPrimaryRole := normalizeRoleOrKind(primaryRole)
	if trimmedPrimaryRole == "" {
		return model.Professional{}, errors.New("primary-role es obligatorio")
	}
	trimmedSecondaryRole := normalizeRoleOrKind(secondaryRole)

	if dup := findDuplicateProfessional(data.Professionals, trimmedName, ""); dup != nil {
		return model.Professional{}, fmt.Errorf("profesional duplicado: coincide con %q", dup.ID)
	}

	professional := model.Professional{
		ID:            newID("pr"),
		Name:          trimmedName,
		PrimaryRole:   trimmedPrimaryRole,
		SecondaryRole: trimmedSecondaryRole,
		CreatedAt:     time.Now().UTC(),
	}
	data.Professionals = append(data.Professionals, professional)

	if err := b.saveUnlocked(data); err != nil {
		return model.Professional{}, err
	}
	return professional, nil
}

func (b *jsonBackend) ListProfessionals() ([]model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	out := append([]model.Professional(nil), data.Professionals...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (b *jsonBackend) GetProfessionalByID(id string) (model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Professional{}, err
	}

	target := strings.TrimSpace(id)
	for _, professional := range data.Professionals {
		if professional.ID == target {
			return professional, nil
		}
	}

	return model.Professional{}, fmt.Errorf("no se encontro el profesional con id %q", target)
}

func (b *jsonBackend) UpdateProfessional(id, name, primaryRole, secondaryRole string) (model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Professional{}, err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Professional{}, errors.New("id de profesional obligatorio")
	}

	idx := -1
	for i := range data.Professionals {
		if data.Professionals[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return model.Professional{}, fmt.Errorf("no se encontro el profesional con id %q", target)
	}

	trimmedName := normalizeRoleOrKind(name)
	trimmedPrimaryRole := normalizeRoleOrKind(primaryRole)
	trimmedSecondaryRole := normalizeRoleOrKind(secondaryRole)

	if trimmedName == "" && trimmedPrimaryRole == "" && trimmedSecondaryRole == "" {
		return model.Professional{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	updated := data.Professionals[idx]
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedPrimaryRole != "" {
		updated.PrimaryRole = trimmedPrimaryRole
	}
	if trimmedSecondaryRole != "" {
		updated.SecondaryRole = trimmedSecondaryRole
	}

	if strings.TrimSpace(updated.Name) == "" {
		return model.Professional{}, errors.New("el nombre del profesional es obligatorio")
	}
	if strings.TrimSpace(updated.PrimaryRole) == "" {
		return model.Professional{}, errors.New("primary-role es obligatorio")
	}

	if dup := findDuplicateProfessional(data.Professionals, updated.Name, updated.ID); dup != nil {
		return model.Professional{}, fmt.Errorf("profesional duplicado: coincide con %q", dup.ID)
	}

	data.Professionals[idx] = updated
	if err := b.saveUnlocked(data); err != nil {
		return model.Professional{}, err
	}
	return updated, nil
}

func (b *jsonBackend) DeleteProfessional(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de profesional obligatorio")
	}
	if target == defaultProfessionalID {
		return errors.New("no se puede eliminar el profesional por defecto")
	}

	idx := -1
	for i := range data.Professionals {
		if data.Professionals[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no se encontro el profesional con id %q", target)
	}

	for _, appt := range data.Appointments {
		if appt.ProfessionalID == target && isActiveAppointmentStatus(appt.Status) {
			return fmt.Errorf("no se puede eliminar el profesional %q porque tiene citas activas", target)
		}
	}

	data.Professionals = append(data.Professionals[:idx], data.Professionals[idx+1:]...)
	filteredAppointments := make([]model.Appointment, 0, len(data.Appointments))
	for _, appt := range data.Appointments {
		if appt.ProfessionalID != target {
			filteredAppointments = append(filteredAppointments, appt)
		}
	}
	data.Appointments = filteredAppointments
	return b.saveUnlocked(data)
}

func (b *jsonBackend) SearchProfessionals(query string) ([]model.Professional, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	results := make([]model.Professional, 0, len(data.Professionals))
	for _, professional := range data.Professionals {
		if q == "" {
			results = append(results, professional)
			continue
		}

		if strings.Contains(strings.ToLower(professional.Name), q) ||
			strings.Contains(strings.ToLower(professional.PrimaryRole), q) ||
			strings.Contains(strings.ToLower(professional.SecondaryRole), q) {
			results = append(results, professional)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

func (b *jsonBackend) AddService(name, kind string) (model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Service{}, err
	}

	trimmedName := normalizeRoleOrKind(name)
	if trimmedName == "" {
		return model.Service{}, errors.New("el nombre del servicio es obligatorio")
	}
	trimmedKind := normalizeRoleOrKind(kind)
	if trimmedKind == "" {
		return model.Service{}, errors.New("kind es obligatorio")
	}

	if dup := findDuplicateService(data.Services, trimmedName, ""); dup != nil {
		return model.Service{}, fmt.Errorf("servicio duplicado: coincide con %q", dup.ID)
	}

	service := model.Service{
		ID:        newID("sv"),
		Name:      trimmedName,
		Kind:      trimmedKind,
		CreatedAt: time.Now().UTC(),
	}
	data.Services = append(data.Services, service)

	if err := b.saveUnlocked(data); err != nil {
		return model.Service{}, err
	}
	return service, nil
}

func (b *jsonBackend) ListServices() ([]model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	out := append([]model.Service(nil), data.Services...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (b *jsonBackend) GetServiceByID(id string) (model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Service{}, err
	}

	target := strings.TrimSpace(id)
	for _, service := range data.Services {
		if service.ID == target {
			return service, nil
		}
	}
	return model.Service{}, fmt.Errorf("no se encontro el servicio con id %q", target)
}

func (b *jsonBackend) UpdateService(id, name, kind string) (model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Service{}, err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Service{}, errors.New("id de servicio obligatorio")
	}

	idx := -1
	for i := range data.Services {
		if data.Services[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return model.Service{}, fmt.Errorf("no se encontro el servicio con id %q", target)
	}

	trimmedName := normalizeRoleOrKind(name)
	trimmedKind := normalizeRoleOrKind(kind)

	if trimmedName == "" && trimmedKind == "" {
		return model.Service{}, errors.New("debe proporcionar al menos un campo para actualizar")
	}

	updated := data.Services[idx]
	if trimmedName != "" {
		updated.Name = trimmedName
	}
	if trimmedKind != "" {
		updated.Kind = trimmedKind
	}

	if strings.TrimSpace(updated.Name) == "" {
		return model.Service{}, errors.New("el nombre del servicio es obligatorio")
	}
	if strings.TrimSpace(updated.Kind) == "" {
		return model.Service{}, errors.New("kind es obligatorio")
	}

	if dup := findDuplicateService(data.Services, updated.Name, updated.ID); dup != nil {
		return model.Service{}, fmt.Errorf("servicio duplicado: coincide con %q", dup.ID)
	}

	data.Services[idx] = updated
	if err := b.saveUnlocked(data); err != nil {
		return model.Service{}, err
	}
	return updated, nil
}

func (b *jsonBackend) DeleteService(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("id de servicio obligatorio")
	}
	if target == defaultServiceID {
		return errors.New("no se puede eliminar el servicio por defecto")
	}

	idx := -1
	for i := range data.Services {
		if data.Services[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no se encontro el servicio con id %q", target)
	}

	for _, appt := range data.Appointments {
		if appt.ServiceID == target && isActiveAppointmentStatus(appt.Status) {
			return fmt.Errorf("no se puede eliminar el servicio %q porque tiene citas activas", target)
		}
	}

	data.Services = append(data.Services[:idx], data.Services[idx+1:]...)
	filteredAppointments := make([]model.Appointment, 0, len(data.Appointments))
	for _, appt := range data.Appointments {
		if appt.ServiceID != target {
			filteredAppointments = append(filteredAppointments, appt)
		}
	}
	data.Appointments = filteredAppointments
	return b.saveUnlocked(data)
}

func (b *jsonBackend) SearchServices(query string) ([]model.Service, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	results := make([]model.Service, 0, len(data.Services))
	for _, service := range data.Services {
		if q == "" {
			results = append(results, service)
			continue
		}

		if strings.Contains(strings.ToLower(service.Name), q) ||
			strings.Contains(strings.ToLower(service.Kind), q) {
			results = append(results, service)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

func (b *jsonBackend) ScheduleAppointment(patientID, professionalID, serviceID string, at time.Time, reason string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Appointment{}, err
	}

	trimmedPatientID := strings.TrimSpace(patientID)
	if trimmedPatientID == "" {
		return model.Appointment{}, errors.New("patient-id es obligatorio")
	}
	if !existsPatient(data.Patients, trimmedPatientID) {
		return model.Appointment{}, fmt.Errorf("no existe el paciente con id %q", trimmedPatientID)
	}

	trimmedProfessionalID := strings.TrimSpace(professionalID)
	if trimmedProfessionalID == "" {
		return model.Appointment{}, errors.New("professional-id es obligatorio")
	}
	if !existsProfessional(data.Professionals, trimmedProfessionalID) {
		return model.Appointment{}, fmt.Errorf("no existe el profesional con id %q", trimmedProfessionalID)
	}

	trimmedServiceID := strings.TrimSpace(serviceID)
	if trimmedServiceID == "" {
		return model.Appointment{}, errors.New("service-id es obligatorio")
	}
	if !existsService(data.Services, trimmedServiceID) {
		return model.Appointment{}, fmt.Errorf("no existe el servicio con id %q", trimmedServiceID)
	}

	trimmedReason, err := validateRequiredReason(reason)
	if err != nil {
		return model.Appointment{}, err
	}

	normalized, err := normalizeAppointmentDateTime(at)
	if err != nil {
		return model.Appointment{}, err
	}

	if hasAppointmentConflict(data.Appointments, normalized, trimmedProfessionalID, trimmedServiceID, "") {
		return model.Appointment{}, errors.New("ya existe una cita en conflicto para ese profesional o servicio")
	}

	appointment := model.Appointment{
		ID:             newID("a"),
		PatientID:      trimmedPatientID,
		ProfessionalID: trimmedProfessionalID,
		ServiceID:      trimmedServiceID,
		DateTime:       normalized,
		Reason:         trimmedReason,
		Status:         model.AppointmentStatusScheduled,
		CreatedAt:      time.Now().UTC(),
	}
	data.Appointments = append(data.Appointments, appointment)

	if err := b.saveUnlocked(data); err != nil {
		return model.Appointment{}, err
	}
	return appointment, nil
}

func (b *jsonBackend) RescheduleAppointment(id string, at time.Time) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Appointment{}, err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return model.Appointment{}, errors.New("id de cita obligatorio")
	}

	normalized, err := normalizeAppointmentDateTime(at)
	if err != nil {
		return model.Appointment{}, err
	}

	idx := -1
	for i := range data.Appointments {
		if data.Appointments[i].ID == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return model.Appointment{}, fmt.Errorf("no se encontro la cita con id %q", target)
	}

	if !canRescheduleStatus(data.Appointments[idx].Status) {
		return model.Appointment{}, fmt.Errorf("no se puede reprogramar una cita en estado %q", data.Appointments[idx].Status)
	}

	if hasAppointmentConflict(
		data.Appointments,
		normalized,
		data.Appointments[idx].ProfessionalID,
		data.Appointments[idx].ServiceID,
		target,
	) {
		return model.Appointment{}, errors.New("ya existe una cita en conflicto para ese profesional o servicio")
	}

	data.Appointments[idx].DateTime = normalized
	if err := b.saveUnlocked(data); err != nil {
		return model.Appointment{}, err
	}

	return data.Appointments[idx], nil
}

func (b *jsonBackend) ListAppointments(filters AppointmentFilters) ([]model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return nil, err
	}

	trimmedPatientID := strings.TrimSpace(filters.PatientID)
	trimmedProfessionalID := strings.TrimSpace(filters.ProfessionalID)
	trimmedServiceID := strings.TrimSpace(filters.ServiceID)
	trimmedStatus := strings.ToLower(strings.TrimSpace(filters.Status))

	var fromUTC *time.Time
	if filters.From != nil {
		v := filters.From.UTC().Truncate(time.Minute)
		fromUTC = &v
	}

	var toUTC *time.Time
	if filters.To != nil {
		v := filters.To.UTC().Truncate(time.Minute)
		toUTC = &v
	}

	filtered := make([]model.Appointment, 0, len(data.Appointments))
	for _, appt := range data.Appointments {
		if !filters.IncludeCanceled && strings.ToLower(appt.Status) == model.AppointmentStatusCanceled {
			continue
		}
		if trimmedPatientID != "" && appt.PatientID != trimmedPatientID {
			continue
		}
		if trimmedProfessionalID != "" && appt.ProfessionalID != trimmedProfessionalID {
			continue
		}
		if trimmedServiceID != "" && appt.ServiceID != trimmedServiceID {
			continue
		}
		if trimmedStatus != "" && strings.ToLower(appt.Status) != trimmedStatus {
			continue
		}

		normalizedApptTime := appt.DateTime.UTC().Truncate(time.Minute)
		if fromUTC != nil && normalizedApptTime.Before(*fromUTC) {
			continue
		}
		if toUTC != nil && normalizedApptTime.After(*toUTC) {
			continue
		}

		filtered = append(filtered, appt)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DateTime.Before(filtered[j].DateTime)
	})

	return filtered, nil
}

func (b *jsonBackend) SetAppointmentStatus(id, status string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Appointment{}, err
	}

	targetID := strings.TrimSpace(id)
	if targetID == "" {
		return model.Appointment{}, errors.New("id de cita obligatorio")
	}

	targetStatus, err := validateStatus(status)
	if err != nil {
		return model.Appointment{}, err
	}

	idx := -1
	for i := range data.Appointments {
		if data.Appointments[i].ID == targetID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return model.Appointment{}, fmt.Errorf("no se encontro la cita con id %q", targetID)
	}

	currentStatus := strings.ToLower(strings.TrimSpace(data.Appointments[idx].Status))
	if err := validateStatusTransition(currentStatus, targetStatus); err != nil {
		return model.Appointment{}, err
	}

	if currentStatus == targetStatus {
		return data.Appointments[idx], errNoStatusChange
	}

	data.Appointments[idx].Status = targetStatus
	if err := b.saveUnlocked(data); err != nil {
		return model.Appointment{}, err
	}
	return data.Appointments[idx], nil
}

func existsPatient(patients []model.Patient, id string) bool {
	for _, patient := range patients {
		if patient.ID == id {
			return true
		}
	}
	return false
}

func existsProfessional(professionals []model.Professional, id string) bool {
	for _, professional := range professionals {
		if professional.ID == id {
			return true
		}
	}
	return false
}

func existsService(services []model.Service, id string) bool {
	for _, service := range services {
		if service.ID == id {
			return true
		}
	}
	return false
}

func hasAppointmentConflict(
	appointments []model.Appointment,
	at time.Time,
	professionalID string,
	serviceID string,
	excludedID string,
) bool {
	for _, appt := range appointments {
		if appt.ID == excludedID {
			continue
		}
		if !isActiveAppointmentStatus(appt.Status) {
			continue
		}
		if !appt.DateTime.UTC().Truncate(time.Minute).Equal(at) {
			continue
		}
		if appt.ProfessionalID == professionalID || appt.ServiceID == serviceID {
			return true
		}
	}
	return false
}

func findDuplicatePatient(patients []model.Patient, email, phone, excludeID string) *model.Patient {
	normEmail := normalizeEmail(email)
	normPhone := normalizePhoneDigits(phone)

	if normEmail == "" && normPhone == "" {
		return nil
	}

	for i := range patients {
		if patients[i].ID == excludeID {
			continue
		}

		existingEmail := normalizeEmail(patients[i].Email)
		existingPhone := normalizePhoneDigits(patients[i].Phone)

		if normEmail != "" && existingEmail == normEmail {
			return &patients[i]
		}
		if normPhone != "" && existingPhone == normPhone {
			return &patients[i]
		}
	}
	return nil
}

func findDuplicateProfessional(professionals []model.Professional, name, excludeID string) *model.Professional {
	normName := normalizeNameKey(name)
	if normName == "" {
		return nil
	}

	for i := range professionals {
		if professionals[i].ID == excludeID {
			continue
		}
		if normalizeNameKey(professionals[i].Name) == normName {
			return &professionals[i]
		}
	}
	return nil
}

func findDuplicateService(services []model.Service, name, excludeID string) *model.Service {
	normName := normalizeNameKey(name)
	if normName == "" {
		return nil
	}

	for i := range services {
		if services[i].ID == excludeID {
			continue
		}
		if normalizeNameKey(services[i].Name) == normName {
			return &services[i]
		}
	}
	return nil
}

func (b *jsonBackend) loadUnlocked() (model.Data, error) {
	data, _, err := b.loadBackfilledUnlocked()
	return data, err
}

func (b *jsonBackend) loadBackfilledUnlocked() (model.Data, bool, error) {
	bytes, err := os.ReadFile(b.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			data := model.Data{}
			changed := ensurePhase3BackfillData(&data)
			return data, changed, nil
		}
		return model.Data{}, false, fmt.Errorf("no se pudo leer %s: %w", b.path, err)
	}

	data := model.Data{}
	if len(strings.TrimSpace(string(bytes))) != 0 {
		if err := json.Unmarshal(bytes, &data); err != nil {
			return model.Data{}, false, fmt.Errorf("JSON invalido en %s: %w", b.path, err)
		}
	}

	changed := ensurePhase3BackfillData(&data)
	if changed {
		if err := b.saveUnlocked(data); err != nil {
			return model.Data{}, false, err
		}
	}

	return data, changed, nil
}

func (b *jsonBackend) saveUnlocked(data model.Data) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("no se pudo serializar datos: %w", err)
	}

	if err := b.createBackupUnlocked(); err != nil {
		return err
	}

	dir := filepath.Dir(b.path)
	if dir == "" {
		dir = "."
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("no se pudo crear directorio de datos %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(b.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("no se pudo crear archivo temporal: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(bytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo escribir archivo temporal: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo sincronizar archivo temporal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("no se pudo cerrar archivo temporal: %w", err)
	}

	if err := os.Rename(tmpPath, b.path); err != nil {
		return fmt.Errorf("no se pudo reemplazar %s: %w", b.path, err)
	}
	cleanup = false

	syncDirBestEffort(dir)
	return nil
}

func (b *jsonBackend) createBackupUnlocked() error {
	existing, err := os.ReadFile(b.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("no se pudo leer archivo para backup %s: %w", b.path, err)
	}

	if len(strings.TrimSpace(string(existing))) == 0 {
		return nil
	}

	dir := filepath.Dir(b.path)
	if dir == "" {
		dir = "."
	}

	base := filepath.Base(b.path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "agenda_data"
	}

	backupDir := filepath.Join(dir, name+".backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("no se pudo crear directorio de backups %s: %w", backupDir, err)
	}

	now := time.Now().UTC()
	backupName := fmt.Sprintf("%s_%s_%03d.json", name, now.Format("20060102_150405"), now.Nanosecond()/1_000_000)
	backupPath := filepath.Join(backupDir, backupName)

	if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
		return fmt.Errorf("no se pudo crear backup %s: %w", backupPath, err)
	}

	if err := b.rotateBackupsUnlocked(backupDir); err != nil {
		return err
	}
	return nil
}

func (b *jsonBackend) rotateBackupsUnlocked(backupDir string) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("no se pudo listar backups en %s: %w", backupDir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, entry.Name())
	}

	if len(files) <= b.backupRetention {
		return nil
	}

	sort.Strings(files)
	toDelete := files[:len(files)-b.backupRetention]
	for _, file := range toDelete {
		if err := os.Remove(filepath.Join(backupDir, file)); err != nil {
			return fmt.Errorf("no se pudo eliminar backup antiguo %s: %w", file, err)
		}
	}
	return nil
}

func syncDirBestEffort(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() {
		_ = d.Close()
	}()
	_ = d.Sync()
}
