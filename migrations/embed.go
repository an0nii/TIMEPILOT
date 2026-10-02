package migrations

import _ "embed"

// Schema contains the idempotent database migration used by the migrate process.
//
//go:embed 001_create_tasks.sql
var Schema string
