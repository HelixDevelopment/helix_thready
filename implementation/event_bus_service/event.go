// Package eventbusservice is the client-facing core of the Helix Thready
// Event Bus service (digital.vasic.eventbusservice).
//
// It realises the behavioural contract described in
// docs/public/research/mvp/architecture/event-model.md and
// docs/public/research/mvp/api/event-bus-contract.md: a typed in-process
// publish/subscribe bus with subscription filters (glob subject + metadata),
// **sticky** last-value events with explicit invalidation, and **durable**
// subscribers backed by an append log that replay missed events from a cursor
// to give at-least-once delivery to disconnected consumers.
//
// This module is the in-process engine only. The NATS JetStream transport that
// the architecture docs describe is an out-of-scope adapter seam: this core
// deliberately models the same guarantees (ordered durable log = the JetStream
// stream; per-subject last-value = the compacted/KV sticky store) so a JetStream
// adapter can be layered on top without changing the client-facing surface.
//
// The module is self-contained and depends only on the Go standard library.
package eventbusservice

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

// Event is the envelope fanned out to subscribers. Type/Subject/Payload/
// TimestampUnix/Metadata are the caller-set fields; ID and Seq are assigned by
// the bus on publish (ID for idempotent consumers, Seq as the durable-replay
// cursor).
type Event struct {
	// ID uniquely identifies this event. Assigned by the bus if empty. Durable
	// consumers dedupe on ID to stay idempotent under at-least-once redelivery.
	ID string
	// Seq is the bus-assigned global publish order (1-based, monotonic). It is
	// the resume cursor for durable subscribers: SubscribeDurable(filter, seq)
	// replays every logged event with Seq > seq.
	Seq int64
	// Type is the dot-notation event type, e.g. "post.received".
	Type string
	// Subject is the routing key subscribers filter on (glob-matchable),
	// e.g. "post.state.9c1e".
	Subject string
	// Payload is the event body. Left opaque (any) exactly like the verified
	// event.Event.Payload interface{} in the architecture doc.
	Payload any
	// TimestampUnix is the publish time in Unix seconds. Assigned by the bus if
	// zero.
	TimestampUnix int64
	// Metadata carries routing/tenant attributes (e.g. account_id) that filters
	// can match on.
	Metadata map[string]string
}

// NewEvent builds an Event with an empty metadata map ready for WithMetadata.
// ID/Seq/TimestampUnix are assigned by the bus at publish time.
func NewEvent(subject, eventType string, payload any) Event {
	return Event{
		Type:     eventType,
		Subject:  subject,
		Payload:  payload,
		Metadata: map[string]string{},
	}
}

// WithMetadata returns a copy of e with key=value added to its metadata. It does
// not mutate the receiver's map.
func (e Event) WithMetadata(key, value string) Event {
	m := make(map[string]string, len(e.Metadata)+1)
	for k, v := range e.Metadata {
		m[k] = v
	}
	m[key] = value
	e.Metadata = m
	return e
}

// Filter selects which events a subscription receives. An event matches when its
// Subject matches the glob Subject pattern AND every entry in Metadata is present
// and equal on the event. A zero Filter (empty Subject, nil Metadata) matches
// every event — this is what SubscribeAll uses.
type Filter struct {
	// Subject is a NATS-style token glob (see MatchSubject). "" or ">" match any
	// subject.
	Subject string
	// Metadata entries must all be present and equal on the event.
	Metadata map[string]string
}

// Matches reports whether e satisfies the filter.
func (f Filter) Matches(e Event) bool {
	if !MatchSubject(f.Subject, e.Subject) {
		return false
	}
	for k, v := range f.Metadata {
		if e.Metadata == nil || e.Metadata[k] != v {
			return false
		}
	}
	return true
}

// MatchSubject reports whether subject matches a NATS-style token glob pattern.
//
// Tokens are split on '.'. The rules are:
//   - ""  and ">"          match any subject (used for subscribe-all).
//   - "*" matches exactly one token in that position (any content).
//   - ">" as the final token matches one OR MORE remaining tokens (a tail).
//   - any other token must equal the subject token in that position.
//
// A plain literal pattern therefore requires an exact, token-count-equal match.
// Examples: "post.*" matches "post.received" but NOT "post.received.web";
// "post.>" matches both "post.received" and "post.received.web"; "post.received"
// matches only itself.
func MatchSubject(pattern, subject string) bool {
	if pattern == "" || pattern == ">" {
		return true
	}
	p := strings.Split(pattern, ".")
	s := strings.Split(subject, ".")
	for i := 0; i < len(p); i++ {
		if p[i] == ">" {
			// Multi-token tail wildcard: valid only as the last pattern token,
			// and it must cover at least one subject token.
			return i == len(p)-1 && i < len(s)
		}
		if i >= len(s) {
			return false
		}
		if p[i] == "*" {
			continue
		}
		if p[i] != s[i] {
			return false
		}
	}
	return len(p) == len(s)
}

// newID returns a UUIDv4-style identifier. It uses crypto/rand and falls back to
// a time-seeded value only if the entropy source fails, so it never returns "".
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		binary.BigEndian.PutUint64(b[0:8], uint64(time.Now().UnixNano()))
		binary.BigEndian.PutUint64(b[8:16], uint64(time.Now().UnixNano()))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
