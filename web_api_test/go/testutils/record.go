// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

// ------------------------------------------------------------------------
// Record and RecordStore
// ------------------------------------------------------------------------

// Record represents a generic data record with common metadata.
type Record struct {
	ID        interface{}            `json:"id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// NewRecord creates a new record with the given ID and data.
// CreatedAt and UpdatedAt are set to the current time.
func NewRecord(id interface{}, data map[string]interface{}) *Record {
	now := time.Now()
	return &Record{
		ID:        id,
		Data:      data,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Update replaces the record's data and updates the UpdatedAt timestamp.
func (r *Record) Update(data map[string]interface{}) {
	r.Data = data
	r.UpdatedAt = time.Now()
}

// RecordStore is a thread-safe in‑memory store for Record objects.
type RecordStore struct {
	mu      sync.RWMutex
	records map[interface{}]*Record
}

// NewRecordStore creates a new empty record store.
func NewRecordStore() *RecordStore {
	return &RecordStore{
		records: make(map[interface{}]*Record),
	}
}

// Get retrieves a record by ID. Returns nil if the record does not exist.
func (s *RecordStore) Get(id interface{}) *Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.records[id]
}

// Set stores or replaces a record.
func (s *RecordStore) Set(record *Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.ID] = record
}

// Delete removes a record by ID.
func (s *RecordStore) Delete(id interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, id)
}

// All returns a copy of all records in the store.
func (s *RecordStore) All() []*Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]*Record, 0, len(s.records))
	for _, rec := range s.records {
		all = append(all, rec)
	}
	return all
}

// ------------------------------------------------------------------------
// HTTPRecorder
// ------------------------------------------------------------------------

// Interaction represents a single HTTP request/response pair captured by Recorder.
type Interaction struct {
	Request      *http.Request
	Response     *http.Response
	RequestBody  []byte
	ResponseBody []byte
}

// Recorder wraps an HTTP client and records all interactions.
type Recorder struct {
	client       *http.Client
	Interactions []*Interaction
	mu           sync.Mutex
}

// NewRecorder creates a new Recorder that uses the given client.
// If client is nil, http.DefaultClient is used.
func NewRecorder(client *http.Client) *Recorder {
	if client == nil {
		client = http.DefaultClient
	}
	return &Recorder{
		client:       client,
		Interactions: make([]*Interaction, 0),
	}
}

// Do executes an HTTP request and records the interaction.
func (r *Recorder) Do(req *http.Request) (*http.Response, error) {
	// Capture request body
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = ioutil.ReadAll(req.Body)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
	}

	// Perform the request
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Capture response body
	var respBody []byte
	if resp.Body != nil {
		respBody, _ = ioutil.ReadAll(resp.Body)
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(respBody))
	}

	// Store interaction
	r.mu.Lock()
	r.Interactions = append(r.Interactions, &Interaction{
		Request:      req,
		Response:     resp,
		RequestBody:  reqBody,
		ResponseBody: respBody,
	})
	r.mu.Unlock()

	return resp, nil
}

// Reset clears all recorded interactions.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Interactions = make([]*Interaction, 0)
}