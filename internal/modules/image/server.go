package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	goimage "image"
	"inkycal/internal/problem"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/render"
)

// Server is upstream's Inkyserver: an image rendered by an Inkycal server
// (https://inkycal.robertsirre.nl/) for exactly this module's size. path may hold
// {width} and {height}; with path_body the items are POSTed as a JSON list.
type Server struct {
	common config.Common
	path   string
	body   []string
	look   look
}

// NewServer builds the module registered as "Inkyserver".
func NewServer(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var cfg struct {
		Path     string            `json:"path"`
		PathBody config.StringList `json:"path_body"`
		rawLook
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	if cfg.Path = strings.TrimSpace(cfg.Path); cfg.Path == "" {
		return nil, problem.Mark(problem.Missing, fmt.Errorf("server: config is missing path"))
	}
	if err := modules.CheckMinSize(c, ServerMeta); err != nil {
		return nil, err
	}
	l, err := cfg.rawLook.parse("server", false) // Python: "Dither … Default is False."
	if err != nil {
		return nil, err
	}
	return &Server{common: c, path: cfg.Path, body: cfg.PathBody, look: l}, nil
}

// serverURL fills in the module size like Python's str.format(width=…, height=…).
func serverURL(path string, w, h int) string {
	return strings.NewReplacer("{width}", strconv.Itoa(w), "{height}", strconv.Itoa(h), "{{", "{", "}}", "}").Replace(path)
}

func (m *Server) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.common.InnerSize()
	url := serverURL(m.path, w, h)
	var img goimage.Image
	var err error
	if len(m.body) == 0 {
		var stale bool
		img, stale, err = picture.Load(ctx, env.Fetch, url)
		env.Stale = env.Stale || stale
	} else {
		img, err = post(ctx, env.Fetch.HTTP, url, m.body)
	}
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	return m.look.place(img, w, h, int(m.common.FontSize)), nil
}

const maxImage = 20 << 20

// post sends items as a JSON list and decodes the image in the answer. fetch.Client
// only does GET, so this request has no retry or cache fallback.
func post(ctx context.Context, hc *http.Client, target string, items []string) (goimage.Image, error) {
	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "inkycal-go")
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		if ue, ok := errors.AsType[*url.Error](err); ok {
			err = fmt.Errorf("POST %s: %w", fetch.Redact(target), ue.Err) // url.Error would log tokens in the query
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImage))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, problem.Mark(problem.ForStatus(resp.StatusCode), fmt.Errorf("POST: HTTP %d", resp.StatusCode))
	}
	return picture.Decode(data)
}
