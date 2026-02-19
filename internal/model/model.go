package model

import "time"

const (
	AppointmentStatusScheduled = "programada"
	AppointmentStatusConfirmed = "confirmada"
	AppointmentStatusAttended  = "atendida"
	AppointmentStatusNoShow    = "ausente"
	AppointmentStatusCanceled  = "cancelada"
)

type Patient struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Professional struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	PrimaryRole   string    `json:"primary_role"`
	SecondaryRole string    `json:"secondary_role"`
	CreatedAt     time.Time `json:"created_at"`
}

type Service struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

type Appointment struct {
	ID             string    `json:"id"`
	PatientID      string    `json:"patient_id"`
	ProfessionalID string    `json:"professional_id"`
	ServiceID      string    `json:"service_id"`
	DateTime       time.Time `json:"date_time"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type Data struct {
	Patients      []Patient      `json:"patients"`
	Professionals []Professional `json:"professionals"`
	Services      []Service      `json:"services"`
	Appointments  []Appointment  `json:"appointments"`
}
