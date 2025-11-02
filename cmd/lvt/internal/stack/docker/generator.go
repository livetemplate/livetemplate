package docker

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

//go:embed templates/docker-compose.yml.tmpl
var dockerComposeTemplate string

//go:embed templates/Dockerfile.tmpl
var dockerfileTemplate string

//go:embed templates/.dockerignore.tmpl
var dockerignoreTemplate string

//go:embed templates/.env.example.tmpl
var envExampleTemplate string

//go:embed templates/README.md.tmpl
var readmeTemplate string

//go:embed templates/litestream.yml.tmpl
var litestreamTemplate string

// Generator implements stack.Generator for Docker
type Generator struct{}

// New creates a new Docker generator
func New() *Generator {
	return &Generator{}
}

// Generate creates Docker deployment configuration
func (g *Generator) Generate(ctx context.Context, config stack.StackConfig, outputDir string) error {
	// Get project name from current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	projectName := filepath.Base(wd)

	// Convert to template data
	data := config.ToTemplateData(projectName)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate files
	files := map[string]string{
		"docker-compose.yml": dockerComposeTemplate,
		"Dockerfile":         dockerfileTemplate,
		".dockerignore":      dockerignoreTemplate,
		".env.example":       envExampleTemplate,
		"README.md":          readmeTemplate,
	}

	for filename, tmplContent := range files {
		if err := g.generateFile(filepath.Join(outputDir, filename), tmplContent, data); err != nil {
			return fmt.Errorf("failed to generate %s: %w", filename, err)
		}
	}

	// Generate litestream.yml if needed
	if config.Backup == stack.BackupLitestream {
		if err := g.generateLitestream(outputDir, config, data); err != nil {
			return fmt.Errorf("failed to generate litestream config: %w", err)
		}
	}

	return nil
}

// generateFile generates a single file from template
func (g *Generator) generateFile(outputPath, tmplContent string, data *stack.TemplateData) error {
	tmpl, err := template.New(filepath.Base(outputPath)).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateLitestream generates litestream.yml
func (g *Generator) generateLitestream(outputDir string, config stack.StackConfig, data *stack.TemplateData) error {
	outputPath := filepath.Join(outputDir, "litestream.yml")
	return g.generateFile(outputPath, litestreamTemplate, data)
}

// Validate validates Docker deployment configuration
func (g *Generator) Validate(ctx context.Context, stackDir string) error {
	// TODO: Implement validation
	return nil
}

// GetInfo returns information about the Docker stack
func (g *Generator) GetInfo(ctx context.Context, stackDir string) (*stack.StackInfo, error) {
	// TODO: Implement info gathering
	return nil, nil
}
