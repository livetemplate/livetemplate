package components

import (
	"fmt"
)

// ErrComponentNotFound is returned when a component cannot be found
type ErrComponentNotFound struct {
	Name string
}

func (e ErrComponentNotFound) Error() string {
	return fmt.Sprintf("component not found: %s", e.Name)
}

// ErrInvalidManifest is returned when a component manifest is invalid
type ErrInvalidManifest struct {
	Field  string
	Reason string
	Index  *int // Optional index for array fields
}

func (e ErrInvalidManifest) Error() string {
	if e.Index != nil {
		return fmt.Sprintf("invalid manifest: %s[%d]: %s", e.Field, *e.Index, e.Reason)
	}
	return fmt.Sprintf("invalid manifest: %s: %s", e.Field, e.Reason)
}

// ErrManifestParse is returned when a manifest file cannot be parsed
type ErrManifestParse struct {
	Path string
	Err  error
}

func (e ErrManifestParse) Error() string {
	return fmt.Sprintf("failed to parse manifest at %s: %v", e.Path, e.Err)
}

// ErrTemplateParse is returned when a template file cannot be parsed
type ErrTemplateParse struct {
	Path string
	Err  error
}

func (e ErrTemplateParse) Error() string {
	return fmt.Sprintf("failed to parse template at %s: %v", e.Path, e.Err)
}

// ErrDependencyCycle is returned when circular dependencies are detected
type ErrDependencyCycle struct {
	Component string
	Cycle     []string
}

func (e ErrDependencyCycle) Error() string {
	return fmt.Sprintf("circular dependency detected for component %s: %v", e.Component, e.Cycle)
}
