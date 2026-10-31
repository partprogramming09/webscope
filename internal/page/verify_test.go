package page

import (
	"strings"
	"testing"
)

func verifyOf(t *testing.T, p *Tool, claim, evidence string) string {
	t.Helper()
	out, err := askPage(t, p, "verify", map[string]any{"claim": claim, "evidence": evidence})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestVerifyStateIsDecidedByTheServerFromTheEvidence(t *testing.T) {
	p := testPage(t, appOrigin)
	p.caps = []Capture{{M: "POST", U: "http://x/api/quotes", S: 201}}
	if out := verifyOf(t, p, "saving creates the quote", "capture POST /api/quotes"); !strings.Contains(out, "OBSERVADO") {
		t.Errorf("a real capture must be OBSERVADO: %q", out)
	}
	if out := verifyOf(t, p, "the quote is deleted", "capture DELETE /api/quotes"); !strings.Contains(out, "NO VERIFICADO") {
		t.Errorf("a capture that does not exist must be NO VERIFICADO: %q", out)
	}
	if out := verifyOf(t, p, "the export downloads", "I saw it work"); !strings.Contains(out, "NO VERIFICADO") {
		t.Errorf("free text is not a reference: %q", out)
	}
	if out := verifyOf(t, p, "the export downloads", "download"); !strings.Contains(out, "NO VERIFICADO") {
		t.Errorf("download evidence needs a file seen by download wait: %q", out)
	}
	p.dlLast = "export.xlsx"
	if out := verifyOf(t, p, "the export downloads", "download"); !strings.Contains(out, "OBSERVADO") {
		t.Errorf("a file seen on disk must be OBSERVADO: %q", out)
	}
}

func TestVerifyDowngradesACaptureSeenThroughAnInterceptor(t *testing.T) {
	p := testPage(t, appOrigin)
	p.caps = []Capture{{M: "POST", U: "http://x/api/quotes", S: 201}}
	p.tamperedEver = true
	out := verifyOf(t, p, "saving creates the quote", "capture POST /api/quotes")
	if !strings.Contains(out, "INFERIDO") || strings.Contains(out, "OBSERVADO") {
		t.Errorf("an interceptor was seen: the capture is INFERIDO: %q", out)
	}
}

func TestVerifyReportComesFromTheTableAndAClaimWithoutEvidenceIsNotVerified(t *testing.T) {
	p := testPage(t, appOrigin)
	p.caps = []Capture{{M: "GET", U: "http://x/api/list", S: 200}}
	verifyOf(t, p, "the list loads", "capture GET /api/list")
	if _, err := askPage(t, p, "verify", map[string]any{"claim": "the filter works"}); err != nil {
		t.Fatal(err)
	}
	out, err := askPage(t, p, "verify", map[string]any{"report": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1. OBSERVADO the list loads", "2. NO VERIFICADO the filter works"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	askPage(t, p, "verify", map[string]any{"clear": true})
	if out, _ := askPage(t, p, "verify", map[string]any{"report": true}); !strings.Contains(out, "no claims") {
		t.Errorf("clear must empty the table: %q", out)
	}
}
