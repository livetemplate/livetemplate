package serve

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

type KitMode struct {
	server *Server
	kit    *kits.KitInfo
}

func NewKitMode(s *Server) (*KitMode, error) {
	km := &KitMode{
		server: s,
	}

	if err := km.loadKit(); err != nil {
		return nil, err
	}

	return km, nil
}

func (km *KitMode) loadKit() error {
	kitYamlPath := filepath.Join(km.server.config.Dir, "kit.yaml")
	if _, err := os.Stat(kitYamlPath); os.IsNotExist(err) {
		return fmt.Errorf("kit.yaml not found in directory: %s", km.server.config.Dir)
	}

	kitLoader := kits.DefaultLoader()
	kitLoader.AddSearchPath(filepath.Dir(km.server.config.Dir))

	kitName := filepath.Base(km.server.config.Dir)

	kit, err := kitLoader.Load(kitName)
	if err != nil {
		return fmt.Errorf("failed to load kit: %w", err)
	}

	km.kit = kit
	log.Printf("Kit loaded: %s (version: %s)", kit.Manifest.Name, kit.Manifest.Version)
	return nil
}

func (km *KitMode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/test":
		km.handleTest(w, r)
	case "/helpers":
		km.handleHelpers(w, r)
	default:
		km.handleIndex(w, r)
	}
}

func (km *KitMode) handleIndex(w http.ResponseWriter, r *http.Request) {
	manifest := km.kit.Manifest

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Kit Development - ` + manifest.Name + `</title>
	` + manifest.CDN + `
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			height: 100vh;
			display: flex;
			flex-direction: column;
		}
		.header {
			background: #2c3e50;
			color: white;
			padding: 1rem 2rem;
			display: flex;
			justify-content: space-between;
			align-items: center;
			border-bottom: 3px solid #9b59b6;
		}
		.header h1 {
			font-size: 1.5rem;
			font-weight: 600;
		}
		.status {
			display: flex;
			align-items: center;
			gap: 0.5rem;
		}
		.status-dot {
			width: 8px;
			height: 8px;
			border-radius: 50%;
			background: #2ecc71;
		}
		.status-dot.disconnected {
			background: #e74c3c;
		}
		.container {
			display: flex;
			flex: 1;
			overflow: hidden;
		}
		.sidebar {
			width: 300px;
			background: #f8f9fa;
			border-right: 1px solid #ddd;
			overflow-y: auto;
		}
		.main-panel {
			flex: 1;
			display: flex;
			flex-direction: column;
		}
		.panel-header {
			background: #ecf0f1;
			padding: 0.75rem 1rem;
			border-bottom: 1px solid #bdc3c7;
			font-weight: 600;
		}
		.panel-content {
			flex: 1;
			overflow: auto;
			padding: 2rem;
		}
		.info-section {
			padding: 1rem;
			border-bottom: 1px solid #ddd;
		}
		.info-section h3 {
			font-size: 0.85rem;
			text-transform: uppercase;
			color: #7f8c8d;
			margin-bottom: 0.5rem;
		}
		.info-item {
			padding: 0.5rem 0;
		}
		.info-label {
			font-weight: 600;
			color: #2c3e50;
		}
		.helper-list {
			list-style: none;
			padding: 1rem;
		}
		.helper-item {
			padding: 0.5rem;
			margin: 0.25rem 0;
			background: white;
			border: 1px solid #ddd;
			border-radius: 4px;
			font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
			font-size: 0.85rem;
		}
		.test-grid {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
			gap: 2rem;
		}
		.test-card {
			border: 1px solid #ddd;
			border-radius: 8px;
			overflow: hidden;
		}
		.test-card-header {
			background: #f8f9fa;
			padding: 0.75rem 1rem;
			font-weight: 600;
			border-bottom: 1px solid #ddd;
		}
		.test-card-body {
			padding: 1rem;
		}
	</style>
</head>
<body>
	<div class="header">
		<h1>Kit Development - ` + manifest.Name + `</h1>
		<div class="status">
			<div class="status-dot" id="statusDot"></div>
			<span id="statusText">Connected</span>
		</div>
	</div>
	<div class="container">
		<div class="sidebar">
			<div class="info-section">
				<h3>Kit Information</h3>
				<div class="info-item">
					<div class="info-label">Name</div>
					<div>` + manifest.Name + `</div>
				</div>
				<div class="info-item">
					<div class="info-label">Version</div>
					<div>` + manifest.Version + `</div>
				</div>
				<div class="info-item">
					<div class="info-label">Framework</div>
					<div>` + manifest.Framework + `</div>
				</div>
				<div class="info-item">
					<div class="info-label">Author</div>
					<div>` + manifest.Author + `</div>
				</div>
			</div>
			<div class="info-section">
				<h3>Helper Methods</h3>
				<ul class="helper-list" id="helpersList">
					<li class="helper-item">Loading...</li>
				</ul>
			</div>
		</div>
		<div class="main-panel">
			<div class="panel-header">Component Examples</div>
			<div class="panel-content">
				<div class="test-grid" id="testGrid">
					` + km.generateTestCards() + `
				</div>
			</div>
		</div>
	</div>
	<script>
		const ws = new WebSocket('ws://' + location.host + '/ws');
		const statusDot = document.getElementById('statusDot');
		const statusText = document.getElementById('statusText');

		ws.onopen = () => {
			statusDot.classList.remove('disconnected');
			statusText.textContent = 'Connected';
			loadHelpers();
		};

		ws.onmessage = (event) => {
			const data = JSON.parse(event.data);
			if (data.type === 'reload') {
				console.log('Reloading kit...');
				window.location.reload();
			}
		};

		ws.onclose = () => {
			statusDot.classList.add('disconnected');
			statusText.textContent = 'Disconnected';
			setTimeout(() => window.location.reload(), 1000);
		};

		async function loadHelpers() {
			try {
				const response = await fetch('/helpers');
				const helpers = await response.json();
				const helpersList = document.getElementById('helpersList');
				helpersList.innerHTML = helpers.map(h =>
					'<li class="helper-item">' + h + '()</li>'
				).join('');
			} catch (e) {
				console.error('Failed to load helpers:', e);
			}
		}
	</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (km *KitMode) generateTestCards() string {
	helpers := km.kit.Helpers

	cards := []string{
		km.testCard("Container", helpers.ContainerClass()),
		km.testCard("Section", helpers.SectionClass()),
		km.testCard("Columns", helpers.ColumnsClass()),
		km.testCard("Button Primary", helpers.ButtonClass("primary")),
		km.testCard("Button Secondary", helpers.ButtonClass("secondary")),
		km.testCard("Card", helpers.CardClass()),
		km.testCard("Table", helpers.TableClass()),
		km.testCard("Field", helpers.FieldClass()),
		km.testCard("Input", helpers.InputClass()),
	}

	return strings.Join(cards, "\n")
}

func (km *KitMode) testCard(title, class string) string {
	return fmt.Sprintf(`
		<div class="test-card">
			<div class="test-card-header">%s</div>
			<div class="test-card-body">
				<div class="%s" style="padding: 1rem; background: #f0f0f0; border: 1px dashed #ccc;">
					<code>class="%s"</code>
				</div>
			</div>
		</div>
	`, title, class, class)
}

func (km *KitMode) handleTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte("<p>Test placeholder</p>")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (km *KitMode) handleHelpers(w http.ResponseWriter, r *http.Request) {
	helpers := []string{
		"ContainerClass",
		"RowClass",
		"ColClass",
		"ButtonClass",
		"CardClass",
		"TableClass",
		"FormGroupClass",
		"InputClass",
		"LabelClass",
		"SelectClass",
		"TextareaClass",
		"AlertClass",
		"BadgeClass",
		"NavbarClass",
		"NavItemClass",
		"NavLinkClass",
		"PaginationClass",
		"PaginationItemClass",
		"PaginationLinkClass",
		"ModalClass",
		"ModalDialogClass",
		"ModalContentClass",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(helpers)
}

func (km *KitMode) Reload() error {
	return km.loadKit()
}
