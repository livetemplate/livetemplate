package testing

import (
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// ModalTester provides methods for testing modal dialogs.
type ModalTester struct {
	test          *E2ETest
	modalSelector string
	openSelector  string
	closeSelector string
}

// NewModalTester creates a modal tester with default selectors.
// Default modal selector: "[data-test-id='modal']" or ".modal"
// You can customize selectors using WithSelectors().
func NewModalTester(test *E2ETest) *ModalTester {
	return &ModalTester{
		test:          test,
		modalSelector: "[data-test-id='modal']",
		openSelector:  "",
		closeSelector: "",
	}
}

// WithModalSelector sets a custom modal selector.
func (m *ModalTester) WithModalSelector(selector string) *ModalTester {
	m.modalSelector = selector
	return m
}

// WithOpenSelector sets the selector for the button/element that opens the modal.
func (m *ModalTester) WithOpenSelector(selector string) *ModalTester {
	m.openSelector = selector
	return m
}

// WithCloseSelector sets the selector for the button/element that closes the modal.
func (m *ModalTester) WithCloseSelector(selector string) *ModalTester {
	m.closeSelector = selector
	return m
}

// Open opens the modal by clicking the open selector.
// If no open selector is set, this will fail.
func (m *ModalTester) Open() error {
	m.test.T.Helper()

	if m.openSelector == "" {
		return fmt.Errorf("open selector not set, use WithOpenSelector()")
	}

	err := chromedp.Run(m.test.Context,
		chromedp.Click(m.openSelector, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to click open button %q: %w", m.openSelector, err)
	}

	return nil
}

// Close closes the modal by clicking the close selector.
// If no close selector is set, this will fail.
func (m *ModalTester) Close() error {
	m.test.T.Helper()

	if m.closeSelector == "" {
		return fmt.Errorf("close selector not set, use WithCloseSelector()")
	}

	err := chromedp.Run(m.test.Context,
		chromedp.Click(m.closeSelector, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to click close button %q: %w", m.closeSelector, err)
	}

	return nil
}

// OpenByAction opens the modal using a LiveTemplate action.
// This clicks the element with lvt-on-click attribute matching the action.
func (m *ModalTester) OpenByAction(action string) error {
	m.test.T.Helper()

	selector := fmt.Sprintf(`[lvt-on-click="%s"]`, action)
	err := chromedp.Run(m.test.Context,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to trigger action %q: %w", action, err)
	}

	return nil
}

// CloseByAction closes the modal using a LiveTemplate action.
func (m *ModalTester) CloseByAction(action string) error {
	m.test.T.Helper()

	selector := fmt.Sprintf(`[lvt-on-click="%s"]`, action)
	err := chromedp.Run(m.test.Context,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to trigger action %q: %w", action, err)
	}

	return nil
}

// VerifyVisible verifies that the modal is visible.
// This checks if the modal element exists and has a visible class or style.
func (m *ModalTester) VerifyVisible() error {
	m.test.T.Helper()

	var visible bool
	err := chromedp.Run(m.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const modal = document.querySelector('%s');
				if (!modal) return false;

				// Check if modal has 'visible', 'show', or 'open' class
				if (modal.classList.contains('visible') ||
				    modal.classList.contains('show') ||
				    modal.classList.contains('open')) {
					return true;
				}

				// Check if display is not 'none'
				const style = window.getComputedStyle(modal);
				return style.display !== 'none';
			})()
		`, m.modalSelector), &visible),
	)

	if err != nil {
		return fmt.Errorf("failed to check modal visibility: %w", err)
	}

	if !visible {
		return fmt.Errorf("modal %q is not visible", m.modalSelector)
	}

	return nil
}

// VerifyHidden verifies that the modal is hidden.
func (m *ModalTester) VerifyHidden() error {
	m.test.T.Helper()

	var visible bool
	err := chromedp.Run(m.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const modal = document.querySelector('%s');
				if (!modal) return false;

				// Check if modal has 'visible', 'show', or 'open' class
				if (modal.classList.contains('visible') ||
				    modal.classList.contains('show') ||
				    modal.classList.contains('open')) {
					return true;
				}

				// Check if display is not 'none'
				const style = window.getComputedStyle(modal);
				return style.display !== 'none';
			})()
		`, m.modalSelector), &visible),
	)

	if err != nil {
		return fmt.Errorf("failed to check modal visibility: %w", err)
	}

	if visible {
		return fmt.Errorf("modal %q should be hidden but is visible", m.modalSelector)
	}

	return nil
}

// FillForm fills a form inside the modal using Field definitions.
// This is useful for modals that contain forms for create/edit operations.
func (m *ModalTester) FillForm(fields ...Field) error {
	m.test.T.Helper()

	for _, field := range fields {
		if err := field.Fill(m.test.Context); err != nil {
			return fmt.Errorf("failed to fill field %q: %w", field.Name(), err)
		}
		chromedp.Sleep(100 * time.Millisecond)
	}

	return nil
}

// ClickButton clicks a button inside the modal.
// This searches for a button containing the specified text within the modal.
func (m *ModalTester) ClickButton(text string) error {
	m.test.T.Helper()

	err := chromedp.Run(m.test.Context,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const modal = document.querySelector('%s');
				if (!modal) throw new Error('Modal not found');

				const buttons = modal.querySelectorAll('button');
				for (const btn of buttons) {
					if (btn.textContent.includes('%s')) {
						btn.click();
						return true;
					}
				}
				throw new Error('Button with text "%s" not found in modal');
			})()
		`, m.modalSelector, text, text), nil),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		return fmt.Errorf("failed to click button %q in modal: %w", text, err)
	}

	return nil
}

// ClickSubmit clicks the submit button inside the modal and waits for WebSocket update.
// This is a convenience method for form submission.
func (m *ModalTester) ClickSubmit() error {
	m.test.T.Helper()

	selector := fmt.Sprintf(`%s button[type="submit"]`, m.modalSelector)

	err := chromedp.Run(m.test.Context,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		return fmt.Errorf("failed to click submit button in modal: %w", err)
	}

	return nil
}

// WaitForClose waits for the modal to close with a timeout.
func (m *ModalTester) WaitForClose(timeout time.Duration) error {
	m.test.T.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := m.VerifyHidden(); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for modal to close")
}

// WaitForOpen waits for the modal to open with a timeout.
func (m *ModalTester) WaitForOpen(timeout time.Duration) error {
	m.test.T.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := m.VerifyVisible(); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for modal to open")
}

// GetText gets the text content of an element inside the modal.
func (m *ModalTester) GetText(selector string) (string, error) {
	m.test.T.Helper()

	fullSelector := fmt.Sprintf(`%s %s`, m.modalSelector, selector)

	var text string
	err := chromedp.Run(m.test.Context,
		chromedp.Text(fullSelector, &text, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get text from %q: %w", fullSelector, err)
	}

	return text, nil
}

// VerifyText verifies that an element inside the modal contains expected text.
func (m *ModalTester) VerifyText(selector, expectedText string) error {
	m.test.T.Helper()

	text, err := m.GetText(selector)
	if err != nil {
		return err
	}

	if text != expectedText {
		return fmt.Errorf("modal element %q has text %q, expected %q", selector, text, expectedText)
	}

	return nil
}
