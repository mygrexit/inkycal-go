// Package fetch is the shared HTTP client: timeouts, retries and a disk cache used as fallback
// when the network fails, so the display shows slightly old data instead of an error.
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"
)

const maxBody = 20 << 20 // 20 MiB, big calendars included

type Client struct {
	HTTP     *http.Client
	CacheDir string        // "" disables the cache
	Attempts int           // tries per request
	Backoff  time.Duration // wait before the 2nd try, doubled each time
	FreshFor time.Duration // > 0: a cached copy younger than this is served without a request (builder)
}

type Result struct {
	Body      []byte
	Stale     bool // served from cache because the request failed
	FetchedAt time.Time
}

// StatusError is returned for non-2xx responses.
type StatusError struct {
	URL  string
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: HTTP %d: %s", e.URL, e.Code, e.Body)
}

func (e *StatusError) retryable() bool { return e.Code >= 500 || e.Code == http.StatusTooManyRequests }

func New(cacheDir string) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = 15 * time.Second
	return &Client{
		HTTP:     &http.Client{Timeout: 30 * time.Second, Transport: tr},
		CacheDir: cacheDir,
		Attempts: 3,
		Backoff:  500 * time.Millisecond,
	}
}

// Get fetches url. On network errors, 5xx or 429 it retries and finally falls back to the cache.
// 4xx errors (wrong key, wrong URL) are returned directly so misconfiguration stays visible.
func (c *Client) Get(ctx context.Context, url string, header http.Header) (Result, error) {
	key := cacheKey(url, header)
	if c.FreshFor > 0 {
		if cached, at, ok := c.load(key); ok && time.Since(at) < c.FreshFor {
			return Result{Body: cached, FetchedAt: at}, nil
		}
	}
	body, err := c.fetch(ctx, url, header)
	if err == nil {
		c.store(key, body)
		return Result{Body: body, FetchedAt: time.Now()}, nil
	}
	var se *StatusError
	if errors.As(err, &se) && !se.retryable() {
		return Result{}, err
	}
	if cached, at, ok := c.load(key); ok {
		slog.Warn("using cached response", "url", redact(url), "cached_at", at, "err", err)
		return Result{Body: cached, Stale: true, FetchedAt: at}, nil
	}
	return Result{}, err
}

// GetJSON is Get followed by json.Unmarshal into v.
func (c *Client) GetJSON(ctx context.Context, url string, header http.Header, v any) (stale bool, err error) {
	res, err := c.Get(ctx, url, header)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(res.Body, v); err != nil {
		return res.Stale, fmt.Errorf("GET %s: invalid JSON: %w", redact(url), err)
	}
	return res.Stale, nil
}

func (c *Client) fetch(ctx context.Context, url string, header http.Header) ([]byte, error) {
	attempts := max(c.Attempts, 1)
	wait := c.Backoff
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
		}
		body, err := c.once(ctx, url, header)
		if err == nil {
			return body, nil
		}
		lastErr = err
		var se *StatusError
		if errors.As(err, &se) && !se.retryable() {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) once(ctx context.Context, url string, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "inkycal-go")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *neturl.Error
		if errors.As(err, &ue) {
			err = fmt.Errorf("GET %s: %w", redact(url), ue.Err) // url.Error would log the API key
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, &StatusError{URL: redact(url), Code: resp.StatusCode, Body: snippet}
	}
	return body, nil
}

func cacheKey(url string, header http.Header) string {
	sum := sha256.Sum256([]byte(url + "\n" + header.Get("Authorization")))
	return hex.EncodeToString(sum[:])
}

func (c *Client) store(key string, body []byte) {
	if c.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(c.CacheDir, 0o700); err != nil {
		slog.Warn("cache dir", "err", err)
		return
	}
	tmp, err := os.CreateTemp(c.CacheDir, ".tmp-*")
	if err != nil {
		slog.Warn("cache write", "err", err)
		return
	}
	_, werr := tmp.Write(body)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(c.CacheDir, key)); err != nil {
		os.Remove(tmp.Name())
	}
}

// Prune deletes cached responses not refreshed within maxAge. Nothing that old is a
// useful fallback, and URLs that change daily (xkcd random) would pile up otherwise.
func (c *Client) Prune(maxAge time.Duration) {
	if c == nil || c.CacheDir == "" {
		return
	}
	entries, err := os.ReadDir(c.CacheDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() && info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(c.CacheDir, e.Name()))
		}
	}
}

func (c *Client) load(key string) ([]byte, time.Time, bool) {
	if c.CacheDir == "" {
		return nil, time.Time{}, false
	}
	p := filepath.Join(c.CacheDir, key)
	st, err := os.Stat(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	body, err := os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	return body, st.ModTime(), true
}

// Redact drops query strings (API keys!) from URLs before logging.
func Redact(url string) string { return redact(url) }

func redact(url string) string {
	for i := 0; i < len(url); i++ {
		if url[i] == '?' {
			return url[:i] + "?…"
		}
	}
	return url
}
