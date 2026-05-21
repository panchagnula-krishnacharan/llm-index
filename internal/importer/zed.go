package importer

import (
	"bufio"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/krishna/llm-index/internal/db"
	"os"
	"path/filepath"
	"strings"
)

func SyncZed(pool *pgxpool.Pool) (int, error) {
	home, _ := os.UserHomeDir()
	convDir := filepath.Join(home, ".local", "share", "zed", "conversations")

	entries, err := os.ReadDir(convDir)
	if err != nil {
		return 0, nil // Zed not installed
	}

	count := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(convDir, entry.Name())
		sess, msgs, err := parseZedConversation(path)
		if err != nil {
			continue
		}

		if err := db.UpsertSession(pool, sess); err != nil {
			continue
		}
		if len(msgs) > 0 {
			_ = db.UpsertMessages(pool, "zed", sess.SourceID, msgs)
		}
		count++
	}
	return count, nil
}

func parseZedConversation(path string) (*db.Session, []db.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	info, _ := f.Stat()
	sourceID := filepath.Base(path)

	// Parse YAML frontmatter and markdown sections
	scanner := bufio.NewScanner(f)
	var model, title string
	inFrontmatter := false
	var msgs []db.Message
	var currentRole, currentContent string
	seq := 0

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			continue
		}

		if inFrontmatter {
			if strings.HasPrefix(line, "model:") {
				model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
			}
			if strings.HasPrefix(line, "title:") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			}
			continue
		}

		// Parse ## User / ## Assistant sections
		if strings.HasPrefix(line, "## User") || strings.HasPrefix(line, "## Assistant") {
			if currentRole != "" {
				msgs = append(msgs, db.Message{
					Role:      currentRole,
					Content:   strings.TrimSpace(currentContent),
					Seq:       seq,
					CreatedAt: info.ModTime(),
				})
				seq++
			}
			if strings.HasPrefix(line, "## User") {
				currentRole = "user"
			} else {
				currentRole = "assistant"
			}
			currentContent = ""
		} else {
			currentContent += line + "\n"
		}
	}

	// Final message
	if currentRole != "" {
		msgs = append(msgs, db.Message{
			Role:      currentRole,
			Content:   strings.TrimSpace(currentContent),
			Seq:       seq,
			CreatedAt: info.ModTime(),
		})
	}

	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	sess := &db.Session{
		Source:       "zed",
		SourceID:     sourceID,
		Title:        title,
		Model:        model,
		Provider:     "zed",
		StartedAt:    info.ModTime(),
		UpdatedAt:    info.ModTime(),
		MessageCount: len(msgs),
		ResumeCmd:    fmt.Sprintf("zed %s", path),
	}

	return sess, msgs, nil
}
