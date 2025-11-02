package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func setupTestDir(t *testing.T) (cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Error(err)
		}
	}
}

func TestStack_Subcommands(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "no subcommand",
			args:        []string{},
			wantErr:     true,
			errContains: "subcommand required",
		},
		{
			name:        "unknown subcommand",
			args:        []string{"unknown"},
			wantErr:     true,
			errContains: "unknown subcommand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Stack(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Stack() error = nil, want error")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Stack() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Stack() unexpected error = %v", err)
			}
		})
	}
}

func TestStackValidate_NoStack(t *testing.T) {
	defer setupTestDir(t)()

	err := StackValidate([]string{})
	if err == nil {
		t.Fatal("StackValidate() should fail when no stack exists")
	}
	if !contains(err.Error(), "no stack found") {
		t.Errorf("Error should mention 'no stack found', got: %v", err)
	}
}

func TestStackValidate_WithStack(t *testing.T) {
	defer setupTestDir(t)()

	// Generate a stack first
	err := GenStack([]string{"docker"})
	if err != nil {
		t.Fatalf("GenStack() error = %v", err)
	}

	// Validate should succeed
	err = StackValidate([]string{})
	if err != nil {
		t.Fatalf("StackValidate() error = %v", err)
	}
}

func TestStackValidate_ModifiedFiles(t *testing.T) {
	defer setupTestDir(t)()

	// Generate a stack
	err := GenStack([]string{"docker"})
	if err != nil {
		t.Fatalf("GenStack() error = %v", err)
	}

	// Modify a file
	wd, _ := os.Getwd()
	dockerComposePath := filepath.Join(wd, "deploy", "docker-compose.yml")
	content, err := os.ReadFile(dockerComposePath)
	if err != nil {
		t.Fatalf("Failed to read docker-compose.yml: %v", err)
	}

	// Append something to modify the file
	err = os.WriteFile(dockerComposePath, append(content, []byte("\n# Modified")...), 0644)
	if err != nil {
		t.Fatalf("Failed to write docker-compose.yml: %v", err)
	}

	// Validate should still succeed but report modifications
	// (Note: In a real implementation, this might print warnings)
	err = StackValidate([]string{})
	if err != nil {
		t.Fatalf("StackValidate() error = %v (should succeed even with modifications)", err)
	}
}

func TestStackInfo_NoStack(t *testing.T) {
	defer setupTestDir(t)()

	err := StackInfo([]string{})
	if err == nil {
		t.Fatal("StackInfo() should fail when no stack exists")
	}
	if !contains(err.Error(), "no stack found") {
		t.Errorf("Error should mention 'no stack found', got: %v", err)
	}
}

func TestStackInfo_WithStack(t *testing.T) {
	defer setupTestDir(t)()

	// Generate a stack first
	err := GenStack([]string{"fly", "--db", "postgres", "--redis", "upstash", "--multi-region"})
	if err != nil {
		t.Fatalf("GenStack() error = %v", err)
	}

	// Info should succeed
	err = StackInfo([]string{})
	if err != nil {
		t.Fatalf("StackInfo() error = %v", err)
	}
}

func TestStackInfo_AllProviders(t *testing.T) {
	providers := []string{"docker", "fly", "do", "k8s"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			defer setupTestDir(t)()

			// Generate stack
			err := GenStack([]string{provider})
			if err != nil {
				t.Fatalf("GenStack(%s) error = %v", provider, err)
			}

			// Get info
			err = StackInfo([]string{})
			if err != nil {
				t.Fatalf("StackInfo(%s) error = %v", provider, err)
			}
		})
	}
}

func TestCreateGenerator(t *testing.T) {
	tests := []struct {
		name     string
		provider stack.Provider
		wantErr  bool
	}{
		{
			name:     "docker",
			provider: stack.ProviderDocker,
			wantErr:  false,
		},
		{
			name:     "fly",
			provider: stack.ProviderFly,
			wantErr:  false,
		},
		{
			name:     "do",
			provider: stack.ProviderDigitalOcean,
			wantErr:  false,
		},
		{
			name:     "k8s",
			provider: stack.ProviderK8s,
			wantErr:  false,
		},
		{
			name:     "unknown",
			provider: stack.Provider("unknown"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := createGenerator(tt.provider)

			if tt.wantErr {
				if err == nil {
					t.Fatal("createGenerator() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("createGenerator() unexpected error = %v", err)
			}

			if gen == nil {
				t.Fatal("createGenerator() returned nil generator")
			}
		})
	}
}
