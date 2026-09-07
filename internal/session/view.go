package session

import (
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/live"
)

// View is a detached snapshot for read-only presentation.
type View struct {
	Session     *live.LiveSession
	Audience    audience.Snapshot
	Observation audience.Observation
}

func (r *Runtime) View() View {
	r.mu.Lock()
	defer r.mu.Unlock()
	view := View{Observation: r.lastObservation}
	if r.session != nil {
		value := *r.session
		value.Timeline.Segments = append([]live.Segment(nil), r.session.Timeline.Segments...)
		view.Session = &value
	}
	if r.config.Audience != nil {
		view.Audience = r.config.Audience.Snapshot(time.Now().UTC())
	}
	return view
}
