package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlexInt(t *testing.T) {
	cases := map[string]FlexInt{`5`: 5, `"7"`: 7, `null`: 0, `""`: 0, `12.0`: 12}
	for in, want := range cases {
		var got FlexInt
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %d want %d", in, got, want)
		}
	}
	var bad FlexInt
	if err := json.Unmarshal([]byte(`"abc"`), &bad); err == nil {
		t.Error("expected error for \"abc\"")
	}
}

func TestFlexFloatAndString(t *testing.T) {
	floats := map[string]FlexFloat{`51.5`: 51.5, `" -0.25 "`: -0.25, `13`: 13}
	for in, want := range floats {
		var got FlexFloat
		if err := json.Unmarshal([]byte(in), &got); err != nil || got != want {
			t.Errorf("%s: got %v, %v want %v", in, got, err, want)
		}
	}
	for _, in := range []string{`"abc"`, `""`, `"NaN"`, `true`} {
		var bad FlexFloat
		if json.Unmarshal([]byte(in), &bad) == nil {
			t.Errorf("%s: want an error", in)
		}
	}
	strs := map[string]FlexString{`"Berlin,DE"`: "Berlin,DE", `2950159`: "2950159", `3.0`: "3.0"}
	for in, want := range strs {
		var got FlexString
		if err := json.Unmarshal([]byte(in), &got); err != nil || got != want {
			t.Errorf("%s: got %q, %v want %q", in, got, err, want)
		}
	}
}

func TestFlexBool(t *testing.T) {
	cases := map[string]FlexBool{`true`: true, `false`: false, `"True"`: true, `"false"`: false, `null`: false}
	for in, want := range cases {
		var got FlexBool
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %v want %v", in, got, want)
		}
	}
	var bad FlexBool
	if err := json.Unmarshal([]byte(`"yes"`), &bad); err == nil {
		t.Error("expected error for \"yes\"")
	}
}

func TestStringList(t *testing.T) {
	cases := map[string]StringList{
		`"https://a/x.ics, https://b/y.ics"`: {"https://a/x.ics", "https://b/y.ics"},
		`["a", " b "]`:                       {"a", "b"},
		`"a,,"`:                              {"a"},
		`null`:                               nil,
		`""`:                                 {},
	}
	for in, want := range cases {
		var got StringList
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v want %#v", in, got, want)
		}
	}
}
