package ipc

import (
	"encoding/json"
	"strconv"
	"sync"
)

// Hub tracks ordered channels. A channel is keyed by the calling window
// and the id the frontend put in the command arguments.
type Hub struct {
	mu       sync.Mutex
	channels map[string]*Channel
	emitter  Emitter
}

// Channel is one ordered stream from the host to a single window.
type Channel struct {
	window string
	id     int64

	mu     sync.Mutex
	next   int64
	closed bool
	hub    *Hub
}

// NewHub creates an empty channel hub.
func NewHub() *Hub {
	return &Hub{channels: make(map[string]*Channel)}
}

// SetEmitter attaches the bridge used to deliver channel messages.
func (h *Hub) SetEmitter(e Emitter) {
	h.mu.Lock()
	h.emitter = e
	h.mu.Unlock()
}

// Open binds id for window, or returns the channel already bound there.
func (h *Hub) Open(window string, id int64) (*Channel, error) {
	if id < 1 {
		return nil, Fail(CodeMissingChannel, "ipc: missing channel")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	k := channelKey(window, id)
	if ch := h.channels[k]; ch != nil {
		ch.mu.Lock()
		closed := ch.closed
		ch.mu.Unlock()
		if closed {
			return nil, Fail(CodeChannelClosed, "ipc: channel %d closed", id)
		}
		return ch, nil
	}
	ch := &Channel{window: window, id: id, hub: h}
	h.channels[k] = ch
	return ch, nil
}

// Close marks the window's channel closed. A missing id is ignored.
func (h *Hub) Close(window string, id int64) {
	if id < 1 {
		return
	}
	h.mu.Lock()
	ch := h.channels[channelKey(window, id)]
	h.mu.Unlock()
	if ch == nil {
		return
	}
	ch.mu.Lock()
	ch.closed = true
	ch.mu.Unlock()
}

// Send appends one payload. Indexes start at 0 and increase by one.
func (c *Channel) Send(payload any) error {
	if c == nil {
		return Fail(CodeMissingChannel, "ipc: missing channel")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Fail(CodeChannelClosed, "ipc: channel %d closed", c.id)
	}
	index := c.next
	c.next++
	window, id, hub := c.window, c.id, c.hub
	c.mu.Unlock()
	return hub.deliver(window, id, index, payload)
}

// ID is the frontend channel id.
func (c *Channel) ID() int64 {
	if c == nil {
		return 0
	}
	return c.id
}

func (h *Hub) deliver(window string, id, index int64, payload any) error {
	h.mu.Lock()
	em := h.emitter
	h.mu.Unlock()
	if em == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"type":    "channel",
		"channel": id,
		"index":   index,
		"payload": payload,
	})
	if err != nil {
		return err
	}
	return em.EmitTo(window, string(raw))
}

func channelKey(window string, id int64) string {
	return window + "\x00" + strconv.FormatInt(id, 10)
}

// ParseChannelID finds the single {"__channel": n} value in a command argument
// object. The value may be top-level or one object deep.
func ParseChannelID(argsJSON []byte) (int64, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(argsJSON, &raw); err != nil {
		return 0, Fail(CodeChannelArgs, "ipc: channel args: %s", err.Error())
	}
	var found []int64
	if id, ok := channelNumber(raw["__channel"]); ok {
		found = append(found, id)
	}
	for key, v := range raw {
		if key == "__channel" {
			continue
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(v, &nested) != nil {
			continue
		}
		if id, ok := channelNumber(nested["__channel"]); ok {
			found = append(found, id)
		}
	}
	if len(found) != 1 {
		return 0, Fail(CodeChannelCount, "ipc: want one channel")
	}
	if found[0] < 1 {
		return 0, Fail(CodeMissingChannel, "ipc: missing channel")
	}
	return found[0], nil
}

func channelNumber(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	return n, true
}

// Channel binds the channel referenced by argsJSON to this invoke's window.
func (c Context) Channel(argsJSON []byte) (*Channel, error) {
	if c.channels == nil {
		return nil, Fail(CodeChannelsUnconfigured, "ipc: channels are not configured")
	}
	id, err := ParseChannelID(argsJSON)
	if err != nil {
		return nil, err
	}
	return c.channels.Open(c.Window, id)
}
