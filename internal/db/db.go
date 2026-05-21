package db

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations.sql
var migrationSQL string

type Session struct {
	ID           string
	Source       string
	SourceID     string
	Title        string
	Model        string
	Provider     string
	Directory    string
	StartedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
	ResumeCmd    string
}

type Message struct {
	ID        string
	SessionID string
	Role      string
	Content   string
	Seq       int
	CreatedAt time.Time
}

func Connect() (*pgxpool.Pool, error) {
	dsn := os.Getenv("LLM_INDEX_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/llm_index?sslmode=disable"
	}
	return pgxpool.New(context.Background(), dsn)
}

func Migrate(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), migrationSQL)
	return err
}

func UpsertSession(pool *pgxpool.Pool, s *Session) error {
	_, err := pool.Exec(context.Background(), `
		INSERT INTO sessions (source, source_id, title, model, provider, directory, started_at, updated_at, message_count, resume_cmd)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (source, source_id) DO UPDATE SET
			title = EXCLUDED.title,
			model = EXCLUDED.model,
			provider = EXCLUDED.provider,
			directory = EXCLUDED.directory,
			updated_at = EXCLUDED.updated_at,
			message_count = EXCLUDED.message_count,
			resume_cmd = EXCLUDED.resume_cmd
	`, s.Source, s.SourceID, s.Title, s.Model, s.Provider, s.Directory, s.StartedAt, s.UpdatedAt, s.MessageCount, s.ResumeCmd)
	return err
}

func UpsertMessages(pool *pgxpool.Pool, sessionSource, sessionSourceID string, msgs []Message) error {
	ctx := context.Background()

	// Get session ID
	var sessionID string
	err := pool.QueryRow(ctx, `SELECT id FROM sessions WHERE source = $1 AND source_id = $2`, sessionSource, sessionSourceID).Scan(&sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Delete existing messages and re-insert
	_, err = pool.Exec(ctx, `DELETE FROM messages WHERE session_id = $1::uuid`, sessionID)
	if err != nil {
		return err
	}

	for _, m := range msgs {
		_, err = pool.Exec(ctx, `
			INSERT INTO messages (session_id, role, content, seq, created_at)
			VALUES ($1::uuid, $2, $3, $4, $5)
		`, sessionID, m.Role, m.Content, m.Seq, m.CreatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func Search(pool *pgxpool.Pool, query string) ([]Session, error) {
	rows, err := pool.Query(context.Background(), `
		SELECT id, source, source_id, coalesce(title,''), coalesce(model,''), coalesce(provider,''),
		       coalesce(directory,''), coalesce(started_at, now()), coalesce(updated_at, now()),
		       message_count, coalesce(resume_cmd,'')
		FROM sessions
		WHERE search_vector @@ plainto_tsquery('english', $1)
		ORDER BY updated_at DESC
		LIMIT 50
	`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Source, &s.SourceID, &s.Title, &s.Model, &s.Provider, &s.Directory, &s.StartedAt, &s.UpdatedAt, &s.MessageCount, &s.ResumeCmd); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func ListSessions(pool *pgxpool.Pool) ([]Session, error) {
	rows, err := pool.Query(context.Background(), `
		SELECT id, source, source_id, coalesce(title,''), coalesce(model,''), coalesce(provider,''),
		       coalesce(directory,''), coalesce(started_at, now()), coalesce(updated_at, now()),
		       message_count, coalesce(resume_cmd,'')
		FROM sessions
		ORDER BY updated_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Source, &s.SourceID, &s.Title, &s.Model, &s.Provider, &s.Directory, &s.StartedAt, &s.UpdatedAt, &s.MessageCount, &s.ResumeCmd); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func GetMessages(pool *pgxpool.Pool, sessionID string) ([]Message, error) {
	rows, err := pool.Query(context.Background(), `
		SELECT id, session_id, role, coalesce(content,''), seq, coalesce(created_at, now())
		FROM messages
		WHERE session_id = $1::uuid
		ORDER BY seq
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Seq, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
