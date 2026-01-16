package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

var errUserNotFound = errors.New("user not found")

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
