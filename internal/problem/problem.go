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

	numKinds // keep last
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

// Classify returns the kind of err. It walks the error tree in order and takes the first
// error with a kind, so with errors.Join (several calendars, two weather requests) the
// first failure decides. A mark wins over the error it wraps, so a module can say
// "missing" about an error that happens to contain a JSON error.
func Classify(err error) Kind {
	if err == nil {
		return Unknown
	}
	if k := kindOf(err); k != Unknown {
		return k
	}
	switch u := err.(type) {
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			if k := Classify(e); k != Unknown {
				return k
			}
		}
	case interface{ Unwrap() error }:
		return Classify(u.Unwrap())
	}
	return Unknown
}

// kindOf is the kind of err itself, not of what it wraps.
func kindOf(err error) Kind {
	switch e := err.(type) {
	case *marked:
		return e.kind
	case *fetch.StatusError:
		return ForStatus(e.Code)
	case *fetch.BadResponseError:
		return BadResponse
	case *net.DNSError:
		return Offline
	case *net.OpError:
		if e.Op == "dial" && e.Timeout() {
			return Offline // no way out (no route, captive Wi-Fi), not a slow service
		}
	case *json.UnmarshalTypeError:
		return BadSetting // responses decode through fetch, which marks them
	case *json.SyntaxError:
		return BadResponse // settings.json errors are marked by config
	case syscall.Errno:
		switch e {
		case syscall.ECONNREFUSED, syscall.ENETUNREACH, syscall.EHOSTUNREACH, syscall.ECONNRESET:
			return Offline
		case syscall.ENOENT:
			return NotFound
		}
	}
	switch {
	case err == context.DeadlineExceeded:
		return Timeout
	case err == fs.ErrNotExist:
		return NotFound
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return Timeout
	}
	return Unknown
}
