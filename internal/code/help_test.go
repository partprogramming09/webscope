package code

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestHelpListsEveryActionAndSaysWhenToUseThePlaybook(t *testing.T) {
	out, err := code(t, codeSetup(t), "help", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range codeActions {
		if !strings.Contains(out, a) {
			t.Errorf("help does not name the action %q:\n%s", a, out)
		}
	}
	for _, want := range []string{"migration", "parity", "legacy"} {
		if !strings.Contains(out, want) {
			t.Errorf("help must say when to ask for the migration playbook (lacks %q):\n%s", want, out)
		}
	}
}

func TestEveryActionOfTheListIsKnownToTheDispatcher(t *testing.T) {
	c := codeSetup(t)
	for _, a := range codeActions {
		if _, err := code(t, c, a, map[string]any{}); err != nil && strings.Contains(err.Error(), "unknown action") {
			t.Errorf("%q is listed but the dispatcher does not know it: %v", a, err)
		}
	}
}

func TestMigrationPlaybookOnlyNamesActionsThatExist(t *testing.T) {
	out, err := code(t, codeSetup(t), "help", map[string]any{"topic": "migration"})
	if err != nil {
		t.Fatal(err)
	}
	pageActions := []string{"load", "state", "read", "fill", "click", "press", "payloads", "simulate", "audit", "shape", "parity", "help"}
	for _, m := range regexp.MustCompile("`?\\b(code|page) ([a-z_]+)").FindAllStringSubmatch(out, -1) {
		known := codeActions
		if m[1] == "page" {
			known = pageActions
		}
		if !slices.Contains(known, m[2]) {
			t.Errorf("the playbook names %s %s, which is not an action", m[1], m[2])
		}
	}
	for _, want := range []string{"code flows", "code legacy", "page shape", "page parity", "code rows", "code cases", "code snap_diff", "ZZ_"} {
		if !strings.Contains(out, want) {
			t.Errorf("the playbook lacks %q:\n%s", want, out)
		}
	}
	if len(out) > 4000 {
		t.Errorf("the playbook is %d bytes; keep it under 4000 so it costs little to read", len(out))
	}
}

func TestHelpRefusesAnUnknownTopicAndListsTheKnownOnes(t *testing.T) {
	_, err := code(t, codeSetup(t), "help", map[string]any{"topic": "nope"})
	if err == nil || !strings.Contains(err.Error(), "migration") {
		t.Errorf("an unknown topic must list the known ones, got %v", err)
	}
}
