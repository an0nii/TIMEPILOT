package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"timepilot/internal/task"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (s *Postgres) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Postgres) List(ctx context.Context) ([]task.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]task.Task, 0)
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, item)
	}
	return tasks, rows.Err()
}

func (s *Postgres) Get(ctx context.Context, id int64) (task.Task, error) {
	item, err := scanTask(s.pool.QueryRow(ctx, `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks WHERE id = $1`, id))
	return item, mapNotFound(err)
}

func (s *Postgres) Create(ctx context.Context, input task.Input) (task.Task, error) {
	item, err := scanTask(s.pool.QueryRow(ctx, `
		INSERT INTO tasks (title, description, status)
		VALUES ($1, $2, $3)
		RETURNING id, title, description, status, created_at, updated_at`,
		input.Title, input.Description, input.Status))
	return item, err
}

func (s *Postgres) Update(ctx context.Context, id int64, input task.Input) (task.Task, error) {
	item, err := scanTask(s.pool.QueryRow(ctx, `
		UPDATE tasks
		SET title = $2, description = $3, status = $4, updated_at = now()
		WHERE id = $1
		RETURNING id, title, description, status, created_at, updated_at`,
		id, input.Title, input.Description, input.Status))
	return item, mapNotFound(err)
}

func (s *Postgres) Delete(ctx context.Context, id int64) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if result.RowsAffected() == 0 {
		return task.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(...any) error
}

func scanTask(row scanner) (task.Task, error) {
	var item task.Task
	err := row.Scan(&item.ID, &item.Title, &item.Description, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return task.Task{}, fmt.Errorf("scan task: %w", err)
	}
	return item, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, task.ErrNotFound) {
		return task.ErrNotFound
	}
	return err
}
