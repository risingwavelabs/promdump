package embed

import "embed"

//go:embed static/*
var Static embed.FS
