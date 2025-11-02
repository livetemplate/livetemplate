package k8s

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack/ci/github"
)

//go:embed templates/namespace.yaml.tmpl
var namespaceTemplate string

//go:embed templates/deployment.yaml.tmpl
var deploymentTemplate string

//go:embed templates/service.yaml.tmpl
var serviceTemplate string

//go:embed templates/ingress.yaml.tmpl
var ingressTemplate string

//go:embed templates/pvc.yaml.tmpl
var pvcTemplate string

//go:embed templates/configmap.yaml.tmpl
var configmapTemplate string

//go:embed templates/secret.yaml.example.tmpl
var secretTemplate string

//go:embed templates/litestream-configmap.yaml.tmpl
var litestreamConfigmapTemplate string

//go:embed templates/README.md.tmpl
var readmeTemplate string

type Generator struct{}

func New() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(ctx context.Context, config stack.StackConfig, outputDir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	projectName := filepath.Base(wd)

	data := config.ToTemplateData(projectName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Always generate these files
	files := map[string]string{
		"namespace.yaml":      namespaceTemplate,
		"deployment.yaml":     deploymentTemplate,
		"service.yaml":        serviceTemplate,
		"configmap.yaml":      configmapTemplate,
		"secret.yaml.example": secretTemplate,
		"README.md":           readmeTemplate,
	}

	for filename, tmplContent := range files {
		if err := g.generateFile(filepath.Join(outputDir, filename), tmplContent, data); err != nil {
			return fmt.Errorf("failed to generate %s: %w", filename, err)
		}
	}

	// Generate PVC only for SQLite (needs persistent storage)
	if config.Database == stack.DatabaseSQLite {
		if err := g.generateFile(filepath.Join(outputDir, "pvc.yaml"), pvcTemplate, data); err != nil {
			return fmt.Errorf("failed to generate pvc.yaml: %w", err)
		}
	}

	// Generate ingress if configured
	if config.Ingress != stack.IngressNone && config.Ingress != "" {
		if err := g.generateFile(filepath.Join(outputDir, "ingress.yaml"), ingressTemplate, data); err != nil {
			return fmt.Errorf("failed to generate ingress.yaml: %w", err)
		}
	}

	// Generate litestream configmap if backup enabled
	if config.Backup == stack.BackupLitestream {
		if err := g.generateFile(filepath.Join(outputDir, "litestream-configmap.yaml"), litestreamConfigmapTemplate, data); err != nil {
			return fmt.Errorf("failed to generate litestream-configmap.yaml: %w", err)
		}
	}

	// Generate CI/CD workflows if configured
	if config.CI == stack.CIGitHub {
		ciGen := github.New()
		projectDir := filepath.Dir(outputDir)
		if err := ciGen.GenerateWorkflow(config, projectDir, data); err != nil {
			return fmt.Errorf("failed to generate CI workflows: %w", err)
		}
	}

	return nil
}

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

func (g *Generator) Validate(ctx context.Context, stackDir string) error {
	// TODO: Implement validation
	return nil
}

func (g *Generator) GetInfo(ctx context.Context, stackDir string) (*stack.StackInfo, error) {
	// TODO: Implement info gathering
	return nil, nil
}
