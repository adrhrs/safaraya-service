package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type server struct {
	db *pgxpool.Pool
}

type User struct {
	ID        int64     `json:"id"`
	Name      *string   `json:"name,omitempty"`
	Age       *int      `json:"age,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	HasCV     bool      `json:"-"` // internal flag to build download URL

	CvFileDownloadURL *string `json:"cv_file_download_url,omitempty"`
}

type Registration struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	FullName       string    `json:"full_name"`
	JobTitle       *string   `json:"job_title,omitempty"`
	AddressFull    *string   `json:"address_full,omitempty"`
	WhatsappNumber string    `json:"whatsapp_number"`
	Note           *string   `json:"note,omitempty"`
	ApplicantCount int       `json:"applicant_count"`
	VisaType       *string   `json:"visa_type,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
