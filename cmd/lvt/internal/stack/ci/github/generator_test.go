package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerateWorkflow_Docker(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
		Database: stack.DatabaseSQLite,
		CI:       stack.CIGitHub,
	}

	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.GenerateWorkflow(config, tmpDir, data); err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")

	testPath := filepath.Join(workflowDir, "test.yml")
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("test.yml not created")
	}

	deployPath := filepath.Join(workflowDir, "deploy-docker.yml")
	if _, err := os.Stat(deployPath); os.IsNotExist(err) {
		t.Errorf("deploy-docker.yml not created")
	}

	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("failed to read deploy-docker.yml: %v", err)
	}

	if !strings.Contains(string(content), "Deploy Docker") {
		t.Errorf("deploy-docker.yml missing expected name")
	}
	if !strings.Contains(string(content), "docker compose") {
		t.Errorf("deploy-docker.yml missing docker compose commands")
	}
}

func TestGenerateWorkflow_Fly(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderFly,
		Database: stack.DatabasePostgres,
		CI:       stack.CIGitHub,
	}

	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.GenerateWorkflow(config, tmpDir, data); err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")

	deployPath := filepath.Join(workflowDir, "deploy-fly.yml")
	if _, err := os.Stat(deployPath); os.IsNotExist(err) {
		t.Errorf("deploy-fly.yml not created")
	}

	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("failed to read deploy-fly.yml: %v", err)
	}

	if !strings.Contains(string(content), "Deploy Fly.io") {
		t.Errorf("deploy-fly.yml missing expected name")
	}
	if !strings.Contains(string(content), "flyctl deploy") {
		t.Errorf("deploy-fly.yml missing flyctl commands")
	}
	if !strings.Contains(string(content), "FLY_API_TOKEN") {
		t.Errorf("deploy-fly.yml missing FLY_API_TOKEN secret")
	}
}

func TestGenerateWorkflow_DigitalOcean(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDigitalOcean,
		Database: stack.DatabasePostgres,
		CI:       stack.CIGitHub,
	}

	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.GenerateWorkflow(config, tmpDir, data); err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")

	deployPath := filepath.Join(workflowDir, "deploy-do.yml")
	if _, err := os.Stat(deployPath); os.IsNotExist(err) {
		t.Errorf("deploy-do.yml not created")
	}

	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("failed to read deploy-do.yml: %v", err)
	}

	if !strings.Contains(string(content), "Deploy DigitalOcean") {
		t.Errorf("deploy-do.yml missing expected name")
	}
	if !strings.Contains(string(content), "doctl") {
		t.Errorf("deploy-do.yml missing doctl commands")
	}
	if !strings.Contains(string(content), "DIGITALOCEAN_ACCESS_TOKEN") {
		t.Errorf("deploy-do.yml missing DIGITALOCEAN_ACCESS_TOKEN secret")
	}
}

func TestGenerateWorkflow_K8s_GHCR(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabasePostgres,
		CI:        stack.CIGitHub,
		Namespace: "testapp",
		Ingress:   stack.IngressNginx,
		Registry:  stack.RegistryGHCR,
	}

	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.GenerateWorkflow(config, tmpDir, data); err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")

	deployPath := filepath.Join(workflowDir, "deploy-k8s.yml")
	if _, err := os.Stat(deployPath); os.IsNotExist(err) {
		t.Errorf("deploy-k8s.yml not created")
	}

	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("failed to read deploy-k8s.yml: %v", err)
	}

	if !strings.Contains(string(content), "Deploy Kubernetes") {
		t.Errorf("deploy-k8s.yml missing expected name")
	}
	if !strings.Contains(string(content), "kubectl apply") {
		t.Errorf("deploy-k8s.yml missing kubectl commands")
	}
	if !strings.Contains(string(content), "ghcr.io") {
		t.Errorf("deploy-k8s.yml missing ghcr.io registry")
	}
	if !strings.Contains(string(content), "ingress.yaml") {
		t.Errorf("deploy-k8s.yml missing ingress deployment")
	}
}

func TestGenerateWorkflow_K8s_DockerHub(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		CI:        stack.CIGitHub,
		Namespace: "testapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryDocker,
	}

	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.GenerateWorkflow(config, tmpDir, data); err != nil {
		t.Fatalf("GenerateWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".github", "workflows")

	deployPath := filepath.Join(workflowDir, "deploy-k8s.yml")
	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("failed to read deploy-k8s.yml: %v", err)
	}

	if !strings.Contains(string(content), "DOCKER_USERNAME") {
		t.Errorf("deploy-k8s.yml missing DOCKER_USERNAME secret for Docker Hub")
	}
	if !strings.Contains(string(content), "DOCKER_PASSWORD") {
		t.Errorf("deploy-k8s.yml missing DOCKER_PASSWORD secret for Docker Hub")
	}
	if !strings.Contains(string(content), "pvc.yaml") {
		t.Errorf("deploy-k8s.yml missing pvc deployment for SQLite")
	}
}

func TestGenerateTestWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatalf("failed to create workflow dir: %v", err)
	}

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
	}
	data := config.ToTemplateData("testproject")

	gen := New()
	if err := gen.generateTestWorkflow(workflowDir, data); err != nil {
		t.Fatalf("generateTestWorkflow failed: %v", err)
	}

	testPath := filepath.Join(workflowDir, "test.yml")
	content, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read test.yml: %v", err)
	}

	if !strings.Contains(string(content), "name: Test") {
		t.Errorf("test.yml missing expected name")
	}
	if !strings.Contains(string(content), "go test") {
		t.Errorf("test.yml missing go test command")
	}
	if !strings.Contains(string(content), "golangci-lint") {
		t.Errorf("test.yml missing linting step")
	}
}

func TestGetRequiredSecrets_Docker(t *testing.T) {
	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
	}

	secrets := GetRequiredSecrets(config)

	if len(secrets) == 0 {
		t.Errorf("expected secrets, got none")
	}

	secretsStr := strings.Join(secrets, " ")
	if !strings.Contains(secretsStr, "DEPLOY_HOST") {
		t.Errorf("missing DEPLOY_HOST in secrets")
	}
	if !strings.Contains(secretsStr, "DEPLOY_SSH_KEY") {
		t.Errorf("missing DEPLOY_SSH_KEY in secrets")
	}
}

func TestGetRequiredSecrets_Fly(t *testing.T) {
	config := stack.StackConfig{
		Provider: stack.ProviderFly,
	}

	secrets := GetRequiredSecrets(config)
	secretsStr := strings.Join(secrets, " ")

	if !strings.Contains(secretsStr, "FLY_API_TOKEN") {
		t.Errorf("missing FLY_API_TOKEN in secrets")
	}
}

func TestGetRequiredSecrets_K8s_GHCR(t *testing.T) {
	config := stack.StackConfig{
		Provider: stack.ProviderK8s,
		Registry: stack.RegistryGHCR,
	}

	secrets := GetRequiredSecrets(config)
	secretsStr := strings.Join(secrets, " ")

	if !strings.Contains(secretsStr, "KUBE_CONFIG") {
		t.Errorf("missing KUBE_CONFIG in secrets")
	}
}

func TestGetRequiredSecrets_K8s_ECR(t *testing.T) {
	config := stack.StackConfig{
		Provider: stack.ProviderK8s,
		Registry: stack.RegistryECR,
	}

	secrets := GetRequiredSecrets(config)
	secretsStr := strings.Join(secrets, " ")

	if !strings.Contains(secretsStr, "AWS_ACCESS_KEY_ID") {
		t.Errorf("missing AWS_ACCESS_KEY_ID in secrets")
	}
	if !strings.Contains(secretsStr, "AWS_SECRET_ACCESS_KEY") {
		t.Errorf("missing AWS_SECRET_ACCESS_KEY in secrets")
	}
}
