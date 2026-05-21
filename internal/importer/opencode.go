package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/krishna/llm-index/internal/db"
	_ "modernc.org/sqlite" // registers as "sqlite"
)

func SyncOpenCode(pool *pgxpool.Pool) (int, error) {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0, nil
	}

	sqlite, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return 0, fmt.Errorf("open opencode db: %w", err)
	}
	defer sqlite.Close()

	rows, err := sqlite.Query(`SELECT id, title, directory, time_created, time_updated FROM session ORDER BY time_updated DESC LIMIT 500`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, title, directory string
		var timeCreated, timeUpdated int64
		if err := rows.Scan(&id, &title, &directory, &timeCreated, &timeUpdated); err != nil {
			continue
		}

		// Get model from first message
		model, provider := getOpenCodeModel(sqlite, id)
		msgCount := getOpenCodeMessageCount(sqlite, id)

		sess := &db.Session{
			Source:       "opencode",
			SourceID:     id,
			Title:        title,
			Model:        model,
			Provider:     provider,
			Directory:    directory,
			StartedAt:    time.UnixMilli(timeCreated),
			UpdatedAt:    time.UnixMilli(timeUpdated),
			MessageCount: msgCount,
			ResumeCmd:    fmt.Sprintf("opencode -s %s", id),
		}

		if err := db.UpsertSession(pool, sess); err != nil {
			continue
		}

		// Import messages
		msgs := getOpenCodeMessages(sqlite, id)
		if len(msgs) > 0 {
			_ = db.UpsertMessages(pool, "opencode", id, msgs)
		}

		count++
	}
	return count, nil
}

func getOpenCodeModel(sqlite *sql.DB, sessionID string) (string, string) {
	var data string
	err := sqlite.QueryRow(`SELECT data FROM message WHERE session_id = ? LIMIT 1`, sessionID).Scan(&data)
	if err != nil {
		return "", ""
	}
	var msg struct {
		Model struct {
			ModelID    string `json:"modelID"`
			ProviderID string `json:"providerID"`
		} `json:"model"`
	}
	if json.Unmarshal([]byte(data), &msg) == nil {
		return msg.Model.ModelID, msg.Model.ProviderID
	}
	return "", ""
}

func getOpenCodeMessageCount(sqlite *sql.DB, sessionID string) int {
	var count int
	sqlite.QueryRow(`SELECT count(*) FROM message WHERE session_id = ?`, sessionID).Scan(&count)
	return count
}

func getOpenCodeMessages(sqlite *sql.DB, sessionID string) []db.Message {
	rows, err := sqlite.Query(`
		SELECT m.id, m.time_created, m.data, GROUP_CONCAT(p.data, '|||')
		FROM message m
		LEFT JOIN part p ON p.message_id = m.id AND p.session_id = m.session_id
		WHERE m.session_id = ?
		GROUP BY m.id
		ORDER BY m.time_created
	`, sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []db.Message
	seq := 0
	for rows.Next() {
		var id string
		var timeCreated int64
		var msgData string
		var partsRaw sql.NullString
		if err := rows.Scan(&id, &timeCreated, &msgData, &partsRaw); err != nil {
			continue
		}

		var parsed struct {
			Role string `json:"role"`
		}
		json.Unmarshal([]byte(msgData), &parsed)

		// Extract text from parts
		content := ""
		if partsRaw.Valid {
			for _, partJSON := range splitParts(partsRaw.String) {
				var part struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if json.Unmarshal([]byte(partJSON), &part) == nil && part.Type == "text" {
					content += part.Text + "\n"
				}
			}
		}

		msgs = append(msgs, db.Message{
			Role:      parsed.Role,
			Content:   content,
			Seq:       seq,
			CreatedAt: time.UnixMilli(timeCreated),
		})
		seq++
	}
	return msgs
}

func splitParts(raw string) []string {
	// Parts are concatenated with |||
	var parts []string
	current := ""
	for i := 0; i < len(raw); i++ {
		if i+2 < len(raw) && raw[i:i+3] == "|||" {
			parts = append(parts, current)
			current = ""
			i += 2
		} else {
			current += string(raw[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
