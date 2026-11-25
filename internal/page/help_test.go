package page

import (
	"regexp"
	"strings"
	"testing"
)

// help without a topic is an index, so the sessions that call it many times pay little; each action stays documented in one topic.

func TestHelpWithoutATopicIsAShortIndexOfTheTopics(t *testing.T) {
	out, err := askPage(t, testPage(t, appOrigin), "help", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "\n") + 1; n > 10 || len(out) > 1500 {
		t.Errorf("the index is %d lines and %d bytes; keep it at 10 lines and 1500 bytes:\n%s", n, len(out), out)
	}
	for _, topic := range helpTopicNames {
		if !strings.Contains(out, "help {topic: \""+topic+"\"}") {
			t.Errorf("the index does not say how to open the topic %s", topic)
		}
	}
}

func TestEveryActionOfPageIsDocumentedInATopic(t *testing.T) {
	var all strings.Builder
	for _, topic := range helpTopicNames {
		out, err := askPage(t, testPage(t, appOrigin), "help", map[string]any{"topic": topic})
		if err != nil || out == "" {
			t.Fatalf("topic %s: %q, %v", topic, out, err)
		}
		all.WriteString(out + "\n")
	}
	for _, a := range []string{"state", "find", "read", "fill", "click", "press", "options", "open", "batch", "upload", "payloads", "simulate", "result", "audit", "shape", "parity", "download", "verify"} {
		// the line of an action starts with its name (or the list state|find|… of the actions that return a snippet)
		if !regexp.MustCompile(`(?m)^\s*(?:[a-z]+\|)*` + a + `(?:\|[a-z]+)*[ {:]`).MatchString(all.String()) {
			t.Errorf("no topic has a line for %s", a)
		}
	}
}

func TestHelpWithAnUnknownTopicListsTheKnownOnesInOneNext(t *testing.T) {
	_, err := askPage(t, testPage(t, appOrigin), "help", map[string]any{"topic": "nope"})
	if err == nil || !strings.Contains(err.Error(), "screen") || !strings.Contains(err.Error(), "Next: page(help)") {
		t.Fatalf("an unknown topic must list the topics: %v", err)
	}
}

func TestTheTopicsTogetherKeepEveryLineOfTheOldHelp(t *testing.T) {
	var all strings.Builder
	for _, topic := range helpTopicNames {
		out, _ := askPage(t, testPage(t, appOrigin), "help", map[string]any{"topic": topic})
		all.WriteString(out)
	}
	for _, want := range []string{"batch {steps", "options {field", "payloads {limit", "download {arm:true}", "verify {claim", "parity {a, b", "upload {path", "simulate {match", "audit {selector"} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("the topics lost %q", want)
		}
	}
}
