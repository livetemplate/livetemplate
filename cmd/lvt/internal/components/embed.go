package components

import (
	"embed"
)

//go:embed system/**
var systemComponents embed.FS

// GetSystemComponents returns the embedded filesystem containing system components
func GetSystemComponents() *embed.FS {
	return &systemComponents
}

// DefaultLoader creates a component loader with embedded system components
func DefaultLoader() *ComponentLoader {
	return NewLoader(&systemComponents)
}
