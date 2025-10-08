package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ResourceEntry represents a resource or view in the application
type ResourceEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "resource" or "view"
}

// RegisterResource adds a resource to the tracking file
func RegisterResource(basePath, name, path, resourceType string) error {
	resources, err := ReadResources(basePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if resource already exists
	for _, r := range resources {
		if r.Path == path {
			return nil // Already registered
		}
	}

	// Add new resource
	resources = append(resources, ResourceEntry{
		Name: name,
		Path: path,
		Type: resourceType,
	})

	return WriteResources(basePath, resources)
}

// ReadResources reads all registered resources
func ReadResources(basePath string) ([]ResourceEntry, error) {
	resourcesPath := filepath.Join(basePath, ".lvtresources")

	data, err := os.ReadFile(resourcesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ResourceEntry{}, nil
		}
		return nil, err
	}

	var resources []ResourceEntry
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, err
	}

	return resources, nil
}

// WriteResources writes the resources list to file
func WriteResources(basePath string, resources []ResourceEntry) error {
	resourcesPath := filepath.Join(basePath, ".lvtresources")

	data, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resourcesPath, data, 0644)
}
