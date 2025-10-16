package serve

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/components"
	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
	"gopkg.in/yaml.v3"
)

type ComponentMode struct {
	server    *Server
	component *components.Component
	kit       *kits.KitInfo
	tmpl      *template.Template
}

type ComponentManifest struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Category    string   `yaml:"category"`
	Kit         string   `yaml:"kit"`
	Tags        []string `yaml:"tags"`
	Templates   []string `yaml:"templates"`
}

func NewComponentMode(s *Server) (*ComponentMode, error) {
	cm := &ComponentMode{
		server: s,
	}

	if err := cm.loadComponent(); err != nil {
		return nil, err
	}

	return cm, nil
}

func (cm *ComponentMode) loadComponent() error {
	manifestPath := filepath.Join(cm.server.config.Dir, "component.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read component.yaml: %w", err)
	}

	var manifest ComponentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse component.yaml: %w", err)
	}

	if manifest.Kit == "" {
		manifest.Kit = "tailwind"
	}

	kitLoader := kits.DefaultLoader()
	kit, err := kitLoader.Load(manifest.Kit)
	if err != nil {
		log.Printf("Warning: Failed to load kit %s, using default: %v", manifest.Kit, err)
		kit, _ = kitLoader.Load("tailwind")
	}
	cm.kit = kit

	if len(manifest.Templates) == 0 {
		return fmt.Errorf("no templates defined in component.yaml")
	}

	templatePath := filepath.Join(cm.server.config.Dir, manifest.Templates[0])
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template file not found: %s", templatePath)
	}

	tmpl, err := cm.parseTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	cm.tmpl = tmpl

	log.Printf("Component loaded: %s (kit: %s)", manifest.Name, manifest.Kit)
	return nil
}

func (cm *ComponentMode) parseTemplate(path string) (*template.Template, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	templateContent := string(content)
	templateContent = strings.ReplaceAll(templateContent, "[[", "{{")
	templateContent = strings.ReplaceAll(templateContent, "]]", "}}")

	tmpl := template.New(filepath.Base(path))

	if cm.kit != nil && cm.kit.Helpers != nil {
		funcs := createTemplateFuncs(cm.kit.Helpers)
		tmpl.Funcs(funcs)
	}

	tmpl, err = tmpl.Parse(templateContent)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func (cm *ComponentMode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/preview":
		cm.handlePreview(w, r)
	case "/render":
		cm.handleRender(w, r)
	case "/reload":
		cm.handleReload(w, r)
	default:
		cm.handleIndex(w, r)
	}
}

func (cm *ComponentMode) handleIndex(w http.ResponseWriter, r *http.Request) {
	cdn := ""
	kitName := "none"
	if cm.kit != nil {
		cdn = cm.kit.Manifest.CDN
		kitName = cm.kit.Manifest.Name
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Component Development - LiveTemplate</title>
	` + cdn + `
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
			border-bottom: 3px solid #3498db;
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
		.editor-panel {
			flex: 1;
			display: flex;
			flex-direction: column;
			border-right: 1px solid #ddd;
		}
		.preview-panel {
			flex: 1;
			display: flex;
			flex-direction: column;
			background: #f5f5f5;
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
			padding: 1rem;
		}
		.preview-frame {
			background: white;
			border: 1px solid #ddd;
			border-radius: 4px;
			padding: 2rem;
			min-height: 200px;
		}
		textarea {
			width: 100%;
			height: 100%;
			border: none;
			padding: 1rem;
			font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
			font-size: 14px;
			resize: none;
			outline: none;
		}
		.error-message {
			background: #fee;
			color: #c33;
			padding: 1rem;
			border-left: 4px solid #c33;
			margin-bottom: 1rem;
		}
		.info {
			background: #e3f2fd;
			padding: 1rem;
			border-radius: 4px;
			margin-bottom: 1rem;
		}
	</style>
</head>
<body>
	<div class="header">
		<h1>Component Development</h1>
		<div class="status">
			<div class="status-dot" id="statusDot"></div>
			<span id="statusText">Connected</span>
		</div>
	</div>
	<div class="container">
		<div class="editor-panel">
			<div class="panel-header">Test Data (JSON)</div>
			<div class="panel-content">
				<textarea id="dataEditor" placeholder='Enter test data as JSON, e.g., {"name": "John", "items": [...]}'>{}</textarea>
			</div>
		</div>
		<div class="preview-panel">
			<div class="panel-header">Live Preview</div>
			<div class="panel-content">
				<div id="error" class="error-message" style="display: none;"></div>
				<div class="info">
					<strong>Component Directory:</strong> ` + cm.server.config.Dir + `<br>
					<strong>Kit:</strong> ` + kitName + `
				</div>
				<div class="preview-frame" id="preview">
					<p style="color: #999;">Component preview will appear here...</p>
				</div>
			</div>
		</div>
	</div>
	<script>
		const ws = new WebSocket('ws://' + location.host + '/ws');
		const statusDot = document.getElementById('statusDot');
		const statusText = document.getElementById('statusText');
		const dataEditor = document.getElementById('dataEditor');
		const preview = document.getElementById('preview');
		const errorDiv = document.getElementById('error');

		ws.onopen = () => {
			statusDot.classList.remove('disconnected');
			statusText.textContent = 'Connected';
			renderPreview();
		};

		ws.onmessage = (event) => {
			const data = JSON.parse(event.data);
			if (data.type === 'reload') {
				console.log('Reloading component...');
				window.location.reload();
			}
		};

		ws.onclose = () => {
			statusDot.classList.add('disconnected');
			statusText.textContent = 'Disconnected';
			setTimeout(() => window.location.reload(), 1000);
		};

		dataEditor.addEventListener('input', debounce(renderPreview, 500));

		function debounce(func, wait) {
			let timeout;
			return function executedFunction(...args) {
				const later = () => {
					clearTimeout(timeout);
					func(...args);
				};
				clearTimeout(timeout);
				timeout = setTimeout(later, wait);
			};
		}

		async function renderPreview() {
			let data = {};
			try {
				const text = dataEditor.value.trim();
				if (text) {
					data = JSON.parse(text);
				}
			} catch (e) {
				showError('Invalid JSON: ' + e.message);
				return;
			}

			try {
				const response = await fetch('/render', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify(data)
				});

				if (!response.ok) {
					const error = await response.text();
					showError('Render error: ' + error);
					return;
				}

				const html = await response.text();
				preview.innerHTML = html;
				hideError();
			} catch (e) {
				showError('Failed to render: ' + e.message);
			}
		}

		function showError(message) {
			errorDiv.textContent = message;
			errorDiv.style.display = 'block';
		}

		function hideError() {
			errorDiv.style.display = 'none';
		}
	</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (cm *ComponentMode) handlePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<p>Preview placeholder</p>"))
}

func (cm *ComponentMode) handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
	}

	if err := cm.loadComponent(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reload component: %v", err), http.StatusInternalServerError)
		return
	}

	var buf strings.Builder
	if err := cm.tmpl.Execute(&buf, data); err != nil {
		http.Error(w, fmt.Sprintf("Template execution error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(buf.String()))
}

func (cm *ComponentMode) handleReload(w http.ResponseWriter, r *http.Request) {
	if err := cm.loadComponent(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reload: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "reloaded",
	})
}
