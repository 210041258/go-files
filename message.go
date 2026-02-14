// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"encoding/json"
	"time"
)

// Message represents a generic message with an ID, content, and timestamp.
type Message struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// NewMessage creates a new Message with the given content.
// The ID is set to 0 by default, and CreatedAt is set to the current time.
func NewMessage(content string) *Message {
	return &Message{
		Content:   content,
		CreatedAt: time.Now(),
	}
}

// ToJSON returns the JSON encoding of the message.
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses the JSON data and populates the message.
func (m *Message) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}