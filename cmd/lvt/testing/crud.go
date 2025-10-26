package testing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// CRUDTester provides helpers for testing CRUD operations on resources.
type CRUDTester struct {
	test        *E2ETest
	resourceURL string
}

// NewCRUDTester creates a CRUD tester for the given resource path.
//
// Example:
//
//	crud := lvttest.NewCRUDTester(test, "/products")
//	crud.Create(
//	    lvttest.TextField("name", "MacBook Pro"),
//	    lvttest.FloatField("price", 2499.99),
//	)
func NewCRUDTester(test *E2ETest, resourcePath string) *CRUDTester {
	return &CRUDTester{
		test:        test,
		resourceURL: resourcePath,
	}
}

// Field represents a form field to be filled during testing.
type Field interface {
	Name() string
	Selector() string
	Fill(ctx context.Context) error
}

// textField implements Field for text input fields.
type textField struct {
	name  string
	value string
}

func (f *textField) Name() string {
	return f.name
}

func (f *textField) Selector() string {
	return fmt.Sprintf(`input[name="%s"]`, f.name)
}

func (f *textField) Fill(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.SendKeys(f.Selector(), f.value, chromedp.ByQuery),
	)
}

// TextField creates a field for text input.
func TextField(name, value string) Field {
	return &textField{name: name, value: value}
}

// textAreaField implements Field for textarea fields.
type textAreaField struct {
	name  string
	value string
}

func (f *textAreaField) Name() string {
	return f.name
}

func (f *textAreaField) Selector() string {
	return fmt.Sprintf(`textarea[name="%s"]`, f.name)
}

func (f *textAreaField) Fill(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.SendKeys(f.Selector(), f.value, chromedp.ByQuery),
	)
}

// TextAreaField creates a field for textarea input.
func TextAreaField(name, value string) Field {
	return &textAreaField{name: name, value: value}
}

// intField implements Field for integer input fields.
type intField struct {
	name  string
	value int64
}

func (f *intField) Name() string {
	return f.name
}

func (f *intField) Selector() string {
	return fmt.Sprintf(`input[name="%s"]`, f.name)
}

func (f *intField) Fill(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.SendKeys(f.Selector(), fmt.Sprintf("%d", f.value), chromedp.ByQuery),
	)
}

// IntField creates a field for integer input.
func IntField(name string, value int64) Field {
	return &intField{name: name, value: value}
}

// floatField implements Field for float input fields.
type floatField struct {
	name  string
	value float64
}

func (f *floatField) Name() string {
	return f.name
}

func (f *floatField) Selector() string {
	return fmt.Sprintf(`input[name="%s"]`, f.name)
}

func (f *floatField) Fill(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.SendKeys(f.Selector(), fmt.Sprintf("%.2f", f.value), chromedp.ByQuery),
	)
}

// FloatField creates a field for float input.
func FloatField(name string, value float64) Field {
	return &floatField{name: name, value: value}
}

// boolField implements Field for checkbox fields.
type boolField struct {
	name  string
	value bool
}

func (f *boolField) Name() string {
	return f.name
}

func (f *boolField) Selector() string {
	return fmt.Sprintf(`input[name="%s"]`, f.name)
}

func (f *boolField) Fill(ctx context.Context) error {
	// Check current state
	var checked bool
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s').checked`, f.Selector()), &checked),
	); err != nil {
		return err
	}

	// Only click if we need to change the state
	if checked != f.value {
		return chromedp.Run(ctx,
			chromedp.Click(f.Selector(), chromedp.ByQuery),
		)
	}

	return nil
}

// BoolField creates a field for checkbox input.
func BoolField(name string, value bool) Field {
	return &boolField{name: name, value: value}
}

// selectField implements Field for select dropdown fields.
type selectField struct {
	name  string
	value string
}

func (f *selectField) Name() string {
	return f.name
}

func (f *selectField) Selector() string {
	return fmt.Sprintf(`select[name="%s"]`, f.name)
}

func (f *selectField) Fill(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(f.Selector(), chromedp.ByQuery),
		chromedp.SetValue(f.Selector(), f.value, chromedp.ByQuery),
	)
}

// SelectField creates a field for select dropdown.
func SelectField(name, value string) Field {
	return &selectField{name: name, value: value}
}

// Create fills the create form with the given fields and submits it.
// It waits for the WebSocket update to complete after submission.
func (c *CRUDTester) Create(fields ...Field) error {
	c.test.T.Helper()

	// Fill all fields
	for _, field := range fields {
		if err := field.Fill(c.test.Context); err != nil {
			return fmt.Errorf("failed to fill field %q: %w", field.Name(), err)
		}
	}

	// Submit the form
	err := chromedp.Run(c.test.Context,
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket update
	)

	if err != nil {
		return fmt.Errorf("failed to submit form: %w", err)
	}

	return nil
}

// VerifyExists checks if a record containing the given text appears in the page.
func (c *CRUDTester) VerifyExists(searchText string) error {
	c.test.T.Helper()

	var html string
	err := chromedp.Run(c.test.Context,
		chromedp.OuterHTML("body", &html, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get page HTML: %w", err)
	}

	if !strings.Contains(html, searchText) {
		return fmt.Errorf("record with text %q not found in page", searchText)
	}

	return nil
}

// VerifyNotExists checks if a record containing the given text does NOT appear in the page.
func (c *CRUDTester) VerifyNotExists(searchText string) error {
	c.test.T.Helper()

	var html string
	err := chromedp.Run(c.test.Context,
		chromedp.OuterHTML("body", &html, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get page HTML: %w", err)
	}

	if strings.Contains(html, searchText) {
		return fmt.Errorf("record with text %q should not exist but was found", searchText)
	}

	return nil
}

// Edit opens the edit form for a record, fills the fields, and saves.
// The recordID is used to identify which record to edit (e.g., using lvt-data-id).
func (c *CRUDTester) Edit(recordID string, fields ...Field) error {
	c.test.T.Helper()

	// Click edit button for the record
	editSelector := fmt.Sprintf(`button[lvt-click="edit"][lvt-data-id="%s"]`, recordID)
	err := chromedp.Run(c.test.Context,
		chromedp.WaitVisible(editSelector, chromedp.ByQuery),
		chromedp.Click(editSelector, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for modal/form to appear
	)

	if err != nil {
		return fmt.Errorf("failed to click edit button for record %q: %w", recordID, err)
	}

	// Fill all fields
	for _, field := range fields {
		if err := field.Fill(c.test.Context); err != nil {
			return fmt.Errorf("failed to fill edit field %q: %w", field.Name(), err)
		}
	}

	// Submit the edit form
	err = chromedp.Run(c.test.Context,
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket update
	)

	if err != nil {
		return fmt.Errorf("failed to submit edit form: %w", err)
	}

	return nil
}

// Delete clicks the delete button for a record and confirms.
// The recordID is used to identify which record to delete (e.g., using lvt-data-id).
func (c *CRUDTester) Delete(recordID string) error {
	c.test.T.Helper()

	// Click delete button for the record
	deleteSelector := fmt.Sprintf(`button[lvt-click="delete"][lvt-data-id="%s"]`, recordID)
	err := chromedp.Run(c.test.Context,
		chromedp.WaitVisible(deleteSelector, chromedp.ByQuery),
		chromedp.Click(deleteSelector, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket update
	)

	if err != nil {
		return fmt.Errorf("failed to click delete button for record %q: %w", recordID, err)
	}

	return nil
}

// GetTableRows extracts table row data from the page.
// This is useful for verifying record order, content, etc.
func (c *CRUDTester) GetTableRows() ([]map[string]string, error) {
	c.test.T.Helper()

	var rows []map[string]string

	// This is a simplified implementation - in real usage, you'd want to
	// customize this based on your table structure
	var html string
	err := chromedp.Run(c.test.Context,
		chromedp.OuterHTML("tbody", &html, chromedp.ByQuery),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get table HTML: %w", err)
	}

	// For now, just return the raw HTML in a single row
	// A more sophisticated implementation would parse the actual table structure
	rows = append(rows, map[string]string{
		"html": html,
	})

	return rows, nil
}
