// Package problem sorts the errors a user can hit into kinds, so the panel (app/hint.go)
// and the builder (builder/explain.go) give the same advice from one place.
//
// Producers mark their errors with Mark where the kind is known (a setting is missing,
// a calendar URL returns a web page); errors from the network and the standard library
// are recognised by type. The error text stays as it was, for logs.
package problem

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"syscall"

	"inkycal/internal/fetch"
)

// Kind is what went wrong, as far as advice to the user goes.
type Kind int

const (
	Unknown       Kind = iota // nothing better to say than the error itself
	UnknownModule             // the module name is not in the registry
	Missing                   // a required setting is empty
	BadSetting                // a setting has the wrong type or format
	Style                     // a tile's style is invalid
	Placement                 // a tile has no cell, or its cell is outside the grid or overlaps
	TooSmall                  // the tile is smaller than the module needs
	Padding                   // the inner margin is larger than the tile
	Denied                    // HTTP 401/403: key or token wrong
	NotFound                  // HTTP 404, or a local file that does not exist
	Unavailable               // HTTP 429 or 5xx: the service is busy or down
	Offline                   // no route, no DNS, connection refused
	Timeout                   // the service or the module took too long
	NotCalendar               // a calendar address returns a web page or other non-iCal content
	BadResponse               // the service answered with data the module cannot read
)

var names = [...]string{"unknown", "unknown-module", "missing", "bad-setting", "style", "placement",
	"too-small", "padding", "denied", "not-found", "unavailable", "offline", "timeout", "not-calendar", "bad-response"}

func (k Kind) String() string {
	if k < 0 || int(k) >= len(names) {
		return "unknown"
	}
	return names[k]
}

// Mark gives err the kind k without changing its text.
func Mark(k Kind, err error) error {
	if err == nil {
		return nil
	}
	return &marked{kind: k, err: err}
}

type marked struct {
	kind Kind
	err  error
}

func (m *marked) Error() string { return m.err.Error() }
func (m *marked) Unwrap() error { return m.err }

// ForStatus is the kind of an HTTP status code (Unknown for codes without advice).
func ForStatus(code int) Kind {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return Denied
	case code == http.StatusNotFound || code == http.StatusGone:
		return NotFound
	case code == http.StatusTooManyRequests || code >= 500:
		return Unavailable
	}
	return Unknown
}

// Classify returns the kind of err. A mark wins over the type of what it wraps, so a
// module can say "missing" about an error that happens to contain a JSON error.
func Classify(err error) Kind {
	if err == nil {
		return Unknown
	}
	if m, ok := errors.AsType[*marked](err); ok {
		return m.kind
	}
	if se, ok := errors.AsType[*fetch.StatusError](err); ok {
		return ForStatus(se.Code)
	}
	if _, ok := errors.AsType[*fetch.BadResponseError](err); ok {
		return BadResponse
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return Timeout
	}
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return Offline
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ECONNRESET) {
		return Offline
	}
	if errors.Is(err, fs.ErrNotExist) {
		return NotFound
	}
	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return BadSetting // responses decode through fetch, which marks them
	}
	if _, ok := errors.AsType[*json.SyntaxError](err); ok {
		return BadResponse // settings.json is read before any module runs: this came from a service
	}
	return Unknown
}
