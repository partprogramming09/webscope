// Package input is the shape of the call that page and code take: an action and its arguments.
package input

import (
	"encoding/json"
	"strings"
)

// MaxAnswer is the most bytes that one answer of a tool carries.
const MaxAnswer = 8000

// Schema is written by hand, like the one of check: the generated one costs twice the bytes of tools/list.
var Schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action": map[string]any{"type": "string"}, "args": map[string]any{"type": "object"},
	},
}

// Args is one call: the action and its arguments, which sit in "args" or at the top level of the call.
type Args struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// UnmarshalJSON reads the arguments from "args" (an object, or text holding one) and also from the top level of the call; a key inside "args" wins. A call with no action and help set means the action help.
func (a *Args) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*a = Args{Args: map[string]any{}}
	for k, v := range raw {
		switch k {
		case "action":
			if err := json.Unmarshal(v, &a.Action); err != nil {
				return err
			}
		case "args":
		default:
			var x any
			if err := json.Unmarshal(v, &x); err != nil {
				return err
			}
			a.Args[k] = x
		}
	}
	if v, ok := raw["args"]; ok {
		var text string
		if json.Unmarshal(v, &text) == nil && strings.HasPrefix(strings.TrimSpace(text), "{") {
			v = json.RawMessage(text)
		}
		var nested map[string]any
		if err := json.Unmarshal(v, &nested); err != nil {
			return err
		}
		for k, x := range nested {
			a.Args[k] = x
		}
	}
	if h, ok := a.Args["help"]; ok && a.Action == "" && (h == true || h == "true") {
		a.Action = "help"
		delete(a.Args, "help")
	}
	return nil
}

// Str is the text argument k, or "" when it is missing or not a text.
func Str(args map[string]any, k string) string { s, _ := args[k].(string); return s }

// Show is a value for an answer: at most 40 characters, with an ellipsis when it was cut.
func Show(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}
