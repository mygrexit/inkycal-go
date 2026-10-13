//go:build !linux

package epd

import "errors"

// Open is only available on Linux; use NewPNG (dry-run) elsewhere.
func Open(model string) (Display, error) {
	return nil, errors.New("hardware displays are only supported on Linux")
}
