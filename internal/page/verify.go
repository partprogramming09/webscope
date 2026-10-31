package page

import (
	"fmt"
	"strings"
)

const (
	maxClaims   = 50
	maxClaimLen = 200
)

type claim struct{ state, text, evidence string }

// verify keeps the claims of a report with the evidence behind each; the server, not the caller, decides the state.
func (p *Tool) verify(args map[string]any) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, _ := args["clear"].(bool); v {
		p.claims = nil
		return "claims cleared", nil
	}
	if v, _ := args["report"].(bool); v {
		if len(p.claims) == 0 {
			return "no claims yet. Next: page(verify, {claim, evidence}) for each thing the report will say.", nil
		}
		var b strings.Builder
		for i, c := range p.claims {
			fmt.Fprintf(&b, "%d. %s %s", i+1, c.state, c.text)
			if c.evidence != "" {
				fmt.Fprintf(&b, " (evidence: %s)", c.evidence)
			}
			b.WriteByte('\n')
		}
		return strings.TrimRight(b.String(), "\n"), nil
	}
	text, _ := args["claim"].(string)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("verify needs a claim, or {report:true} or {clear:true}")
	}
	if len(p.claims) >= maxClaims {
		return "", fmt.Errorf("%d claims is the most the table holds. Next: page(verify, {report:true}), then {clear:true}", maxClaims)
	}
	ev, _ := args["evidence"].(string)
	c := claim{text: clip(oneLine(text), maxClaimLen), evidence: clip(oneLine(ev), 80)}
	var why string
	c.state, why = p.stateOf(ev)
	p.claims = append(p.claims, c)
	out := fmt.Sprintf("%d. %s %s", len(p.claims), c.state, c.text)
	if why != "" {
		out += " (" + why + ")"
	}
	return out, nil
}

// stateOf grades one piece of evidence: OBSERVADO (a real action and its effect were seen), INFERIDO (seen, but through an interceptor) or
// NO VERIFICADO (no reference the server can find). The caller holds p.mu.
func (p *Tool) stateOf(ev string) (state, why string) {
	ev = strings.TrimSpace(ev)
	switch {
	case ev == "download":
		if p.dlLast != "" {
			return "OBSERVADO", ""
		}
		return "NO VERIFICADO", "no file was seen by download wait"
	case strings.HasPrefix(ev, "capture "):
		if !p.hasCapture(strings.TrimSpace(strings.TrimPrefix(ev, "capture "))) {
			return "NO VERIFICADO", "no captured request matches"
		}
		if p.tamperedEver {
			return "INFERIDO", "an interceptor was seen since page(load)"
		}
		return "OBSERVADO", ""
	}
	return "NO VERIFICADO", "evidence must be «capture <METHOD address>» or «download»"
}

// hasCapture says whether a captured request matches spec ("POST /api/quotes"). The caller holds p.mu.
func (p *Tool) hasCapture(spec string) bool {
	method, part, _ := strings.Cut(spec, " ")
	for _, c := range p.caps {
		if strings.EqualFold(c.M, method) && strings.Contains(c.U, strings.TrimSpace(part)) {
			return true
		}
	}
	return false
}
