package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexInt accepts 5, "5" and null (→ 0). The Python web UI is not strict about types.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*f = 0
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("not an integer: %q", s)
		}
		*f = FlexInt(n)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexInt(int(n))
	return nil
}

// FlexBool accepts true/false, "true"/"True"/"false"/"False" and null (→ false).
type FlexBool bool

func (f *FlexBool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch strings.ToLower(strings.Trim(string(b), `"`)) {
	case "true":
		*f = true
	case "false", "null", "":
		*f = false
	default:
		return fmt.Errorf("not a boolean: %s", b)
	}
	return nil
}

// StringList accepts "a, b" (comma separated), ["a","b"] and null.
type StringList []string

func (s *StringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*s = nil
		return nil
	}
	var raw []string
	if len(b) > 0 && b[0] == '[' {
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
	} else {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		raw = strings.Split(one, ",")
	}
	out := StringList{}
	for _, v := range raw {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	*s = out
	return nil
}
