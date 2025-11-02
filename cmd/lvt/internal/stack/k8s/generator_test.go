package k8s

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check expected files exist
	expectedFiles := []string{
		"namespace.yaml",
		"deployment.yaml",
		"service.yaml",
		"configmap.yaml",
		"secret.yaml.example",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}
}

func TestGenerator_Generate_SQLiteWithPVC(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check PVC exists for SQLite
	pvcPath := filepath.Join(tmpDir, "pvc.yaml")
	if _, err := os.Stat(pvcPath); os.IsNotExist(err) {
		t.Errorf("Expected pvc.yaml for SQLite database")
	}
}

func TestGenerator_Generate_WithLitestream(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupLitestream,
		Storage:   stack.StorageS3,
		Redis:     stack.RedisNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check litestream-configmap.yaml exists
	litestreamPath := filepath.Join(tmpDir, "litestream-configmap.yaml")
	if _, err := os.Stat(litestreamPath); os.IsNotExist(err) {
		t.Errorf("Expected litestream-configmap.yaml does not exist")
	}
}

func TestGenerator_Generate_WithIngress(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNginx,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check ingress.yaml exists
	ingressPath := filepath.Join(tmpDir, "ingress.yaml")
	if _, err := os.Stat(ingressPath); os.IsNotExist(err) {
		t.Errorf("Expected ingress.yaml does not exist")
	}

	// Verify it contains nginx annotations
	content, err := os.ReadFile(ingressPath)
	if err != nil {
		t.Fatalf("Failed to read ingress.yaml: %v", err)
	}

	if !strings.Contains(string(content), "nginx") {
		t.Errorf("Expected nginx ingress class or annotations in ingress.yaml")
	}
}

func TestGenerator_Generate_WithPostgres(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabasePostgres,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check that PVC is NOT generated for Postgres (external DB)
	pvcPath := filepath.Join(tmpDir, "pvc.yaml")
	if _, err := os.Stat(pvcPath); err == nil {
		t.Errorf("PVC should not exist for Postgres (external database)")
	}

	// Verify deployment contains Postgres env vars
	deploymentPath := filepath.Join(tmpDir, "deployment.yaml")
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("Failed to read deployment.yaml: %v", err)
	}

	if !strings.Contains(string(content), "DATABASE_URL") {
		t.Errorf("Expected DATABASE_URL in deployment for Postgres")
	}
}

func TestGenerator_Generate_RegistryTypes(t *testing.T) {
	tests := []struct {
		name     string
		registry stack.RegistryType
		expected string
	}{
		{
			name:     "ghcr",
			registry: stack.RegistryGHCR,
			expected: "ghcr.io",
		},
		{
			name:     "docker",
			registry: stack.RegistryDocker,
			expected: "docker.io",
		},
		{
			name:     "gcr",
			registry: stack.RegistryGCR,
			expected: "gcr.io",
		},
		{
			name:     "ecr",
			registry: stack.RegistryECR,
			expected: "amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			config := stack.StackConfig{
				Provider:  stack.ProviderK8s,
				Database:  stack.DatabaseSQLite,
				Backup:    stack.BackupNone,
				Redis:     stack.RedisNone,
				Storage:   stack.StorageNone,
				CI:        stack.CINone,
				Namespace: "myapp",
				Ingress:   stack.IngressNone,
				Registry:  tt.registry,
			}

			gen := New()
			ctx := context.Background()

			err := gen.Generate(ctx, config, tmpDir)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			// Verify deployment contains correct registry
			deploymentPath := filepath.Join(tmpDir, "deployment.yaml")
			content, err := os.ReadFile(deploymentPath)
			if err != nil {
				t.Fatalf("Failed to read deployment.yaml: %v", err)
			}

			if !strings.Contains(string(content), tt.expected) {
				t.Errorf("Expected %s registry reference in deployment.yaml", tt.expected)
			}
		})
	}
}

func TestGenerator_Generate_ResourceLimits(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify deployment has resource limits
	deploymentPath := filepath.Join(tmpDir, "deployment.yaml")
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("Failed to read deployment.yaml: %v", err)
	}

	requiredFields := []string{"resources:", "limits:", "requests:", "memory:", "cpu:"}
	for _, field := range requiredFields {
		if !strings.Contains(string(content), field) {
			t.Errorf("Expected %s in deployment.yaml for resource management", field)
		}
	}
}

func TestGenerator_Generate_HealthProbes(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider:  stack.ProviderK8s,
		Database:  stack.DatabaseSQLite,
		Backup:    stack.BackupNone,
		Redis:     stack.RedisNone,
		Storage:   stack.StorageNone,
		CI:        stack.CINone,
		Namespace: "myapp",
		Ingress:   stack.IngressNone,
		Registry:  stack.RegistryGHCR,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify deployment has health probes
	deploymentPath := filepath.Join(tmpDir, "deployment.yaml")
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("Failed to read deployment.yaml: %v", err)
	}

	requiredProbes := []string{"livenessProbe:", "readinessProbe:"}
	for _, probe := range requiredProbes {
		if !strings.Contains(string(content), probe) {
			t.Errorf("Expected %s in deployment.yaml for health checks", probe)
		}
	}
}
