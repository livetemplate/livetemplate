package send

import (
	"fmt"

	"github.com/livetemplate/livetemplate/internal/jsonutil"
)

// UpdateResponse represents a tree update response sent to the client.
type UpdateResponse struct {
	Tree interface{}       `json:"tree"` // Opaque tree update (internal format)
	Meta *ResponseMetadata `json:"meta,omitempty"`
}

// ResponseMetadata contains information about the action that generated the update.
type ResponseMetadata struct {
	Success      bool              `json:"success"`
	Errors       map[string]string `json:"errors"`
	Action       string            `json:"action,omitempty"`       // only on action responses
	Capabilities []string          `json:"capabilities,omitempty"` // only on initial render
}

// PrepareUpdate wraps a tree with metadata for action responses.
// For initial renders, construct UpdateResponse directly to include Capabilities.
// If errors is nil or empty, metadata is not included.
// If action is non-empty, it's included in the metadata.
func PrepareUpdate(tree interface{}, errors map[string]string, action string) *UpdateResponse {
	resp := &UpdateResponse{
		Tree: tree,
	}

	// Only add metadata if there are errors or an action
	if len(errors) > 0 || action != "" {
		resp.Meta = &ResponseMetadata{
			Success: len(errors) == 0,
			Errors:  errors,
			Action:  action,
		}
	}

	return resp
}

// SerializeUpdate marshals an UpdateResponse to JSON bytes.
// Includes recovery from stack overflow to handle cyclic data gracefully
// (json-iterator does not detect cycles like encoding/json does).
func SerializeUpdate(resp *UpdateResponse) (result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("failed to marshal update response: panic during serialization (possible cyclic data): %v", r)
		}
	}()

	bytes, marshalErr := jsonutil.API.Marshal(resp)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal update response: %w", marshalErr)
	}
	return bytes, nil
}

// PrepareAndSerialize combines PrepareUpdate and SerializeUpdate in one call.
// This is a convenience function for the common case of preparing and serializing.
func PrepareAndSerialize(tree interface{}, errors map[string]string, action string) ([]byte, error) {
	resp := PrepareUpdate(tree, errors, action)
	return SerializeUpdate(resp)
}
