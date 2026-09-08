package session

import (
	"context"
	"sync"
)

// workGroup permits one in-flight task per lane. The runtime loop alone owns
// active; completions return through done rather than changing it in workers.
type workGroup struct {
	active map[string]bool
	done   chan string
	wg     sync.WaitGroup
}

func newWorkGroup() *workGroup {
	return &workGroup{active: make(map[string]bool), done: make(chan string, 4)}
}

func (w *workGroup) start(ctx context.Context, name string, work func(context.Context)) {
	if w.active[name] || ctx.Err() != nil {
		return
	}
	w.active[name] = true
	w.wg.Go(func() {
		defer func() { w.done <- name }()
		work(ctx)
	})
}
