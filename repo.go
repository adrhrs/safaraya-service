package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	errUserNotFound         = errors.New("user not found")
	errRegistrationNotFound = errors.New("registration not found")
	errFileNotFound         = errors.New("file not found")
)

func (s *server) fetchUsers(ctx context.Context) ([]User, error) {
	start := time.Now()
	log.Println("fetchUsers: running SELECT id, name, age, created_at, cv_file IS NOT NULL FROM users")
	rows, err := s.db.Query(ctx, `SELECT id, name, age, created_at, cv_file IS NOT NULL AS has_cv FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var (
			u    User
			name sql.NullString
			age  sql.NullInt32
			cv   bool
		)

		if err := rows.Scan(&u.ID, &name, &age, &u.CreatedAt, &cv); err != nil {
			return nil, err
		}

		if name.Valid {
			u.Name = &name.String
		}

		if age.Valid {
			v := int(age.Int32)
			u.Age = &v
		}

		u.HasCV = cv
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("fetchUsers: fetched %d rows in %s", len(users), time.Since(start).String())
	return users, nil
}

func (s *server) insertUser(ctx context.Context, req createUserRequest) (User, error) {
	start := time.Now()
	log.Println("insertUser: running INSERT INTO users (name, age) VALUES ($1, $2) RETURNING id, name, age, created_at")

	row := s.db.QueryRow(ctx, `INSERT INTO users (name, age) VALUES ($1, $2) RETURNING id, name, age, created_at`, req.Name, req.Age)

	var (
		u    User
		name sql.NullString
		age  sql.NullInt32
	)

	if err := row.Scan(&u.ID, &name, &age, &u.CreatedAt); err != nil {
		return User{}, err
	}

	if name.Valid {
		u.Name = &name.String
	}

	if age.Valid {
		v := int(age.Int32)
		u.Age = &v
	}

	log.Printf("insertUser: inserted id=%d in %s", u.ID, time.Since(start).String())
	return u, nil
}

func (s *server) saveUserCV(ctx context.Context, userID int64, cvData []byte) error {
	start := time.Now()
	log.Println("saveUserCV: running UPDATE users SET cv_file")

	tag, err := s.db.Exec(ctx, `UPDATE users SET cv_file = $2 WHERE id = $1`, userID, cvData)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return errUserNotFound
	}

	log.Printf("saveUserCV: saved CV for user=%d in %s", userID, time.Since(start).String())
	return nil
}

func (s *server) getUserCV(ctx context.Context, userID int64) ([]byte, error) {
	start := time.Now()
	log.Println("getUserCV: running SELECT cv_file FROM users WHERE id=$1")

	var cv []byte
	err := s.db.QueryRow(ctx, `SELECT cv_file FROM users WHERE id = $1`, userID).Scan(&cv)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errUserNotFound
		}
		return nil, err
	}

	log.Printf("getUserCV: fetched CV for user=%d in %s", userID, time.Since(start).String())
	return cv, nil
}

func (s *server) insertRegistration(ctx context.Context, req createRegistrationRequest) (Registration, error) {
	start := time.Now()
	log.Println("insertRegistration: running INSERT INTO registration")

	applicantCount := 1
	if req.ApplicantCount != nil {
		applicantCount = *req.ApplicantCount
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO registration (
			full_name, job_title, address_full, whatsapp_number, note, applicant_count, visa_type
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING registration_id, full_name, job_title, address_full, whatsapp_number, note, applicant_count, visa_type, created_at, updated_at
	`,
		req.FullName, req.JobTitle, req.AddressFull, req.WhatsappNumber, req.Note, applicantCount, req.VisaType,
	)

	var (
		r           Registration
		jobTitle    sql.NullString
		addressFull sql.NullString
		note        sql.NullString
		visaType    sql.NullString
	)

	if err := row.Scan(
		&r.RegistrationID,
		&r.FullName,
		&jobTitle,
		&addressFull,
		&r.WhatsappNumber,
		&note,
		&r.ApplicantCount,
		&visaType,
		&r.CreatedAt,
		&r.UpdatedAt,
	); err != nil {
		return Registration{}, err
	}

	if jobTitle.Valid {
		r.JobTitle = &jobTitle.String
	}
	if addressFull.Valid {
		r.AddressFull = &addressFull.String
	}
	if note.Valid {
		r.Note = &note.String
	}
	if visaType.Valid {
		r.VisaType = &visaType.String
	}

	log.Printf("insertRegistration: inserted id=%s in %s", r.RegistrationID.String(), time.Since(start).String())
	return r, nil
}

func (s *server) getRegistrationByID(ctx context.Context, id uuid.UUID) (Registration, error) {
	start := time.Now()
	log.Println("getRegistrationByID: running SELECT ... FROM registration WHERE registration_id=$1")

	var (
		r           Registration
		jobTitle    sql.NullString
		addressFull sql.NullString
		note        sql.NullString
		visaType    sql.NullString
	)

	err := s.db.QueryRow(ctx, `
		SELECT registration_id, full_name, job_title, address_full, whatsapp_number, note, applicant_count, visa_type, created_at, updated_at
		FROM registration
		WHERE registration_id = $1
	`, id).Scan(
		&r.RegistrationID,
		&r.FullName,
		&jobTitle,
		&addressFull,
		&r.WhatsappNumber,
		&note,
		&r.ApplicantCount,
		&visaType,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Registration{}, errRegistrationNotFound
		}
		return Registration{}, err
	}

	if jobTitle.Valid {
		r.JobTitle = &jobTitle.String
	}
	if addressFull.Valid {
		r.AddressFull = &addressFull.String
	}
	if note.Valid {
		r.Note = &note.String
	}
	if visaType.Valid {
		r.VisaType = &visaType.String
	}

	log.Printf("getRegistrationByID: fetched id=%s in %s", r.RegistrationID.String(), time.Since(start).String())
	return r, nil
}

func (s *server) saveRegistrationFile(ctx context.Context, registrationID uuid.UUID, fileType, filename string, data []byte) (uuid.UUID, error) {
	start := time.Now()
	log.Println("saveRegistrationFile: verifying registration exists")

	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM registration WHERE registration_id = $1)`, registrationID).Scan(&exists); err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, errRegistrationNotFound
	}

	var fileID uuid.UUID
	log.Println("saveRegistrationFile: inserting into file_upload")
	if err := s.db.QueryRow(ctx, `
		INSERT INTO file_upload (registration_id, file_type, filename, file, file_size)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING file_id
	`, registrationID, fileType, filename, data, int64(len(data))).Scan(&fileID); err != nil {
		return uuid.Nil, err
	}

	log.Printf("saveRegistrationFile: saved file_id=%s for registration=%s in %s", fileID.String(), registrationID.String(), time.Since(start).String())
	return fileID, nil
}

type RegistrationFile struct {
	FileID         uuid.UUID
	RegistrationID uuid.UUID
	FileType       string
	Filename       string
	FileSize       int64
	Data           []byte
	CreatedAt      time.Time
}

func (s *server) getRegistrationFile(ctx context.Context, fileID uuid.UUID) (RegistrationFile, error) {
	start := time.Now()
	log.Println("getRegistrationFile: running SELECT ... FROM file_upload WHERE file_id=$1")

	var rf RegistrationFile
	err := s.db.QueryRow(ctx, `
		SELECT file_id, registration_id, file_type, filename, file_size, file, created_at
		FROM file_upload
		WHERE file_id = $1
	`, fileID).Scan(
		&rf.FileID,
		&rf.RegistrationID,
		&rf.FileType,
		&rf.Filename,
		&rf.FileSize,
		&rf.Data,
		&rf.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RegistrationFile{}, errFileNotFound
		}
		return RegistrationFile{}, err
	}

	log.Printf("getRegistrationFile: fetched file_id=%s in %s", rf.FileID.String(), time.Since(start).String())
	return rf, nil
}
