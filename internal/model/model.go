package model

import "time"

const (
	AppointmentStatusScheduled = "programada"
	AppointmentStatusCanceled  = "cancelada"
)

type Patient struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Appointment struct {
	ID        string    `json:"id"`
	PatientID string    `json:"patient_id"`
	DateTime  time.Time `json:"date_time"`
	Reason    string    `json:"reason"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Data struct {
	Patients     []Patient     `json:"patients"`
	Appointments []Appointment `json:"appointments"`
}
