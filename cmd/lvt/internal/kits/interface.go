package kits

// Kit represents a design system that provides CSS/styling for components
type Kit interface {
	// Name returns the kit name
	Name() string

	// Version returns the kit version
	Version() string

	// GetHelpers returns the CSS helper interface for this kit
	GetHelpers() CSSHelpers
}

// CSSHelpers provides CSS framework-specific helper functions for templates
// These methods abstract away framework-specific class names and structures
type CSSHelpers interface {
	// Framework information
	CSSCDN() string

	// Layout helpers
	ContainerClass() string
	SectionClass() string
	BoxClass() string
	ColumnClass() string
	ColumnsClass() string

	// Form helpers
	FieldClass() string
	LabelClass() string
	InputClass() string
	TextareaClass() string
	SelectClass() string
	CheckboxClass() string
	RadioClass() string
	ButtonClass(variant string) string
	ButtonGroupClass() string
	FormClass() string

	// Table helpers
	TableClass() string
	TheadClass() string
	TbodyClass() string
	ThClass() string
	TdClass() string
	TrClass() string
	TableContainerClass() string

	// Navigation helpers
	NavbarClass() string
	NavbarBrandClass() string
	NavbarMenuClass() string
	NavbarItemClass() string
	NavbarStartClass() string
	NavbarEndClass() string

	// Text/Typography helpers
	TitleClass(level int) string
	SubtitleClass() string
	TextClass(size string) string
	TextMutedClass() string
	TextPrimaryClass() string
	TextDangerClass() string
	TextSuccessClass() string
	TextWarningClass() string

	// Pagination helpers
	PaginationClass() string
	PaginationButtonClass(state string) string
	PaginationListClass() string
	PaginationItemClass() string

	// Card/Panel helpers
	CardClass() string
	CardHeaderClass() string
	CardBodyClass() string
	CardFooterClass() string

	// Modal/Dialog helpers
	ModalClass() string
	ModalBackgroundClass() string
	ModalContentClass() string
	ModalCloseClass() string

	// Alert/Notification helpers
	AlertClass(variant string) string
	NotificationClass(variant string) string

	// Badge/Tag helpers
	BadgeClass(variant string) string
	TagClass(variant string) string

	// Loading/Spinner helpers
	SpinnerClass() string
	LoadingClass() string

	// Grid helpers
	GridClass() string
	GridItemClass() string

	// Flex helpers
	FlexClass() string
	FlexItemClass() string

	// Spacing helpers
	MarginClass(size string) string
	PaddingClass(size string) string

	// Display helpers
	HiddenClass() string
	VisibleClass() string

	// Framework-specific checks
	NeedsWrapper() bool
	NeedsArticle() bool

	// Utility functions
	Dict(values ...interface{}) map[string]interface{}
	Until(count int) []int
	Add(a, b int) int
}
