package main

import (
	"context"
	"database/sql"
	"log"
	"time"
)

func (s *server) fetchUsers(ctx context.Context) ([]User, error) {
	start := time.Now()
	log.Println("fetchUsers: running SELECT id, name, age, created_at FROM users")
	rows, err := s.db.Query(ctx, `SELECT id, name, age, created_at FROM users`)
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
		)

		if err := rows.Scan(&u.ID, &name, &age, &u.CreatedAt); err != nil {
			return nil, err
		}

		if name.Valid {
			u.Name = &name.String
		}

		if age.Valid {
			v := int(age.Int32)
			u.Age = &v
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("fetchUsers: fetched %d rows in %s", len(users), time.Since(start).String())
	return users, nil
}
