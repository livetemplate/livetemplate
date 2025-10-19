package e2e

import "time"

// E2E test timing constants
// These constants define wait times for various browser operations to make tests
// more maintainable and easier to tune for different environments
const (
	// shortDelay is used for brief pauses between operations (e.g., after clicking buttons)
	shortDelay = 500 * time.Millisecond

	// standardDelay is used for typical operations (e.g., waiting for navigation)
	standardDelay = 1 * time.Second

	// formSubmitDelay is used after form submissions to wait for processing and WebSocket updates
	formSubmitDelay = 2 * time.Second

	// modalAnimationDelay is used to wait for modal open/close animations to complete
	modalAnimationDelay = 3 * time.Second

	// quickPollDelay is used for rapid polling checks (e.g., waiting for server readiness)
	quickPollDelay = 200 * time.Millisecond
)
