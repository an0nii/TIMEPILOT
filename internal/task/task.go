package task

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"
)

var ErrNotFound = errors.New("task not found")

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Input struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      Status `json:"status"`
}

func (i Input) Valid() bool {
	titleLength := utf8.RuneCountInString(i.Title)
	return titleLength > 0 && titleLength <= 200 && validStatus(i.Status)
}

func validStatus(status Status) bool {
	return status == StatusTodo || status == StatusInProgress || status == StatusDone
}

type Store interface {
	List(context.Context) ([]Task, error)
	Get(context.Context, int64) (Task, error)
	Create(context.Context, Input) (Task, error)
	Update(context.Context, int64, Input) (Task, error)
	Delete(context.Context, int64) error
	Ping(context.Context) error
}
