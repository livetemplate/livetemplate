package livetemplate

import "context"

// asyncContinuation is an erased-type async operation registered by Async[S, R]().
// The event loop spawns work in a goroutine after render #1; on completion it
// enqueues apply onto the connection's DispatchChan for on-loop execution.
type asyncContinuation struct {
	work  func(ctx context.Context) (any, error)
	apply func(state any, result any, err error) (any, error)
}

// Async runs work off the connection event loop, then re-enters the loop to
// apply its result to the CURRENT session state and re-render this connection.
//
//   - work runs in a supervised goroutine bound to the connection's context;
//     it must NOT touch session state (it has none) — only its own inputs.
//   - apply runs ON the event loop against the latest state at completion time
//     (exactly like a dispatched action), so it composes safely with any edits
//     that landed during the async window. It should mutate only the fields it owns.
//   - If the connection closes first, work is cancelled and apply never runs.
//
// On HTTP/fetch (no persistent connection), pending async operations are
// silently dropped — the action returns synchronously with its state changes.
func Async[S any, R any](
	ctx *Context,
	work func(context.Context) (R, error),
	apply func(s S, result R, err error) (S, error),
) {
	ctx.pendingAsync = append(ctx.pendingAsync, asyncContinuation{
		work: func(ctx context.Context) (any, error) {
			return work(ctx)
		},
		apply: func(state any, result any, err error) (any, error) {
			s := state.(S)
			var r R
			if result != nil {
				r = result.(R)
			}
			return apply(s, r, err)
		},
	})
}
