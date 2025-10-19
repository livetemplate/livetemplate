package kits

import (
	"embed"
)

//go:embed system/**
var systemKits embed.FS

// GetSystemKits returns the embedded filesystem containing system kits
func GetSystemKits() *embed.FS {
	return &systemKits
}

// DefaultLoader creates a kit loader with embedded system kits
func DefaultLoader() *KitLoader {
	return NewLoader(&systemKits)
}
