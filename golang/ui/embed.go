package ui

import "embed"

//go:embed assets/* static/*
var EmbeddedFS embed.FS
