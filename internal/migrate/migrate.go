package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"timepilot/migrations"
)

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, migrations.Schema); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}
