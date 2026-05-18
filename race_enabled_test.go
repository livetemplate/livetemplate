//go:build race

package livetemplate

// raceEnabled is true when the binary is built with -race. Build-tagged
// sentinel (the canonical Go idiom; stdlib has no runtime race query) used to
// skip wall-clock latency assertions whose ceilings are meaningless under the
// race detector's instrumentation overhead.
const raceEnabled = true
