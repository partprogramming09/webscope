package page

import (
	"net/http"
	"strings"
	"testing"
)

// The helper fails to load for different reasons; each needs a different next step, so the answer must tell them apart (real Chrome).

func TestBootUnderAScriptPolicySaysThePolicyBlockedItAndDoesNotSendTheModelBackToLoad(t *testing.T) {
	out, _ := runStepsTool(t, func(p *Tool, load string) []string {
		state, err := askPage(t, p, "state", nil)
		if err != nil {
			t.Fatal(err)
		}
		return []string{load, state}
	}, func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Security-Policy", "script-src 'unsafe-inline' 'unsafe-eval'")
		return false
	})
	got := out[1]
	if !strings.HasPrefix(got, "Failed: the page's security policy") || !strings.Contains(got, "script-src") {
		t.Fatalf("a CSP block must be named with its directive: %q", got)
	}
	if strings.Contains(got, "Next: page(load)") || !strings.Contains(got, "do not repeat page(load)") {
		t.Errorf("page(load) loads the same script and fails the same way: %q", got)
	}
	if !strings.Contains(got, "Next: ") {
		t.Errorf("no next step: %q", got)
	}
}

func TestBootWithNoListenerSaysTheListenerDidNotAnswerAndToRunLoadAgain(t *testing.T) {
	out, _ := runStepsTool(t, func(p *Tool, load string) []string {
		dead, err := snippetBoot("state", nil, "http://127.0.0.1:1/page.js?t=x")
		if err != nil {
			t.Fatal(err)
		}
		return []string{load, "(delete window.__ws,'removed')", dead}
	}, nil)
	got := out[2]
	if !strings.HasPrefix(got, "Failed: the helper script did not arrive") || !strings.Contains(got, "listener") || !strings.Contains(got, "page(load)") || !strings.Contains(got, "code(doctor)") {
		t.Fatalf("a dead listener must say so, point to page(load) and code(doctor): %q", got)
	}
	if strings.Contains(got, "security policy") {
		t.Errorf("no policy blocked it: %q", got)
	}
}

func TestBootIgnoresAPolicyViolationThatIsNotAboutScripts(t *testing.T) {
	out, _ := runStepsTool(t, func(p *Tool, load string) []string {
		dead, err := snippetBoot("state", nil, "http://127.0.0.1:1/page.js?t=x")
		if err != nil {
			t.Fatal(err)
		}
		// an unrelated violation (an image) is reported while the script tag is still loading
		img := "setTimeout(()=>document.dispatchEvent(new SecurityPolicyViolationEvent('securitypolicyviolation',{violatedDirective:'img-src',effectiveDirective:'img-src'})),0)"
		return []string{load, "(delete window.__ws,'removed')", "(" + img + "," + dead + ")"}
	}, nil)
	if got := out[2]; strings.Contains(got, "security policy") || !strings.Contains(got, "listener") {
		t.Fatalf("an img-src violation must not be blamed for the missing helper: %q", got)
	}
}
