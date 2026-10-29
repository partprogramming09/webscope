package input

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestArgumentsAreReadFromArgsAndFromTheTopLevel(t *testing.T) {
	cases := map[string]string{
		"nested":    `{"action":"consumers","args":{"table":"quotes"}}`,
		"top level": `{"action":"consumers","table":"quotes"}`,
		"args win":  `{"action":"consumers","table":"nope","args":{"table":"quotes"}}`,
	}
	for name, raw := range cases {
		var in Args
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if in.Action != "consumers" || Str(in.Args, "table") != "quotes" {
			t.Errorf("%s: %+v", name, in)
		}
	}
}

func TestACallWithTheWrongTypesIsRefused(t *testing.T) {
	for _, raw := range []string{`{"action":1}`, `{"action":"fields","args":"x"}`} {
		if err := json.Unmarshal([]byte(raw), &Args{}); err == nil {
			t.Errorf("%s is accepted", raw)
		}
	}
}

func TestStrIsEmptyWhenTheValueIsNotAText(t *testing.T) {
	if got := Str(map[string]any{"n": 3}, "n"); got != "" {
		t.Errorf("%q", got)
	}
}

func TestShowCutsAtFortyCharactersAndKeepsAShortValue(t *testing.T) {
	if got := Show("short"); got != "short" {
		t.Errorf("%q", got)
	}
	if got := Show(strings.Repeat("ñ", 41)); got != strings.Repeat("ñ", 40)+"…" {
		t.Errorf("%q", got)
	}
}
