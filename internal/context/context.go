// Package context provides template execution context utilities for the LiveTemplate library.
// It handles the "lvt" namespace in templates, providing access to validation errors,
// flash messages, and development mode flags.
package context

import (
	"bytes"
	"html"
	"html/template"
	"reflect"
	"strings"
	"sync"
)

// FlashPrefix is the prefix used to identify flash messages in the unified messages map.
// Flash messages are stored as "_flash:key" -> "message" in the map.
// This allows a single map to contain both field errors and flash messages.
const FlashPrefix = "_flash:"

var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// maxPooledBufferBytes caps the capacity of a bytes.Buffer we are willing to
// return to bufferPool. A single very large template render grows a buffer to
// that size; without a cap, sync.Pool would retain it (for up to two GC cycles)
// and bloat idle memory for apps that mix tiny and huge templates.
const maxPooledBufferBytes = 256 << 10 // 256 KB

// bufferReusable reports whether buf is small enough to be worth returning to
// bufferPool. Buffers grown past maxPooledBufferBytes by a large render are
// dropped so sync.Pool cannot pin their capacity.
func bufferReusable(buf *bytes.Buffer) bool {
	return buf.Cap() <= maxPooledBufferBytes
}

// putBuffer returns buf to bufferPool unless it has grown too large to be worth
// retaining, in which case it is dropped for the GC to reclaim.
func putBuffer(buf *bytes.Buffer) {
	if bufferReusable(buf) {
		bufferPool.Put(buf)
	}
}

// TemplateContext provides utility functions for templates via the lvt namespace.
//
// It provides two separate message systems via a unified messages map:
//   - Errors: Field validation errors from action dispatch (affects ResponseMetadata.Success)
//   - Flash: Page-level messages (prefixed with "_flash:") that don't affect Success
//
// Thread-safety: TemplateContext is safe for concurrent reads but not for concurrent writes.
// If you need to share a TemplateContext across goroutines that modify it, external
// synchronization is required. In typical usage, each template execution creates a new
// TemplateContext, so concurrent access is not an issue.
type TemplateContext struct {
	messages      map[string]string // Unified map: field errors + flash (prefixed with "_flash:")
	DevMode       bool              // Development mode - exposed to templates as {{.lvt.DevMode}}
	Pending       bool              // True when the current action registered async work via Async()
	uploadEntries interface{}       // *upload.Registry for accessing upload state
}

// NewTemplateContext creates a new TemplateContext with the given messages map and devMode flag.
//
// The messages map contains both field errors and flash messages. Flash messages are
// identified by the "_flash:" prefix (e.g., "_flash:success" -> "Changes saved!").
//
// The messages map is stored by reference, not copied. Callers should not modify
// the map after passing it to NewTemplateContext.
func NewTemplateContext(messages map[string]string, devMode bool) *TemplateContext {
	return &TemplateContext{
		messages: messages,
		DevMode:  devMode,
	}
}

// SetUploadRegistry sets the upload registry for this template context.
// This allows templates to access upload state via .lvt.Uploads(name).
func (t *TemplateContext) SetUploadRegistry(registry interface{}) {
	t.uploadEntries = registry
}

// Error returns the error message for a field.
// Returns empty string if the field has no error, if messages map is nil,
// or if the field is a flash message key (use Flash() for those).
func (t *TemplateContext) Error(field string) string {
	if t.messages == nil {
		return ""
	}
	// Don't return flash messages via Error()
	if strings.HasPrefix(field, FlashPrefix) {
		return ""
	}
	return t.messages[field]
}

// ErrorTag returns the error message wrapped in a <small> element as template.HTML.
// Returns empty template.HTML if the field has no error.
// The error message text is HTML-escaped to prevent XSS.
func (t *TemplateContext) ErrorTag(field string) template.HTML {
	msg := t.Error(field)
	if msg == "" {
		return ""
	}
	return template.HTML("<small>" + html.EscapeString(msg) + "</small>")
}

// AriaInvalid returns aria-invalid="true" as template.HTMLAttr if the field has an error.
// Returns empty template.HTMLAttr if no error exists.
func (t *TemplateContext) AriaInvalid(field string) template.HTMLAttr {
	if t.HasError(field) {
		return template.HTMLAttr(`aria-invalid="true"`)
	}
	return ""
}

// AriaDisabled returns aria-disabled="true" if any of the given fields have an error.
// Returns empty template.HTMLAttr if no errors exist for any listed field.
//
// Use this on related UI elements (e.g. submit buttons) that should appear disabled
// while validation errors are present — not on the errored field itself. For marking
// errored fields, use AriaInvalid instead.
//
// Note: aria-disabled signals a disabled state to assistive technology but does not
// prevent interaction. Pair with the HTML disabled attribute or JavaScript to actually
// block form submission.
func (t *TemplateContext) AriaDisabled(fields ...string) template.HTMLAttr {
	for _, field := range fields {
		if t.HasError(field) {
			return template.HTMLAttr(`aria-disabled="true"`)
		}
	}
	return ""
}

// HasError checks if a field has an error.
// Returns false if messages map is nil or if the field is a flash message key.
func (t *TemplateContext) HasError(field string) bool {
	if t.messages == nil {
		return false
	}
	// Flash messages are not errors
	if strings.HasPrefix(field, FlashPrefix) {
		return false
	}
	_, exists := t.messages[field]
	return exists
}

// HasAnyError checks if any field errors exist (excludes flash messages).
// Returns false if messages map is nil or contains only flash messages.
func (t *TemplateContext) HasAnyError() bool {
	if t.messages == nil {
		return false
	}
	for k := range t.messages {
		if !strings.HasPrefix(k, FlashPrefix) {
			return true
		}
	}
	return false
}

// AllErrors returns a copy of all field errors (excludes flash messages).
// The returned map is a defensive copy and mutations will not affect internal state.
func (t *TemplateContext) AllErrors() map[string]string {
	result := make(map[string]string)
	if t.messages == nil {
		return result
	}
	for k, v := range t.messages {
		if !strings.HasPrefix(k, FlashPrefix) {
			result[k] = v
		}
	}
	return result
}

// Flash returns the flash message for a key.
// Returns empty string if the key has no flash message or if messages map is nil.
// Common keys: "success", "error", "info", "warning"
func (t *TemplateContext) Flash(key string) string {
	if t.messages == nil {
		return ""
	}
	return t.messages[FlashPrefix+key]
}

// HasFlash checks if a key has a flash message.
// Returns false if messages map is nil.
func (t *TemplateContext) HasFlash(key string) bool {
	if t.messages == nil {
		return false
	}
	_, exists := t.messages[FlashPrefix+key]
	return exists
}

// HasAnyFlash checks if any flash messages exist.
// Returns false if messages map is nil or contains no flash messages.
func (t *TemplateContext) HasAnyFlash() bool {
	if t.messages == nil {
		return false
	}
	for k := range t.messages {
		if strings.HasPrefix(k, FlashPrefix) {
			return true
		}
	}
	return false
}

// AllFlash returns a copy of all flash messages (with prefix stripped).
// The returned map is a defensive copy and mutations will not affect internal state.
func (t *TemplateContext) AllFlash() map[string]string {
	result := make(map[string]string)
	if t.messages == nil {
		return result
	}
	for k, v := range t.messages {
		if strings.HasPrefix(k, FlashPrefix) {
			result[strings.TrimPrefix(k, FlashPrefix)] = v
		}
	}
	return result
}

// FlashTag returns the flash message wrapped in an <output> element as template.HTML.
// Returns empty template.HTML if the key has no flash message.
// Uses role="alert" for "error" key and role="status" for all other keys.
// The flash message text is HTML-escaped to prevent XSS.
func (t *TemplateContext) FlashTag(key string) template.HTML {
	msg := t.Flash(key)
	if msg == "" {
		return ""
	}
	role := "status"
	if key == "error" {
		role = "alert"
	}
	return template.HTML(`<output role="` + role + `" data-flash="` +
		html.EscapeString(key) + `">` + html.EscapeString(msg) + "</output>")
}

// Uploads returns upload entries for a given upload name.
// Returns nil if no upload registry is set or upload doesn't exist.
// The upload registry is expected to have a method: GetUpload(name string) interface{}
// which returns an object with GetEntries() []interface{} method.
func (t *TemplateContext) Uploads(name string) interface{} {
	if t.uploadEntries == nil {
		return nil
	}

	// Use reflection to call GetUpload method
	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return nil
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return nil
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 {
		return nil
	}

	upload := results[0].Interface()
	if upload == nil {
		return nil
	}

	// Call GetEntries on the upload object
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return nil
	}

	getEntriesMethod := uploadVal.MethodByName("GetEntries")
	if !getEntriesMethod.IsValid() {
		return nil
	}

	entriesResults := getEntriesMethod.Call(nil)
	if len(entriesResults) == 0 {
		return nil
	}

	return entriesResults[0].Interface()
}

// HasUploadError checks if any upload entry has an error for the given upload name.
func (t *TemplateContext) HasUploadError(name string) bool {
	if t.uploadEntries == nil {
		return false
	}

	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return false
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return false
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 || results[0].IsNil() {
		return false
	}

	upload := results[0].Interface()
	if upload == nil {
		return false
	}
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return false
	}

	hasErrorMethod := uploadVal.MethodByName("HasError")
	if !hasErrorMethod.IsValid() {
		return false
	}

	errorResults := hasErrorMethod.Call(nil)
	if len(errorResults) == 0 {
		return false
	}

	return errorResults[0].Bool()
}

// UploadError returns the first error message for the given upload name.
// Returns empty string if no errors exist.
func (t *TemplateContext) UploadError(name string) string {
	if t.uploadEntries == nil {
		return ""
	}

	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return ""
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return ""
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 || results[0].IsNil() {
		return ""
	}

	upload := results[0].Interface()
	if upload == nil {
		return ""
	}
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return ""
	}

	getErrorMethod := uploadVal.MethodByName("GetError")
	if !getErrorMethod.IsValid() {
		return ""
	}

	errorResults := getErrorMethod.Call(nil)
	if len(errorResults) == 0 {
		return ""
	}

	return errorResults[0].String()
}

// Redact emits a self-contained <span data-lvt-redact="name"> the client fills
// from localStorage; the trust signal is the attribute, so no free-floating
// token can be spoofed by user-posted content (editable inputs carry the same
// attribute directly). Empty until the client hydrates it.
func (t *TemplateContext) Redact(name string) template.HTML {
	return template.HTML(`<span data-lvt-redact="` + html.EscapeString(name) + `"></span>`)
}

// UploadPreview emits an <img data-lvt-upload-preview="name"> placeholder for a
// Preview-mode upload field. The client fills its src from a local object URL
// (URL.createObjectURL) when a file is selected, so the image previews
// on-device without the bytes ever being uploaded. Empty until the client
// attaches the blob.
func (t *TemplateContext) UploadPreview(name string) template.HTML {
	return template.HTML(`<img data-lvt-upload-preview="` + html.EscapeString(name) + `" alt="" />`)
}

const (
	// TemplateContextKey is the key used to access lvt context in templates
	TemplateContextKey = "lvt"
)

// ExecuteTemplateWithContext adds lvt context to template execution by augmenting the data.
//
// This function handles different types of input data:
//   - Structs: Fields are copied to a map, using json tags if present
//   - Maps: Keys are copied to template data
//   - Nil pointers: Only lvt context is provided
//   - Primitives: Passed directly to the template as-is (lvt not available)
//
// The lvt context is available in templates via {{.lvt}} for structs, maps, and nil pointers.
// For primitive types (string, int, bool, etc.), lvt context is not available to maintain
// compatibility with standard Go templates.
//
// The messages map contains both field errors and flash messages. Flash messages are
// identified by the "_flash:" prefix (e.g., "_flash:success" -> "Changes saved!").
// Field errors affect ResponseMetadata.Success; flash messages don't.
//
// Note: If struct fields or map keys conflict with the reserved "lvt" key, they will be
// skipped to ensure the lvt context remains accessible in templates.
func ExecuteTemplateWithContext(tmpl *template.Template, data interface{}, messages map[string]string, devMode bool, uploadRegistry interface{}, precomputeAllow map[string]struct{}) ([]byte, error) {
	dataMap := BuildDataMap(data, messages, devMode, uploadRegistry, precomputeAllow, false)
	return executeWithBuffer(tmpl, dataMap)
}

// ExecuteTemplateWithDataMap executes a template with a pre-built data map.
// Use this when the data map has already been constructed via BuildDataMap
// to avoid duplicate reflection.
func ExecuteTemplateWithDataMap(tmpl *template.Template, dataMap interface{}) ([]byte, error) {
	return executeWithBuffer(tmpl, dataMap)
}

func executeWithBuffer(tmpl *template.Template, data interface{}) ([]byte, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	err := tmpl.Execute(buf, data)
	if err != nil {
		putBuffer(buf)
		return nil, err
	}
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	putBuffer(buf)
	return result, nil
}
