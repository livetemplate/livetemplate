package testing

import (
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

// Assert provides assertion helpers for e2e tests.
type Assert struct {
	test *E2ETest
}

// NewAssert creates an assertion helper for the given test.
//
// Example:
//
//	assert := lvttest.NewAssert(test)
//	assert.PageContains("Welcome")
//	assert.WebSocketConnected()
func NewAssert(test *E2ETest) *Assert {
	return &Assert{test: test}
}

// PageContains verifies that the page body contains the given text.
func (a *Assert) PageContains(text string) error {
	a.test.T.Helper()

	var html string
	err := chromedp.Run(a.test.Context,
		chromedp.OuterHTML("body", &html, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("failed to get page HTML: %w", err)
	}

	if !strings.Contains(html, text) {
		return fmt.Errorf("page does not contain %q", text)
	}

	return nil
}

// PageNotContains verifies that the page body does NOT contain the given text.
func (a *Assert) PageNotContains(text string) error {
	a.test.T.Helper()

	var html string
	err := chromedp.Run(a.test.Context,
		chromedp.OuterHTML("body", &html, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("failed to get page HTML: %w", err)
	}

	if strings.Contains(html, text) {
		return fmt.Errorf("page should not contain %q but it does", text)
	}

	return nil
}

// WebSocketConnected verifies that the WebSocket connection is established.
// This checks if the data-lvt-loading attribute has been removed from the wrapper.
func (a *Assert) WebSocketConnected() error {
	a.test.T.Helper()

	var connected bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(`
			(() => {
				const wrapper = document.querySelector('[data-lvt-id]');
				return wrapper && !wrapper.hasAttribute('data-lvt-loading');
			})()
		`, &connected),
	)

	if err != nil {
		return fmt.Errorf("failed to check WebSocket connection: %w", err)
	}

	if !connected {
		return fmt.Errorf("WebSocket not connected (data-lvt-loading attribute still present)")
	}

	return nil
}

// NoTemplateErrors verifies that the page does not contain raw Go template expressions.
// This catches bugs where unflattened templates are sent to the client.
func (a *Assert) NoTemplateErrors() error {
	a.test.T.Helper()

	err := chromedp.Run(a.test.Context,
		ValidateNoTemplateExpressions("[data-lvt-id]"),
	)

	if err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	return nil
}

// ElementVisible verifies that an element matching the selector is visible.
func (a *Assert) ElementVisible(selector string) error {
	a.test.T.Helper()

	err := chromedp.Run(a.test.Context,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("element %q is not visible: %w", selector, err)
	}

	return nil
}

// ElementHidden verifies that an element matching the selector is hidden or doesn't exist.
func (a *Assert) ElementHidden(selector string) error {
	a.test.T.Helper()

	var exists bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				return el !== null && el.offsetParent !== null;
			})()
		`, selector), &exists),
	)

	if err != nil {
		return fmt.Errorf("failed to check element visibility: %w", err)
	}

	if exists {
		return fmt.Errorf("element %q should be hidden but is visible", selector)
	}

	return nil
}

// FormFieldValue verifies that a form field has the expected value.
func (a *Assert) FormFieldValue(selector, expectedValue string) error {
	a.test.T.Helper()

	var actualValue string
	err := chromedp.Run(a.test.Context,
		chromedp.Value(selector, &actualValue, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get field value for %q: %w", selector, err)
	}

	if actualValue != expectedValue {
		return fmt.Errorf("field %q has value %q, expected %q", selector, actualValue, expectedValue)
	}

	return nil
}

// NoConsoleErrors verifies that there are no console errors.
// This is useful for catching JavaScript errors during tests.
func (a *Assert) NoConsoleErrors() error {
	a.test.T.Helper()

	if a.test.Console == nil {
		return fmt.Errorf("console logger not initialized")
	}

	if a.test.Console.HasErrors() {
		errors := a.test.Console.GetErrors()
		return fmt.Errorf("found %d console error(s): %v", len(errors), errors)
	}

	return nil
}

// ElementCount verifies that exactly expectedCount elements match the selector.
func (a *Assert) ElementCount(selector string, expectedCount int) error {
	a.test.T.Helper()

	var count int
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, selector), &count),
	)

	if err != nil {
		return fmt.Errorf("failed to count elements matching %q: %w", selector, err)
	}

	if count != expectedCount {
		return fmt.Errorf("found %d elements matching %q, expected %d", count, selector, expectedCount)
	}

	return nil
}

// AttributeValue verifies that an element has the expected attribute value.
func (a *Assert) AttributeValue(selector, attribute, expectedValue string) error {
	a.test.T.Helper()

	var actualValue string
	err := chromedp.Run(a.test.Context,
		chromedp.AttributeValue(selector, attribute, &actualValue, nil, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get attribute %q for %q: %w", attribute, selector, err)
	}

	if actualValue != expectedValue {
		return fmt.Errorf("element %q has attribute %q=%q, expected %q", selector, attribute, actualValue, expectedValue)
	}

	return nil
}

// TableRowCount verifies that a table has the expected number of rows.
// This counts tbody tr elements by default.
func (a *Assert) TableRowCount(expectedCount int) error {
	a.test.T.Helper()

	return a.ElementCount("tbody tr", expectedCount)
}

// TextContent verifies that an element has the expected text content.
func (a *Assert) TextContent(selector, expectedText string) error {
	a.test.T.Helper()

	var actualText string
	err := chromedp.Run(a.test.Context,
		chromedp.Text(selector, &actualText, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get text content for %q: %w", selector, err)
	}

	actualText = strings.TrimSpace(actualText)
	expectedText = strings.TrimSpace(expectedText)

	if actualText != expectedText {
		return fmt.Errorf("element %q has text %q, expected %q", selector, actualText, expectedText)
	}

	return nil
}

// TextContains verifies that an element's text contains the expected substring.
func (a *Assert) TextContains(selector, expectedSubstring string) error {
	a.test.T.Helper()

	var actualText string
	err := chromedp.Run(a.test.Context,
		chromedp.Text(selector, &actualText, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to get text content for %q: %w", selector, err)
	}

	if !strings.Contains(actualText, expectedSubstring) {
		return fmt.Errorf("element %q text %q does not contain %q", selector, actualText, expectedSubstring)
	}

	return nil
}

// ElementExists verifies that at least one element matches the selector.
func (a *Assert) ElementExists(selector string) error {
	a.test.T.Helper()

	var exists bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, selector), &exists),
	)

	if err != nil {
		return fmt.Errorf("failed to check if element exists: %w", err)
	}

	if !exists {
		return fmt.Errorf("element %q does not exist", selector)
	}

	return nil
}

// ElementNotExists verifies that no elements match the selector.
func (a *Assert) ElementNotExists(selector string) error {
	a.test.T.Helper()

	var exists bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, selector), &exists),
	)

	if err != nil {
		return fmt.Errorf("failed to check if element exists: %w", err)
	}

	if exists {
		return fmt.Errorf("element %q should not exist but it does", selector)
	}

	return nil
}

// HasClass verifies that an element has the expected CSS class.
func (a *Assert) HasClass(selector, className string) error {
	a.test.T.Helper()

	var hasClass bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s').classList.contains('%s')`, selector, className), &hasClass),
	)

	if err != nil {
		return fmt.Errorf("failed to check class for %q: %w", selector, err)
	}

	if !hasClass {
		return fmt.Errorf("element %q does not have class %q", selector, className)
	}

	return nil
}

// NotHasClass verifies that an element does NOT have the expected CSS class.
func (a *Assert) NotHasClass(selector, className string) error {
	a.test.T.Helper()

	var hasClass bool
	err := chromedp.Run(a.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s').classList.contains('%s')`, selector, className), &hasClass),
	)

	if err != nil {
		return fmt.Errorf("failed to check class for %q: %w", selector, err)
	}

	if hasClass {
		return fmt.Errorf("element %q should not have class %q but it does", selector, className)
	}

	return nil
}
