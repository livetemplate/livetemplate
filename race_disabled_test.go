//go:build !race

package livetemplate

// raceEnabled is false in normal (non-race) builds. See race_enabled_test.go.
const raceEnabled = false
