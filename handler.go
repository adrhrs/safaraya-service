package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxUploadSize = 5 << 20 // 5MB

func (s *server) usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getUsersHandler(w, r)
	case http.MethodPost:
		s.createUserHandler(w, r)
	default:
		log.Printf("usersHandler invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
	}
}

func (s *server) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("getUsers start: method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodGet {
		log.Printf("getUsers invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	log.Println("getUsers querying database")
	users, err := s.fetchUsers(ctx)
	if err != nil {
		log.Printf("getUsers query failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	for i := range users {
		if users[i].HasCV {
			url := buildDownloadURL(r, users[i].ID)
			users[i].CvFileDownloadURL = &url
		}
	}

	log.Printf("getUsers returning %d users", len(users))
	if err := json.NewEncoder(w).Encode(users); err != nil {
		log.Printf("getUsers encode failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
	}
}

type createUserRequest struct {
	Name *string `json:"name"`
	Age  *int    `json:"age"`
}

type createRegistrationRequest struct {
	FullName       string  `json:"full_name"`
	JobTitle       *string `json:"job_title"`
	AddressFull    *string `json:"address_full"`
	WhatsappNumber string  `json:"whatsapp_number"`
	Note           *string `json:"note"`
	ApplicantCount *int    `json:"applicant_count"`
	VisaType       *string `json:"visa_type"`
}

func (s *server) createUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("createUser start: method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		log.Printf("createUser invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("createUser decode failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_json"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	log.Println("createUser inserting into database")
	user, err := s.insertUser(ctx, req)
	if err != nil {
		log.Printf("createUser insert failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(user); err != nil {
		log.Printf("createUser encode failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
	}
}

func (s *server) registrationsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createRegistrationHandler(w, r)
	default:
		log.Printf("registrationsHandler invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
	}
}

func (s *server) registrationDetailHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "registrations" {
		notFoundHandler(w, r)
		return
	}

	regID, err := uuid.Parse(parts[1])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_registration_id"})
		return
	}

	if len(parts) == 2 {
		if r.Method == http.MethodGet {
			s.getRegistrationHandler(w, r, regID)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	notFoundHandler(w, r)
}

func (s *server) createRegistrationHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("createRegistration start: method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		log.Printf("createRegistration invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req createRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("createRegistration decode failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_json"})
		return
	}

	if strings.TrimSpace(req.FullName) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "full_name_required"})
		return
	}

	if strings.TrimSpace(req.WhatsappNumber) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "whatsapp_number_required"})
		return
	}

	if req.ApplicantCount != nil && *req.ApplicantCount < 1 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_applicant_count"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	log.Println("createRegistration inserting into database")
	registration, err := s.insertRegistration(ctx, req)
	if err != nil {
		log.Printf("createRegistration insert failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(registration); err != nil {
		log.Printf("createRegistration encode failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
	}
}

func (s *server) getRegistrationHandler(w http.ResponseWriter, r *http.Request, registrationID uuid.UUID) {
	log.Printf("getRegistration start: registrationID=%s method=%s remote=%s", registrationID.String(), r.Method, r.RemoteAddr)
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	registration, err := s.getRegistrationByID(ctx, registrationID)
	if err != nil {
		if errors.Is(err, errRegistrationNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "registration_not_found"})
			return
		}
		log.Printf("getRegistration fetch failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if err := json.NewEncoder(w).Encode(registration); err != nil {
		log.Printf("getRegistration encode failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
	}
}

func (s *server) uploadRegistrationFileHandler(w http.ResponseWriter, r *http.Request, registrationID uuid.UUID) {
	log.Printf("uploadRegistrationFile start: registrationID=%s method=%s remote=%s", registrationID.String(), r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("uploadRegistrationFile parse form failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_form"})
		return
	}

	fileType := strings.TrimSpace(r.FormValue("file_type"))
	if fileType == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_type_required"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("uploadRegistrationFile missing file: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_required"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		log.Printf("uploadRegistrationFile file too large: %d bytes", header.Size)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	buf := bytes.NewBuffer(nil)
	n, err := io.Copy(buf, io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		log.Printf("uploadRegistrationFile read failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if n > maxUploadSize {
		log.Printf("uploadRegistrationFile exceeded limit during read: %d bytes", n)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	fileData := buf.Bytes()
	if len(fileData) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "empty_file"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	fileID, err := s.saveRegistrationFile(ctx, registrationID, fileType, header.Filename, fileData)
	if err != nil {
		if errors.Is(err, errRegistrationNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "registration_not_found"})
			return
		}
		log.Printf("uploadRegistrationFile save failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "uploaded",
		"file_id": fileID.String(),
	})
}

func (s *server) registrationFilesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("registrationFiles start: method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("registrationFiles parse form failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_form"})
		return
	}

	regIDStr := strings.TrimSpace(r.FormValue("registration_id"))
	if regIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "registration_id_required"})
		return
	}

	regID, err := uuid.Parse(regIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_registration_id"})
		return
	}

	fileType := strings.TrimSpace(r.FormValue("file_type"))
	if fileType == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_type_required"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("registrationFiles missing file: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_required"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		log.Printf("registrationFiles file too large: %d bytes", header.Size)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	buf := bytes.NewBuffer(nil)
	n, err := io.Copy(buf, io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		log.Printf("registrationFiles read failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if n > maxUploadSize {
		log.Printf("registrationFiles exceeded limit during read: %d bytes", n)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	fileData := buf.Bytes()
	if len(fileData) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "empty_file"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	fileID, err := s.saveRegistrationFile(ctx, regID, fileType, header.Filename, fileData)
	if err != nil {
		if errors.Is(err, errRegistrationNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "registration_not_found"})
			return
		}
		log.Printf("registrationFiles save failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "uploaded",
		"file_id": fileID.String(),
	})
}

func (s *server) registrationFileHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "registration-files" {
		notFoundHandler(w, r)
		return
	}

	fileID, err := uuid.Parse(parts[1])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_file_id"})
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	s.downloadRegistrationFileHandler(w, r, fileID)
}

func (s *server) downloadRegistrationFileHandler(w http.ResponseWriter, r *http.Request, fileID uuid.UUID) {
	log.Printf("downloadRegistrationFile start: fileID=%s method=%s remote=%s", fileID.String(), r.Method, r.RemoteAddr)
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rf, err := s.getRegistrationFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, errFileNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_not_found"})
			return
		}
		log.Printf("downloadRegistrationFile fetch failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if len(rf.Data) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_not_found"})
		return
	}

	contentType := http.DetectContentType(rf.Data)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", rf.Filename))
	if rf.FileSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(rf.FileSize, 10))
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(rf.Data); err != nil {
		log.Printf("downloadRegistrationFile write failed: %v", err)
	}
}

func (s *server) userCVHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "users" || parts[2] != "cv" {
		notFoundHandler(w, r)
		return
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_user_id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.downloadUserCVHandler(w, r, userID)
	case http.MethodPost:
		s.uploadUserCVHandler(w, r, userID)
	default:
		log.Printf("userCVHandler invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
	}
}

func (s *server) uploadUserCVHandler(w http.ResponseWriter, r *http.Request, userID int64) {
	log.Printf("uploadUserCV start: userID=%d method=%s remote=%s", userID, r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("uploadUserCV parse form failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("uploadUserCV missing file: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_required"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		log.Printf("uploadUserCV file too large: %d bytes", header.Size)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	buf := bytes.NewBuffer(nil)
	n, err := io.Copy(buf, io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		log.Printf("uploadUserCV read failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if n > maxUploadSize {
		log.Printf("uploadUserCV file exceeded limit during read: %d bytes", n)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	cvData := buf.Bytes()
	if len(cvData) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "empty_file"})
		return
	}

	mimeType := http.DetectContentType(cvData)
	contentTypeHeader := header.Header.Get("Content-Type")
	if mimeType != "application/pdf" && contentTypeHeader != "application/pdf" {
		log.Printf("uploadUserCV invalid mime type: detected=%s header=%s", mimeType, contentTypeHeader)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_file_type"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.saveUserCV(ctx, userID, cvData); err != nil {
		if errors.Is(err, errUserNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_not_found"})
			return
		}
		log.Printf("uploadUserCV save failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "uploaded"})
}

func (s *server) downloadUserCVHandler(w http.ResponseWriter, r *http.Request, userID int64) {
	log.Printf("downloadUserCV start: userID=%d method=%s remote=%s", userID, r.Method, r.RemoteAddr)
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cvData, err := s.getUserCV(ctx, userID)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_not_found"})
			return
		}
		log.Printf("downloadUserCV fetch failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if len(cvData) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cv_not_found"})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"cv-"+strconv.FormatInt(userID, 10)+".pdf\"")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(cvData); err != nil {
		log.Printf("downloadUserCV write failed: %v", err)
	}
}

func buildDownloadURL(r *http.Request, userID int64) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	base := fmt.Sprintf("%s://%s", scheme, r.Host)
	if strings.HasPrefix(serviceHost, "http://") || strings.HasPrefix(serviceHost, "https://") {
		base = serviceHost
	}
	return base + fmt.Sprintf(cvDownloadPathTemplate, userID)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("ping request: method=%s remote=%s", r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"message": "pong v2"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Best-effort error response if encoding fails
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("not found: path=%s method=%s remote=%s", r.URL.Path, r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
