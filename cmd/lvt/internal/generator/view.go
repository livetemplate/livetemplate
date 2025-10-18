package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

type ViewData struct {
	PackageName   string
	ModuleName    string
	ViewName      string
	ViewNameLower string
	Kit           *kits.KitInfo // CSS framework kit (new)
	CSSFramework  string        // CSS framework: "tailwind", "bulma", "pico", "none" (for backward compatibility)
	DevMode       bool          // Use local client library instead of CDN
}

func GenerateView(basePath, moduleName, viewName string, kitName, cssFramework string) error {
	// Load kit using KitLoader
	kitLoader := kits.DefaultLoader()
	kit, err := kitLoader.Load(kitName)
	if err != nil {
		return fmt.Errorf("failed to load kit %q: %w", kitName, err)
	}

	// Inject CSS helpers if kit is CSS-agnostic
	if kit.Helpers == nil {
		if err := kit.SetHelpersForFramework(cssFramework); err != nil {
			return fmt.Errorf("failed to load CSS helpers for framework %q: %w", cssFramework, err)
		}
	}

	// Ensure view name is capitalized
	viewName = strings.Title(viewName)
	viewNameLower := strings.ToLower(viewName)

	// Read dev mode setting from .lvtrc
	devMode := ReadDevMode(basePath)

	data := ViewData{
		PackageName:   viewNameLower,
		ModuleName:    moduleName,
		ViewName:      viewName,
		ViewNameLower: viewNameLower,
		Kit:           kit,
		CSSFramework:  cssFramework, // Keep for backward compatibility
		DevMode:       devMode,
	}

	// Create view directory
	viewDir := filepath.Join(basePath, "internal", "app", viewNameLower)
	if err := os.MkdirAll(viewDir, 0755); err != nil {
		return fmt.Errorf("failed to create view directory: %w", err)
	}

	// Read templates using kit loader (checks project kits, user kits, then embedded)
	handlerTmpl, err := kitLoader.LoadKitTemplate(kitName, "view/handler.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read handler template: %w", err)
	}

	templateTmpl, err := kitLoader.LoadKitTemplate(kitName, "view/template.tmpl.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read template template: %w", err)
	}

	testTmpl, err := kitLoader.LoadKitTemplate(kitName, "view/test.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read test template: %w", err)
	}

	// Generate handler
	if err := generateFile(string(handlerTmpl), data, filepath.Join(viewDir, viewNameLower+".go"), kit); err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}

	// Generate template
	if err := generateFile(string(templateTmpl), data, filepath.Join(viewDir, viewNameLower+".tmpl"), kit); err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	// Generate consolidated test file (E2E + WebSocket)
	if err := generateFile(string(testTmpl), data, filepath.Join(viewDir, viewNameLower+"_test.go"), kit); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}

	// Inject router registration into main.go
	mainGoPath := findMainGo(basePath)
	if mainGoPath != "" {
		route := RouteInfo{
			Path:        "/" + viewNameLower,
			PackageName: viewNameLower,
			HandlerCall: viewNameLower + ".Handler()",
			ImportPath:  moduleName + "/internal/app/" + viewNameLower,
		}
		if err := InjectRoute(mainGoPath, route); err != nil {
			// Log warning but don't fail - user can add route manually
			fmt.Printf("⚠️  Could not auto-inject route: %v\n", err)
			fmt.Printf("   Please add manually: http.Handle(\"/%s\", %s.Handler())\n",
				viewNameLower, viewNameLower)
		}
	}

	// Register view for home page
	if err := RegisterResource(basePath, data.ViewName, "/"+viewNameLower, "view"); err != nil {
		fmt.Printf("⚠️  Could not register view in home page: %v\n", err)
	}

	return nil
}
