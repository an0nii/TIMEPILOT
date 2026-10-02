package httpapi

import "embed"

//go:embed static/*
var staticFiles embed.FS
