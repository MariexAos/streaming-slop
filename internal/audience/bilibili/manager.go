package bilibili

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	blive "github.com/Akegarasu/blivedm-go/client"
	"github.com/Akegarasu/blivedm-go/message"

	"streaming-agent/internal/audience"
)

type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusFailed       Status = "failed"
)

type Config struct {
	RoomID           int    `json:"roomId"`
	CookieConfigured bool   `json:"cookieConfigured"`
	Status           Status `json:"status"`
	LastError        string `json:"-"`
}

type Manager struct {
	mu         sync.Mutex
	window     *audience.Window
	roomID     int
	cookie     string
	status     Status
	lastError  string
	generation uint64
	client     *blive.Client
}

func NewManager(window *audience.Window) *Manager {
	return &Manager{window: window, status: StatusDisconnected}
}

func (m *Manager) Update(roomID int, cookie string) error {
	if roomID <= 0 {
		return errors.New("bilibili room ID must be positive")
	}
	cookie = strings.TrimSpace(cookie)

	m.mu.Lock()
	old := m.client
	if cookie != "" {
		m.cookie = cookie
	}
	m.roomID = roomID
	m.status = StatusConnecting
	m.lastError = ""
	m.generation++
	generation := m.generation
	configuredCookie := m.cookie
	client := blive.NewClient(roomID)
	if configuredCookie != "" {
		client.SetCookie(configuredCookie)
	}
	client.OnDanmaku(func(danmaku *message.Danmaku) {
		m.receive(generation, danmaku)
	})
	m.client = client
	m.mu.Unlock()

	if old != nil {
		old.Stop()
	}
	go m.start(generation, client)
	return nil
}

func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Config{
		RoomID: m.roomID, CookieConfigured: m.cookie != "", Status: m.status, LastError: m.lastError,
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	client := m.client
	m.client = nil
	m.status = StatusDisconnected
	m.generation++
	m.mu.Unlock()
	if client != nil {
		client.Stop()
	}
}

func (m *Manager) start(generation uint64, client *blive.Client) {
	err := client.Start()
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation != m.generation || client != m.client {
		return
	}
	if err != nil {
		m.status = StatusFailed
		m.lastError = err.Error()
		return
	}
	m.status = StatusConnected
	m.lastError = ""
}

func (m *Manager) receive(generation uint64, danmaku *message.Danmaku) {
	m.mu.Lock()
	current := generation == m.generation
	m.mu.Unlock()
	if !current {
		return
	}
	if danmaku == nil || danmaku.Type != message.TextDanmaku {
		return
	}
	username := ""
	if danmaku.Sender != nil {
		username = danmaku.Sender.Uname
	}
	id := ""
	if danmaku.Extra != nil {
		id = danmaku.Extra.IDStr
	}
	if id == "" {
		id = fmt.Sprintf("%d:%s:%s", danmaku.Timestamp, username, danmaku.Content)
	}
	at := time.UnixMilli(danmaku.Timestamp)
	if danmaku.Timestamp < 1_000_000_000_000 {
		at = time.Unix(danmaku.Timestamp, 0)
	}
	m.window.Add(audience.Message{ID: id, Username: username, Text: danmaku.Content, At: at.UTC()})
}
