package builder

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

const maxFont = 5 << 20

// fontName keeps uploads to plain file names inside fonts/.
var fontName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,59}\.(?i:ttf|otf)$`)

func (s *Server) fontDir() string { return filepath.Join(filepath.Dir(s.cfg.SettingsPath), "fonts") }

// coverage samples tell which scripts a font can write.
var coverage = map[string]string{"latin": "AZaz09", "german": "äöüßÄÖÜ", "greek": "αβγΩώ", "cyrillic": "абвЖЯ"}

type fontInfo struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"` // as written into settings.json
	Coverage map[string]bool `json:"coverage"`
}

func (s *Server) listFonts(w http.ResponseWriter, _ *http.Request) {
	out := []fontInfo{}
	entries, _ := os.ReadDir(s.fontDir())
	for _, e := range entries {
		if e.IsDir() || !fontName.MatchString(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.fontDir(), e.Name()))
		if err != nil {
			continue
		}
		f, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		info := fontInfo{Name: e.Name(), Path: "fonts/" + e.Name(), Coverage: map[string]bool{}}
		var buf sfnt.Buffer
		for script, sample := range coverage {
			ok := true
			for _, r := range sample {
				if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
					ok = false
					break
				}
			}
			info.Coverage[script] = ok
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, out)
}

func (s *Server) putFont(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !fontName.MatchString(name) {
		http.Error(w, "font name: letters, digits, . _ - and .ttf or .otf", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFont))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "font larger than 5 MiB", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	if _, err := opentype.Parse(data); err != nil {
		http.Error(w, "not a TrueType or OpenType font: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(s.fontDir(), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmp, err := os.CreateTemp(s.fontDir(), ".upload-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644) // fonts are not secret
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(s.fontDir(), name))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, fontInfo{Name: name, Path: "fonts/" + name})
}
