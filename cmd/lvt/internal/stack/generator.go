package stack

import "context"

// Generator is the interface all providers must implement
type Generator interface {
	// Generate creates deployment configuration files
	Generate(ctx context.Context, config StackConfig, outputDir string) error

	// Validate checks if the generated stack is valid
	Validate(ctx context.Context, stackDir string) error

	// GetInfo returns information about the stack
	GetInfo(ctx context.Context, stackDir string) (*StackInfo, error)
}

// StackInfo contains information about a deployed stack
type StackInfo struct {
	Provider          string
	Configuration     TrackingConfig
	ModifiedFiles     []string
	RequiredSecrets   []string
	DeploymentCommand string
	EstimatedCost     string
}
