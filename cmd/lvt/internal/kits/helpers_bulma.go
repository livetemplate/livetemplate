package kits

// BulmaHelpers implements CSSHelpers for Bulma CSS
type BulmaHelpers struct {
	BaseHelpers
}

// NewBulmaHelpers creates a new Bulma CSS helper
func NewBulmaHelpers() CSSHelpers {
	return &BulmaHelpers{}
}

// Framework information
func (h *BulmaHelpers) CSSCDN() string {
	return `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">`
}

// Layout helpers
func (h *BulmaHelpers) ContainerClass() string {
	return "container"
}

func (h *BulmaHelpers) SectionClass() string {
	return "section"
}

func (h *BulmaHelpers) BoxClass() string {
	return "box"
}

func (h *BulmaHelpers) ColumnClass() string {
	return "column"
}

func (h *BulmaHelpers) ColumnsClass() string {
	return "columns"
}

// Form helpers
func (h *BulmaHelpers) FieldClass() string {
	return "field"
}

func (h *BulmaHelpers) LabelClass() string {
	return "label"
}

func (h *BulmaHelpers) InputClass() string {
	return "input"
}

func (h *BulmaHelpers) TextareaClass() string {
	return "textarea"
}

func (h *BulmaHelpers) SelectClass() string {
	return "" // Bulma uses wrapper div.select
}

func (h *BulmaHelpers) CheckboxClass() string {
	return "checkbox"
}

func (h *BulmaHelpers) RadioClass() string {
	return "radio"
}

func (h *BulmaHelpers) ButtonClass(variant string) string {
	switch variant {
	case "primary":
		return "button is-primary"
	case "secondary":
		return "button is-small"
	case "danger":
		return "button is-danger"
	default:
		return "button is-primary"
	}
}

func (h *BulmaHelpers) ButtonGroupClass() string {
	return "buttons"
}

func (h *BulmaHelpers) FormClass() string {
	return ""
}

// Table helpers
func (h *BulmaHelpers) TableClass() string {
	return "table is-fullwidth is-striped"
}

func (h *BulmaHelpers) TheadClass() string {
	return ""
}

func (h *BulmaHelpers) TbodyClass() string {
	return ""
}

func (h *BulmaHelpers) ThClass() string {
	return ""
}

func (h *BulmaHelpers) TdClass() string {
	return ""
}

func (h *BulmaHelpers) TrClass() string {
	return ""
}

func (h *BulmaHelpers) TableContainerClass() string {
	return "table-container"
}

// Navigation helpers
func (h *BulmaHelpers) NavbarClass() string {
	return "navbar"
}

func (h *BulmaHelpers) NavbarBrandClass() string {
	return "navbar-brand"
}

func (h *BulmaHelpers) NavbarMenuClass() string {
	return "navbar-menu"
}

func (h *BulmaHelpers) NavbarItemClass() string {
	return "navbar-item"
}

func (h *BulmaHelpers) NavbarStartClass() string {
	return "navbar-start"
}

func (h *BulmaHelpers) NavbarEndClass() string {
	return "navbar-end"
}

// Text/Typography helpers
func (h *BulmaHelpers) TitleClass(level int) string {
	return "title"
}

func (h *BulmaHelpers) SubtitleClass() string {
	return "subtitle"
}

func (h *BulmaHelpers) TextClass(size string) string {
	return ""
}

func (h *BulmaHelpers) TextMutedClass() string {
	return "has-text-grey"
}

func (h *BulmaHelpers) TextPrimaryClass() string {
	return "has-text-primary"
}

func (h *BulmaHelpers) TextDangerClass() string {
	return "has-text-danger"
}

func (h *BulmaHelpers) TextSuccessClass() string {
	return "has-text-success"
}

func (h *BulmaHelpers) TextWarningClass() string {
	return "has-text-warning"
}

// Pagination helpers
func (h *BulmaHelpers) PaginationClass() string {
	return "pagination"
}

func (h *BulmaHelpers) PaginationButtonClass(state string) string {
	if state == "active" {
		return "pagination-link is-current"
	}
	return "button"
}

func (h *BulmaHelpers) PaginationListClass() string {
	return "pagination-list"
}

func (h *BulmaHelpers) PaginationItemClass() string {
	return ""
}

// Card/Panel helpers
func (h *BulmaHelpers) CardClass() string {
	return "card"
}

func (h *BulmaHelpers) CardHeaderClass() string {
	return "card-header"
}

func (h *BulmaHelpers) CardBodyClass() string {
	return "card-content"
}

func (h *BulmaHelpers) CardFooterClass() string {
	return "card-footer"
}

// Modal/Dialog helpers
func (h *BulmaHelpers) ModalClass() string {
	return "modal is-active"
}

func (h *BulmaHelpers) ModalBackgroundClass() string {
	return "modal-background"
}

func (h *BulmaHelpers) ModalContentClass() string {
	return "modal-content"
}

func (h *BulmaHelpers) ModalCloseClass() string {
	return "modal-close is-large"
}

// Alert/Notification helpers
func (h *BulmaHelpers) AlertClass(variant string) string {
	switch variant {
	case "success":
		return "notification is-success"
	case "danger":
		return "notification is-danger"
	case "warning":
		return "notification is-warning"
	case "info":
		return "notification is-info"
	default:
		return "notification"
	}
}

func (h *BulmaHelpers) NotificationClass(variant string) string {
	return h.AlertClass(variant)
}

// Badge/Tag helpers
func (h *BulmaHelpers) BadgeClass(variant string) string {
	switch variant {
	case "primary":
		return "tag is-primary"
	case "success":
		return "tag is-success"
	case "danger":
		return "tag is-danger"
	default:
		return "tag"
	}
}

func (h *BulmaHelpers) TagClass(variant string) string {
	return h.BadgeClass(variant)
}

// Loading/Spinner helpers
func (h *BulmaHelpers) SpinnerClass() string {
	return "loader"
}

func (h *BulmaHelpers) LoadingClass() string {
	return "has-text-grey"
}

// Grid helpers
func (h *BulmaHelpers) GridClass() string {
	return "columns is-multiline"
}

func (h *BulmaHelpers) GridItemClass() string {
	return "column"
}

// Flex helpers
func (h *BulmaHelpers) FlexClass() string {
	return "is-flex"
}

func (h *BulmaHelpers) FlexItemClass() string {
	return ""
}

// Spacing helpers
func (h *BulmaHelpers) MarginClass(size string) string {
	switch size {
	case "small":
		return "m-2"
	case "medium":
		return "m-4"
	case "large":
		return "m-6"
	default:
		return "m-4"
	}
}

func (h *BulmaHelpers) PaddingClass(size string) string {
	switch size {
	case "small":
		return "p-2"
	case "medium":
		return "p-4"
	case "large":
		return "p-6"
	default:
		return "p-4"
	}
}

// Display helpers
func (h *BulmaHelpers) HiddenClass() string {
	return "is-hidden"
}

func (h *BulmaHelpers) VisibleClass() string {
	return "is-block"
}

// Framework-specific checks
func (h *BulmaHelpers) NeedsWrapper() bool {
	return false
}

func (h *BulmaHelpers) NeedsArticle() bool {
	return false
}
