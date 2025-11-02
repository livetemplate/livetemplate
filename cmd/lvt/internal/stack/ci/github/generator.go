package github

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/stack"
)

//go:embed templates/test.yml.tmpl
var testWorkflowTemplate string

//go:embed templates/deploy-docker.yml.tmpl
var deployDockerTemplate string

//go:embed templates/deploy-fly.yml.tmpl
var deployFlyTemplate string

//go:embed templates/deploy-do.yml.tmpl
var deployDOTemplate string

//go:embed templates/deploy-k8s.yml.tmpl
var deployK8sTemplate string

type Generator struct{}

func New() *Generator {
	return &Generator{}
}

func (g *Generator) GenerateWorkflow(config stack.StackConfig, projectDir string, data *stack.TemplateData) error {
	workflowDir := filepath.Join(projectDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	if err := g.generateTestWorkflow(workflowDir, data); err != nil {
		return fmt.Errorf("failed to generate test workflow: %w", err)
	}

	var deployTemplate string
	var deployFilename string

	switch config.Provider {
	case stack.ProviderDocker:
		deployTemplate = deployDockerTemplate
		deployFilename = "deploy-docker.yml"
	case stack.ProviderFly:
		deployTemplate = deployFlyTemplate
		deployFilename = "deploy-fly.yml"
	case stack.ProviderDigitalOcean:
		deployTemplate = deployDOTemplate
		deployFilename = "deploy-do.yml"
	case stack.ProviderK8s:
		deployTemplate = deployK8sTemplate
		deployFilename = "deploy-k8s.yml"
	default:
		return fmt.Errorf("unsupported provider for CI generation: %s", config.Provider)
	}

	if err := g.generateFile(filepath.Join(workflowDir, deployFilename), deployTemplate, data); err != nil {
		return fmt.Errorf("failed to generate deployment workflow: %w", err)
	}

	return nil
}

func (g *Generator) generateTestWorkflow(workflowDir string, data *stack.TemplateData) error {
	outputPath := filepath.Join(workflowDir, "test.yml")
	return g.generateFile(outputPath, testWorkflowTemplate, data)
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

func GetRequiredSecrets(config stack.StackConfig) []string {
	secrets := []string{}

	switch config.Provider {
	case stack.ProviderDocker:
		secrets = append(secrets,
			"DOCKER_REGISTRY (optional, defaults to ghcr.io)",
			"DOCKER_USERNAME (optional, defaults to github.actor)",
			"DOCKER_PASSWORD (optional, defaults to GITHUB_TOKEN)",
			"DEPLOY_HOST",
			"DEPLOY_USER",
			"DEPLOY_SSH_KEY",
			"DEPLOY_PORT (optional, defaults to 22)",
		)
	case stack.ProviderFly:
		secrets = append(secrets, "FLY_API_TOKEN")
	case stack.ProviderDigitalOcean:
		secrets = append(secrets,
			"DIGITALOCEAN_ACCESS_TOKEN",
			"DO_APP_ID",
		)
	case stack.ProviderK8s:
		secrets = append(secrets, "KUBE_CONFIG")

		switch config.Registry {
		case stack.RegistryGHCR:
			// Uses GITHUB_TOKEN automatically
		case stack.RegistryDocker:
			secrets = append(secrets, "DOCKER_USERNAME", "DOCKER_PASSWORD")
		case stack.RegistryGCR:
			secrets = append(secrets, "GCR_JSON_KEY", "GCP_PROJECT_ID")
		case stack.RegistryECR:
			secrets = append(secrets,
				"AWS_ACCESS_KEY_ID",
				"AWS_SECRET_ACCESS_KEY",
				"AWS_REGION",
				"AWS_ACCOUNT_ID",
			)
		}
	}

	secrets = append(secrets, "CODECOV_TOKEN (optional, for code coverage)")

	return secrets
}
