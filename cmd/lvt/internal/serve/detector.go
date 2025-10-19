package serve

import (
	"fmt"
	"os"
	"path/filepath"
)

type ModeDetector struct {
	dir string
}

func NewModeDetector(dir string) *ModeDetector {
	return &ModeDetector{dir: dir}
}

func (d *ModeDetector) DetectMode() (ServeMode, error) {
	absDir, err := filepath.Abs(d.dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve directory: %w", err)
	}

	if d.isComponentDirectory(absDir) {
		return ModeComponent, nil
	}

	if d.isKitDirectory(absDir) {
		return ModeKit, nil
	}

	if d.isAppDirectory(absDir) {
		return ModeApp, nil
	}

	return "", fmt.Errorf("unable to detect serve mode in directory: %s", absDir)
}

func (d *ModeDetector) isComponentDirectory(dir string) bool {
	manifestPath := filepath.Join(dir, "component.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		return true
	}

	componentTmplPath := filepath.Join(dir, "component.tmpl")
	if _, err := os.Stat(componentTmplPath); err == nil {
		return true
	}

	return false
}

func (d *ModeDetector) isKitDirectory(dir string) bool {
	kitYamlPath := filepath.Join(dir, "kit.yaml")
	if _, err := os.Stat(kitYamlPath); err == nil {
		return true
	}

	helpersGoPath := filepath.Join(dir, "helpers.go")
	kitYamlExists := false
	helpersGoExists := false

	if _, err := os.Stat(kitYamlPath); err == nil {
		kitYamlExists = true
	}
	if _, err := os.Stat(helpersGoPath); err == nil {
		helpersGoExists = true
	}

	return kitYamlExists || helpersGoExists
}

func (d *ModeDetector) isAppDirectory(dir string) bool {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		if !d.isComponentDirectory(dir) && !d.isKitDirectory(dir) {
			return true
		}
	}

	mainGoPath := filepath.Join(dir, "main.go")
	if _, err := os.Stat(mainGoPath); err == nil {
		if !d.isComponentDirectory(dir) && !d.isKitDirectory(dir) {
			return true
		}
	}

	return false
}

func (d *ModeDetector) GetModeInfo(mode ServeMode) string {
	switch mode {
	case ModeComponent:
		return "Component development mode - Live preview of component templates"
	case ModeKit:
		return "Kit development mode - Live CSS and helper testing"
	case ModeApp:
		return "App development mode - Full Go application with hot reload"
	default:
		return "Unknown mode"
	}
}

func (d *ModeDetector) ValidateMode(mode ServeMode) error {
	switch mode {
	case ModeComponent:
		if !d.isComponentDirectory(d.dir) {
			return fmt.Errorf("directory is not a valid component directory")
		}
	case ModeKit:
		if !d.isKitDirectory(d.dir) {
			return fmt.Errorf("directory is not a valid kit directory")
		}
	case ModeApp:
		if !d.isAppDirectory(d.dir) {
			return fmt.Errorf("directory is not a valid app directory")
		}
	default:
		return fmt.Errorf("invalid serve mode: %s", mode)
	}
	return nil
}
