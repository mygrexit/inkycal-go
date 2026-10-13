//go:build linux

package epd

func init() {
	for _, m := range []string{"image_file", "image_file_12_in_48"} {
		openers[m] = func() (Display, error) { return NewImageFile(m, "") }
	}
}
