package image

import (
	"context"
	"encoding/json"
	"fmt"
	goimage "image"
	"net/http"
	"strconv"
	"strings"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/problem"
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
	var stale bool
	var err error
	if len(m.body) == 0 {
		img, stale, err = picture.Load(ctx, env.Fetch, url)
	} else {
		img, stale, err = post(ctx, env.Fetch, url, m.body)
	}
	env.Stale = env.Stale || stale
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	return m.look.place(img, w, h, int(m.common.FontSize)), nil
}

// post sends items as a JSON list and decodes the image in the answer; offline it is
// the last image the server sent for the same items.
func post(ctx context.Context, fc *fetch.Client, target string, items []string) (goimage.Image, bool, error) {
	payload, err := json.Marshal(items)
	if err != nil {
		return nil, false, err
	}
	res, err := fc.Post(ctx, target, http.Header{"Content-Type": {"application/json"}}, payload)
	if err != nil {
		return nil, false, err
	}
	img, err := picture.Decode(res.Body)
	return img, res.Stale, err
}
