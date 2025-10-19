package kits

import "fmt"

// ErrKitNotFound is returned when a kit cannot be found
type ErrKitNotFound struct {
	Name string
}

func (e ErrKitNotFound) Error() string {
	return fmt.Sprintf("kit not found: %s", e.Name)
}

// ErrInvalidManifest is returned when a kit manifest is invalid
type ErrInvalidManifest struct {
	Field  string
	Reason string
	Index  *int // Optional index for array fields
}

func (e ErrInvalidManifest) Error() string {
	if e.Index != nil {
		return fmt.Sprintf("invalid kit manifest: %s[%d]: %s", e.Field, *e.Index, e.Reason)
	}
	return fmt.Sprintf("invalid kit manifest: %s: %s", e.Field, e.Reason)
}

// ErrManifestParse is returned when parsing a manifest fails
type ErrManifestParse struct {
	Path string
	Err  error
}

func (e ErrManifestParse) Error() string {
	return fmt.Sprintf("failed to parse kit manifest at %s: %v", e.Path, e.Err)
}

func (e ErrManifestParse) Unwrap() error {
	return e.Err
}

// ErrHelperLoad is returned when loading kit helpers fails
type ErrHelperLoad struct {
	Kit string
	Err error
}

func (e ErrHelperLoad) Error() string {
	return fmt.Sprintf("failed to load helpers for kit %s: %v", e.Kit, e.Err)
}

func (e ErrHelperLoad) Unwrap() error {
	return e.Err
}
