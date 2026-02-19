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

func (b *jsonBackend) ScheduleAppointment(patientID string, at time.Time, reason string) (model.Appointment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := b.loadUnlocked()
	if err != nil {
		return model.Appointment{}, err
	}

	trimmedPatientID := strings.TrimSpace(patientID)
	if !existsPatient(data.Patients, trimmedPatientID) {
		return model.Appointment{}, fmt.Errorf("no existe el paciente con id %q", trimmedPatientID)
	}

	trimmedReason, err := validateRequiredReason(reason)
	if err != nil {
		return model.Appointment{}, err
	}

	normalized, err := normalizeAppointmentDateTime(at)
	if err != nil {
		return model.Appointment{}, err
	}

	if hasAppointmentConflict(data.Appointments, normalized, "") {
		return model.Appointment{}, errors.New("ya existe una cita en ese horario")
	}

	appointment := model.Appointment{
		ID:        newID("a"),
		PatientID: trimmedPatientID,
		DateTime:  normalized,
		Reason:    trimmedReason,
		Status:    model.AppointmentStatusScheduled,
		CreatedAt: time.Now().UTC(),
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

	if hasAppointmentConflict(data.Appointments, normalized, target) {
		return model.Appointment{}, errors.New("ya existe una cita en ese horario")
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

func hasAppointmentConflict(appointments []model.Appointment, at time.Time, excludedID string) bool {
	for _, appt := range appointments {
		if appt.ID == excludedID {
			continue
		}
		if !isActiveAppointmentStatus(appt.Status) {
			continue
		}
		if appt.DateTime.UTC().Truncate(time.Minute).Equal(at) {
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

func (b *jsonBackend) loadUnlocked() (model.Data, error) {
	bytes, err := os.ReadFile(b.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Data{}, nil
		}
		return model.Data{}, fmt.Errorf("no se pudo leer %s: %w", b.path, err)
	}

	if len(strings.TrimSpace(string(bytes))) == 0 {
		return model.Data{}, nil
	}

	var data model.Data
	if err := json.Unmarshal(bytes, &data); err != nil {
		return model.Data{}, fmt.Errorf("JSON invalido en %s: %w", b.path, err)
	}
	return data, nil
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
