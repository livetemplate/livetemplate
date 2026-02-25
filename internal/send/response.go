package send

import (
	"encoding/json"
	"fmt"
)

// UpdateResponse represents a tree update response sent to the client.
type UpdateResponse struct {
	Tree any               `json:"tree"` // Opaque tree update (internal format)
	Meta *ResponseMetadata `json:"meta,omitempty"`
}

// ResponseMetadata contains information about the action that generated the update.
type ResponseMetadata struct {
	Success bool              `json:"success"` // true if no validation errors
	Errors  map[string]string `json:"errors"`  // field errors
	Action  string            `json:"action,omitempty"`
}

// PrepareUpdate wraps a tree with metadata for sending to client.
// If errors is nil or empty, metadata is not included.
// If action is non-empty, it's included in the metadata.
func PrepareUpdate(tree any, errors map[string]string, action string) *UpdateResponse {
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
func SerializeUpdate(resp *UpdateResponse) ([]byte, error) {
	bytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update response: %w", err)
	}
	return bytes, nil
}

// PrepareAndSerialize combines PrepareUpdate and SerializeUpdate in one call.
// This is a convenience function for the common case of preparing and serializing.
func PrepareAndSerialize(tree any, errors map[string]string, action string) ([]byte, error) {
	resp := PrepareUpdate(tree, errors, action)
	return SerializeUpdate(resp)
}
