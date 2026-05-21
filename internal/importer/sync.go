package importer

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncAll imports sessions from all known sources.
func SyncAll(pool *pgxpool.Pool) (int, error) {
	total := 0

	n, err := SyncOpenCode(pool)
	if err != nil {
		return total, err
	}
	total += n

	n, err = SyncZed(pool)
	if err != nil {
		// Non-fatal: Zed might not be installed
		_ = err
	}
	total += n

	n, err = SyncCodex(pool)
	if err != nil {
		_ = err
	}
	total += n

	return total, nil
}
