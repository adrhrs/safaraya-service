package main

import (
	"time"

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
