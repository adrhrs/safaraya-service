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
)

const maxCVUploadSize = 5 << 20 // 5MB

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

	r.Body = http.MaxBytesReader(w, r.Body, maxCVUploadSize+1024)
	if err := r.ParseMultipartForm(maxCVUploadSize); err != nil {
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

	if header.Size > maxCVUploadSize {
		log.Printf("uploadUserCV file too large: %d bytes", header.Size)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file_too_large"})
		return
	}

	buf := bytes.NewBuffer(nil)
	n, err := io.Copy(buf, io.LimitReader(file, maxCVUploadSize+1))
	if err != nil {
		log.Printf("uploadUserCV read failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if n > maxCVUploadSize {
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
