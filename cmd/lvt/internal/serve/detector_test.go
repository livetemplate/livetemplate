package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModeDetector_DetectMode(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(dir string)
		wantMode  ServeMode
		wantErr   bool
	}{
		{
			name: "detects component mode",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "component.yaml"), []byte("name: test"), 0644)
			},
			wantMode: ModeComponent,
			wantErr:  false,
		},
		{
			name: "detects component mode with tmpl file",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "component.tmpl"), []byte("<div>test</div>"), 0644)
			},
			wantMode: ModeComponent,
			wantErr:  false,
		},
		{
			name: "detects kit mode",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte("name: test"), 0644)
			},
			wantMode: ModeKit,
			wantErr:  false,
		},
		{
			name: "detects app mode with go.mod",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			wantMode: ModeApp,
			wantErr:  false,
		},
		{
			name: "detects app mode with main.go",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
			},
			wantMode: ModeApp,
			wantErr:  false,
		},
		{
			name: "prefers component over app",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "component.yaml"), []byte("name: test"), 0644)
				_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			wantMode: ModeComponent,
			wantErr:  false,
		},
		{
			name: "prefers kit over app",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte("name: test"), 0644)
				_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			wantMode: ModeKit,
			wantErr:  false,
		},
		{
			name:      "fails when no mode detected",
			setupFunc: func(dir string) {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(tmpDir)

			detector := NewModeDetector(tmpDir)
			gotMode, err := detector.DetectMode()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if gotMode != tt.wantMode {
				t.Errorf("DetectMode() = %v, want %v", gotMode, tt.wantMode)
			}
		})
	}
}

func TestModeDetector_ValidateMode(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(dir string)
		mode      ServeMode
		wantErr   bool
	}{
		{
			name: "validates component mode",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "component.yaml"), []byte("name: test"), 0644)
			},
			mode:    ModeComponent,
			wantErr: false,
		},
		{
			name: "validates kit mode",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte("name: test"), 0644)
			},
			mode:    ModeKit,
			wantErr: false,
		},
		{
			name: "validates app mode",
			setupFunc: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			mode:    ModeApp,
			wantErr: false,
		},
		{
			name:      "fails for invalid component mode",
			setupFunc: func(dir string) {},
			mode:      ModeComponent,
			wantErr:   true,
		},
		{
			name:      "fails for invalid mode string",
			setupFunc: func(dir string) {},
			mode:      "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(tmpDir)

			detector := NewModeDetector(tmpDir)
			err := detector.ValidateMode(tt.mode)

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestModeDetector_GetModeInfo(t *testing.T) {
	detector := NewModeDetector(".")

	tests := []struct {
		mode ServeMode
		want string
	}{
		{ModeComponent, "Component development mode - Live preview of component templates"},
		{ModeKit, "Kit development mode - Live CSS and helper testing"},
		{ModeApp, "App development mode - Full Go application with hot reload"},
		{"unknown", "Unknown mode"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got := detector.GetModeInfo(tt.mode)
			if got != tt.want {
				t.Errorf("GetModeInfo(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}
