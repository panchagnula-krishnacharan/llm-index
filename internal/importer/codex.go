package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/krishna/llm-index/internal/db"
)

func SyncCodex(pool *pgxpool.Pool) (int, error) {
	home, _ := os.UserHomeDir()
	codexDir := filepath.Join(home, ".codex")

	if _, err := os.Stat(codexDir); os.IsNotExist(err) {
		return 0, nil
	}

	// Walk for JSON session files
	count := 0
	filepath.Walk(codexDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		sess, msgs, err := parseCodexSession(path)
		if err != nil {
			return nil
		}

		if err := db.UpsertSession(pool, sess); err != nil {
			return nil
		}
		if len(msgs) > 0 {
			_ = db.UpsertMessages(pool, "codex", sess.SourceID, msgs)
		}
		count++
		return nil
	})

	return count, nil
}

type codexMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type codexSession struct {
	ID       string         `json:"id"`
	Model    string         `json:"model"`
	Messages []codexMessage `json:"messages"`
}

func parseCodexSession(path string) (*db.Session, []db.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var cs codexSession
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, nil, err
	}

	info, _ := os.Stat(path)
	sourceID := cs.ID
	if sourceID == "" {
		sourceID = filepath.Base(path)
	}

	// Derive title from first user message
	title := ""
	for _, m := range cs.Messages {
		if m.Role == "user" {
			title = m.Content
			if len(title) > 80 {
				title = title[:80]
			}
			break
		}
	}

	var msgs []db.Message
	for i, m := range cs.Messages {
		msgs = append(msgs, db.Message{
			Role:      m.Role,
			Content:   m.Content,
			Seq:       i,
			CreatedAt: info.ModTime(),
		})
	}

	sess := &db.Session{
		Source:       "codex",
		SourceID:     sourceID,
		Title:        title,
		Model:        cs.Model,
		Provider:     "openai",
		StartedAt:    info.ModTime(),
		UpdatedAt:    info.ModTime(),
		MessageCount: len(msgs),
		ResumeCmd:    fmt.Sprintf("codex --resume %s", path),
	}

	return sess, msgs, nil
}
