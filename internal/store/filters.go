package store

import "time"

type AppointmentFilters struct {
	IncludeCanceled bool
	From            *time.Time
	To              *time.Time
	PatientID       string
	ProfessionalID  string
	ServiceID       string
	Status          string
}
