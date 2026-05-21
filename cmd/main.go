package main

import (
	"fmt"
	"os"

	"github.com/krishna/llm-index/internal/db"
	"github.com/krishna/llm-index/internal/importer"
	"github.com/krishna/llm-index/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "sync":
		runSync()
	case "version":
		fmt.Printf("llm-index %s (%s) built %s\n", version, commit, date)

	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: llm-index search <query>")
			os.Exit(1)
		}
		runSearch(os.Args[2])
	case "migrate":
		runMigrate()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nusage: llm-index [sync|search|migrate]\n", os.Args[1])
		os.Exit(1)
	}
}

func runTUI() {
	pool, err := db.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	tui.Run(pool)
}

func runSync() {
	pool, err := db.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	synced, err := importer.SyncAll(pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("synced %d sessions\n", synced)
}

func runSearch(query string) {
	pool, err := db.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	sessions, err := db.Search(pool, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search: %v\n", err)
		os.Exit(1)
	}
	for _, s := range sessions {
		fmt.Printf("[%s] %s — %s (%s)\n", s.Source, s.Title, s.Model, s.UpdatedAt.Format("2006-01-02 15:04"))
	}
}

func runMigrate() {
	pool, err := db.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("migration complete")
}
