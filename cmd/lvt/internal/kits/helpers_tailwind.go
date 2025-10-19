package kits

// TailwindHelpers implements CSSHelpers for Tailwind CSS
type TailwindHelpers struct {
	BaseHelpers
}

// NewTailwindHelpers creates a new Tailwind CSS helper
func NewTailwindHelpers() CSSHelpers {
	return &TailwindHelpers{}
}

// Framework information
func (h *TailwindHelpers) CSSCDN() string {
	return `<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>`
}

// Layout helpers
func (h *TailwindHelpers) ContainerClass() string {
	return "max-w-7xl mx-auto px-4 py-8"
}

func (h *TailwindHelpers) SectionClass() string {
	return ""
}

func (h *TailwindHelpers) BoxClass() string {
	return "bg-white shadow rounded-lg p-6 mb-6"
}

func (h *TailwindHelpers) ColumnClass() string {
	return ""
}

func (h *TailwindHelpers) ColumnsClass() string {
	return "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
}

// Form helpers
func (h *TailwindHelpers) FieldClass() string {
	return "mb-4"
}

func (h *TailwindHelpers) LabelClass() string {
	return "block text-sm font-medium text-gray-700 mb-2"
}

func (h *TailwindHelpers) InputClass() string {
	return "w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
}

func (h *TailwindHelpers) TextareaClass() string {
	return "w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
}

func (h *TailwindHelpers) SelectClass() string {
	return "w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
}

func (h *TailwindHelpers) CheckboxClass() string {
	return "flex items-center"
}

func (h *TailwindHelpers) RadioClass() string {
	return "flex items-center"
}

func (h *TailwindHelpers) ButtonClass(variant string) string {
	switch variant {
	case "primary":
		return "bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50"
	case "secondary":
		return "bg-gray-600 text-white px-2 py-1 text-sm rounded hover:bg-gray-700"
	case "danger":
		return "bg-red-600 text-white px-4 py-2 rounded-md hover:bg-red-700 disabled:opacity-50"
	default:
		return "bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50"
	}
}

func (h *TailwindHelpers) ButtonGroupClass() string {
	return "flex space-x-2"
}

func (h *TailwindHelpers) FormClass() string {
	return "space-y-4"
}

// Table helpers
func (h *TailwindHelpers) TableClass() string {
	return "min-w-full divide-y divide-gray-200"
}

func (h *TailwindHelpers) TheadClass() string {
	return "bg-gray-50"
}

func (h *TailwindHelpers) TbodyClass() string {
	return "bg-white divide-y divide-gray-200"
}

func (h *TailwindHelpers) ThClass() string {
	return "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
}

func (h *TailwindHelpers) TdClass() string {
	return "px-6 py-4 whitespace-nowrap text-sm text-gray-900"
}

func (h *TailwindHelpers) TrClass() string {
	return "hover:bg-gray-50"
}

func (h *TailwindHelpers) TableContainerClass() string {
	return "overflow-x-auto"
}

// Navigation helpers
func (h *TailwindHelpers) NavbarClass() string {
	return "bg-white shadow"
}

func (h *TailwindHelpers) NavbarBrandClass() string {
	return "font-bold text-xl"
}

func (h *TailwindHelpers) NavbarMenuClass() string {
	return "flex space-x-4"
}

func (h *TailwindHelpers) NavbarItemClass() string {
	return "text-gray-700 hover:text-gray-900"
}

func (h *TailwindHelpers) NavbarStartClass() string {
	return "flex items-center"
}

func (h *TailwindHelpers) NavbarEndClass() string {
	return "flex items-center ml-auto"
}

// Text/Typography helpers
func (h *TailwindHelpers) TitleClass(level int) string {
	switch level {
	case 1:
		return "text-3xl font-bold text-gray-900 mb-6"
	case 2:
		return "text-2xl font-bold text-gray-900 mb-4"
	case 3:
		return "text-xl font-bold text-gray-900 mb-3"
	default:
		return "text-3xl font-bold text-gray-900 mb-6"
	}
}

func (h *TailwindHelpers) SubtitleClass() string {
	return "text-xl font-semibold text-gray-700 mb-4"
}

func (h *TailwindHelpers) TextClass(size string) string {
	switch size {
	case "small":
		return "text-sm text-gray-700"
	case "large":
		return "text-lg text-gray-700"
	default:
		return "text-gray-700"
	}
}

func (h *TailwindHelpers) TextMutedClass() string {
	return "text-gray-500"
}

func (h *TailwindHelpers) TextPrimaryClass() string {
	return "text-blue-600"
}

func (h *TailwindHelpers) TextDangerClass() string {
	return "text-red-600"
}

func (h *TailwindHelpers) TextSuccessClass() string {
	return "text-green-600"
}

func (h *TailwindHelpers) TextWarningClass() string {
	return "text-yellow-600"
}

// Pagination helpers
func (h *TailwindHelpers) PaginationClass() string {
	return "flex justify-between items-center mt-4"
}

func (h *TailwindHelpers) PaginationButtonClass(state string) string {
	if state == "active" {
		return "bg-blue-600 text-white px-3 py-1 rounded"
	}
	return "px-4 py-2 border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
}

func (h *TailwindHelpers) PaginationListClass() string {
	return "flex items-center space-x-2"
}

func (h *TailwindHelpers) PaginationItemClass() string {
	return ""
}

// Card/Panel helpers
func (h *TailwindHelpers) CardClass() string {
	return "bg-white shadow rounded-lg overflow-hidden"
}

func (h *TailwindHelpers) CardHeaderClass() string {
	return "px-6 py-4 bg-gray-50 border-b"
}

func (h *TailwindHelpers) CardBodyClass() string {
	return "p-6"
}

func (h *TailwindHelpers) CardFooterClass() string {
	return "px-6 py-4 bg-gray-50 border-t"
}

// Modal/Dialog helpers
func (h *TailwindHelpers) ModalClass() string {
	return "fixed inset-0 z-50 overflow-y-auto"
}

func (h *TailwindHelpers) ModalBackgroundClass() string {
	return "fixed inset-0 bg-black opacity-50"
}

func (h *TailwindHelpers) ModalContentClass() string {
	return "relative bg-white rounded-lg shadow-xl max-w-lg mx-auto my-8 p-6"
}

func (h *TailwindHelpers) ModalCloseClass() string {
	return "absolute top-4 right-4 text-gray-400 hover:text-gray-600"
}

// Alert/Notification helpers
func (h *TailwindHelpers) AlertClass(variant string) string {
	switch variant {
	case "success":
		return "bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded"
	case "danger":
		return "bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded"
	case "warning":
		return "bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded"
	case "info":
		return "bg-blue-100 border border-blue-400 text-blue-700 px-4 py-3 rounded"
	default:
		return "bg-gray-100 border border-gray-400 text-gray-700 px-4 py-3 rounded"
	}
}

func (h *TailwindHelpers) NotificationClass(variant string) string {
	return h.AlertClass(variant)
}

// Badge/Tag helpers
func (h *TailwindHelpers) BadgeClass(variant string) string {
	switch variant {
	case "primary":
		return "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
	case "success":
		return "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
	case "danger":
		return "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800"
	default:
		return "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
	}
}

func (h *TailwindHelpers) TagClass(variant string) string {
	return h.BadgeClass(variant)
}

// Loading/Spinner helpers
func (h *TailwindHelpers) SpinnerClass() string {
	return "animate-spin h-5 w-5 border-2 border-blue-600 border-t-transparent rounded-full"
}

func (h *TailwindHelpers) LoadingClass() string {
	return "text-gray-600 animate-pulse"
}

// Grid helpers
func (h *TailwindHelpers) GridClass() string {
	return "grid gap-4"
}

func (h *TailwindHelpers) GridItemClass() string {
	return ""
}

// Flex helpers
func (h *TailwindHelpers) FlexClass() string {
	return "flex"
}

func (h *TailwindHelpers) FlexItemClass() string {
	return ""
}

// Spacing helpers
func (h *TailwindHelpers) MarginClass(size string) string {
	switch size {
	case "small":
		return "m-2"
	case "medium":
		return "m-4"
	case "large":
		return "m-8"
	default:
		return "m-4"
	}
}

func (h *TailwindHelpers) PaddingClass(size string) string {
	switch size {
	case "small":
		return "p-2"
	case "medium":
		return "p-4"
	case "large":
		return "p-8"
	default:
		return "p-4"
	}
}

// Display helpers
func (h *TailwindHelpers) HiddenClass() string {
	return "hidden"
}

func (h *TailwindHelpers) VisibleClass() string {
	return "block"
}

// Framework-specific checks
func (h *TailwindHelpers) NeedsWrapper() bool {
	return false
}

func (h *TailwindHelpers) NeedsArticle() bool {
	return false
}
