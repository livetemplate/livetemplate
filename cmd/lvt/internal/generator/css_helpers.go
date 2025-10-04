package generator

import "text/template"

// CSSHelpers returns template functions for CSS framework selection
func CSSHelpers() template.FuncMap {
	return template.FuncMap{
		// CDN link for framework
		"csscdn": func(framework string) string {
			switch framework {
			case "tailwind":
				return `<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>`
			case "bulma":
				return `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">`
			case "pico":
				return `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">`
			case "none":
				return ""
			default:
				return `<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>` // default to tailwind
			}
		},

		// Container classes
		"containerClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "max-w-7xl mx-auto px-4 py-8"
			case "bulma":
				return "container"
			case "pico":
				return "container"
			case "none":
				return ""
			default:
				return "max-w-7xl mx-auto px-4 py-8"
			}
		},

		// Section wrapper classes (Bulma-specific)
		"sectionClass": func(framework string) string {
			switch framework {
			case "bulma":
				return "section"
			default:
				return ""
			}
		},

		// Box/Card classes
		"boxClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "bg-white shadow rounded-lg p-6 mb-6"
			case "bulma":
				return "box"
			case "pico":
				return "" // Pico uses <article> semantically
			case "none":
				return ""
			default:
				return "bg-white shadow rounded-lg p-6 mb-6"
			}
		},

		// Title classes
		"titleClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "text-3xl font-bold text-gray-900 mb-6"
			case "bulma":
				return "title"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "text-3xl font-bold text-gray-900 mb-6"
			}
		},

		// Subtitle classes
		"subtitleClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "text-xl font-semibold text-gray-700 mb-4"
			case "bulma":
				return "subtitle"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "text-xl font-semibold text-gray-700 mb-4"
			}
		},

		// Field wrapper classes
		"fieldClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "mb-4"
			case "bulma":
				return "field"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "mb-4"
			}
		},

		// Label classes
		"labelClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "block text-sm font-medium text-gray-700 mb-2"
			case "bulma":
				return "label"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "block text-sm font-medium text-gray-700 mb-2"
			}
		},

		// Control wrapper (Bulma-specific)
		"controlClass": func(framework string) string {
			switch framework {
			case "bulma":
				return "control"
			default:
				return ""
			}
		},

		// Input classes
		"inputClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
			case "bulma":
				return "input"
			case "pico":
				return "" // Pico styles inputs automatically
			case "none":
				return ""
			default:
				return "w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
			}
		},

		// Input error classes
		"inputErrorClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "border-red-500 focus:ring-red-500"
			case "bulma":
				return "input is-danger"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "border-red-500 focus:ring-red-500"
			}
		},

		// Checkbox wrapper classes
		"checkboxClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "flex items-center"
			case "bulma":
				return "checkbox"
			case "pico":
				return ""
			case "none":
				return ""
			default:
				return "flex items-center"
			}
		},

		// Button classes
		"buttonClass": func(framework string, variant string) string {
			switch framework {
			case "tailwind":
				if variant == "primary" {
					return "bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50"
				}
				return "bg-red-600 text-white px-2 py-1 text-sm rounded hover:bg-red-700"
			case "bulma":
				if variant == "primary" {
					return "button is-primary"
				}
				return "button is-danger is-small"
			case "pico":
				return "" // Pico styles buttons automatically
			case "none":
				return ""
			default:
				if variant == "primary" {
					return "bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50"
				}
				return "bg-red-600 text-white px-2 py-1 text-sm rounded hover:bg-red-700"
			}
		},

		// Table classes
		"tableClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "min-w-full divide-y divide-gray-200"
			case "bulma":
				return "table is-fullwidth is-striped"
			case "pico":
				return "" // Pico styles tables automatically
			case "none":
				return ""
			default:
				return "min-w-full divide-y divide-gray-200"
			}
		},

		// Table header classes
		"theadClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "bg-gray-50"
			case "pico":
				return ""
			case "bulma":
				return ""
			case "none":
				return ""
			default:
				return "bg-gray-50"
			}
		},

		// Table header cell classes
		"thClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
			case "pico":
				return ""
			case "bulma":
				return ""
			case "none":
				return ""
			default:
				return "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
			}
		},

		// Table body classes
		"tbodyClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "bg-white divide-y divide-gray-200"
			default:
				return ""
			}
		},

		// Table row classes
		"trClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "hover:bg-gray-50"
			default:
				return ""
			}
		},

		// Table cell classes
		"tdClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "px-6 py-4 whitespace-nowrap text-sm text-gray-900"
			default:
				return ""
			}
		},

		// Select wrapper classes
		"selectWrapperClass": func(framework string) string {
			switch framework {
			case "bulma":
				return "select"
			default:
				return ""
			}
		},

		// Pagination wrapper classes
		"paginationClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "flex justify-between items-center mt-4"
			case "bulma":
				return "pagination"
			default:
				return ""
			}
		},

		// Help text classes
		"helpTextClass": func(framework string) string {
			switch framework {
			case "tailwind":
				return "text-sm text-red-600 mt-1"
			case "bulma":
				return "help is-danger"
			default:
				return ""
			}
		},

		// Check if framework needs semantic wrapper (like Pico's <main>)
		"needsWrapper": func(framework string) bool {
			return framework == "pico"
		},

		// Check if framework needs article tags (Pico)
		"needsArticle": func(framework string) bool {
			return framework == "pico"
		},
	}
}
