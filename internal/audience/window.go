package audience

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type Message struct {
	ID       string
	Username string
	Text     string
	At       time.Time
}

type Intent struct {
	Label   string
	Support float64
}

type Snapshot struct {
	WindowSeconds int
	MessageCount  int
	Summary       string
	Intents       []Intent
	Messages      []string
	Revision      uint64
}

type Window struct {
	mu       sync.Mutex
	duration time.Duration
	messages []Message
	revision uint64
}

func NewWindow(duration time.Duration) *Window {
	return &Window{duration: duration}
}

func (w *Window) Add(message Message) {
	message.Text = strings.TrimSpace(message.Text)
	if message.Text == "" {
		return
	}
	if message.At.IsZero() {
		message.At = time.Now().UTC()
	}
	w.mu.Lock()
	w.prune(message.At)
	for _, existing := range w.messages {
		if message.ID != "" && existing.ID == message.ID {
			w.mu.Unlock()
			return
		}
	}
	w.messages = append(w.messages, message)
	w.revision++
	w.mu.Unlock()
}

func (w *Window) Snapshot(now time.Time) Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.prune(now)

	texts := make([]string, 0, len(w.messages))
	counts := make(map[string]int)
	for _, message := range w.messages {
		texts = append(texts, message.Text)
		counts[message.Text]++
	}
	if len(texts) > 30 {
		texts = texts[len(texts)-30:]
	}
	type count struct {
		text  string
		value int
	}
	ordered := make([]count, 0, len(counts))
	for text, value := range counts {
		ordered = append(ordered, count{text: text, value: value})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].value == ordered[j].value {
			return ordered[i].text < ordered[j].text
		}
		return ordered[i].value > ordered[j].value
	})
	if len(ordered) > 3 {
		ordered = ordered[:3]
	}
	intents := make([]Intent, 0, len(ordered))
	labels := make([]string, 0, len(ordered))
	for _, item := range ordered {
		intents = append(intents, Intent{Label: item.text, Support: float64(item.value) / float64(len(w.messages))})
		labels = append(labels, item.text)
	}
	return Snapshot{
		WindowSeconds: int(w.duration.Seconds()), MessageCount: len(w.messages),
		Summary: strings.Join(labels, "；"), Intents: intents, Messages: texts, Revision: w.revision,
	}
}

func (w *Window) prune(now time.Time) {
	cutoff := now.Add(-w.duration)
	first := 0
	for first < len(w.messages) && w.messages[first].At.Before(cutoff) {
		first++
	}
	if first > 0 {
		w.messages = append([]Message(nil), w.messages[first:]...)
		w.revision++
	}
}
