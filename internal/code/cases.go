package code

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/partprogramming09/webscope/internal/input"
)

var (
	actionLine = regexp.MustCompile(`^action \d+ (.*?)(?: if (.*?))?(?: «(.*)»)?$`)
	canRe      = regexp.MustCompile(`can\('([^']+)'(?:\s*,\s*'([^']+)')?`)
	apiLine    = regexp.MustCompile(`^api (GET|POST|PUT|PATCH|DELETE) (\S+) ->`)
)

const casesLegend = "Verify: happy = page payloads, then check the saved ZZ_ row · permission = the action is not offered and its endpoint is refused · required empty = page state shows the error, page payloads no request · status = each one is shown and its actions follow; query count by status · http error = an error is shown and the count is unchanged · empty = page state shows 0 rows and the empty message."

// cases turns the flows of a module into a list of test cases to run with page and to verify with check, query or page. It runs nothing.
func (c *Tool) cases(args map[string]any) (string, error) {
	module, _ := args["module"].(string)
	if module == "" {
		return "", errors.New("cases needs a module, a part of the path such as crm/quotes")
	}
	screens, endpoints, found, err := c.flowParts(args)
	if err != nil || !found {
		return screens, err
	}
	return casesFrom(module, screens, endpoints), nil
}

// readable is false for a label that is a translation key or a template, which no screen shows as written.
func readable(label string) bool {
	return label != "" && !strings.Contains(label, "$t(") && !strings.Contains(label, "{{")
}

// casesFrom builds the table from the screens and endpoints of flows: each case once, in the order the module lists them.
func casesFrom(module, screens, endpoints string) string {
	var rows [][3]string
	have := map[string]bool{}
	add := func(flow, kase, run string) {
		if k := flow + "\x00" + kase; !have[k] {
			have[k] = true
			rows = append(rows, [3]string{flow, kase, run})
		}
	}
	screen := ""
	for _, l := range strings.Split(screens, "\n") {
		switch {
		case strings.HasPrefix(l, "### "):
			screen, _, _ = strings.Cut(strings.TrimPrefix(l, "### "), " [")
			screen = path.Base(screen)
		case strings.HasPrefix(l, "statuses "):
			list, _, _ := strings.Cut(strings.TrimPrefix(l, "statuses "), "  ")
			add(echo(module), "status: "+strings.Join(strings.Fields(list), ", "), "state on a row of each")
		case strings.HasPrefix(l, "action "):
			m := actionLine.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			kase, run := "happy: "+m[1], "click the button that calls it"
			if readable(m[3]) {
				kase, run = kase+" «"+m[3]+"»", "click «"+m[3]+"»"
			}
			add(screen, kase, run)
			if p := canRe.FindStringSubmatch(m[2]); p != nil {
				add(screen, "permission: "+strings.TrimSpace(p[1]+" "+p[2]), "state as a user without it")
			}
		case strings.HasPrefix(l, "permission "):
			if p := canRe.FindStringSubmatch(l); p != nil {
				add(screen, "permission: "+strings.TrimSpace(p[1]+" "+p[2]), "state as a user without it")
			}
		case strings.HasPrefix(l, "fields "):
			_, list, _ := strings.Cut(strings.TrimPrefix(l, "fields "), " ")
			var req []string
			for _, f := range strings.Split(list, ", ") {
				if name, ok := strings.CutSuffix(f, "*"); ok && !slicesContains(req, name) {
					req = append(req, name)
				}
			}
			if len(req) > 0 {
				add(screen, "required empty: "+strings.Join(req, ", "), "fill all but one of them at a time, save")
			}
		}
	}
	for _, l := range strings.Split(endpoints, "\n") {
		if m := apiLine.FindStringSubmatch(l); m != nil {
			flow := m[1] + " " + m[2]
			if m[1] != "GET" {
				match, _, _ := strings.Cut(m[2], "{") // an id in the address is a {} here and a number in the request
				add(flow, "http error", `simulate {match: "`+match+`", status: 500}, repeat the action`)
			} else if !strings.Contains(m[2], "{") {
				add(flow, "empty", "filter that matches nothing")
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cases of «%s»: a list to run, not a result. Run each with page; writes only with the user's approval, on ZZ_ rows.\n%s\nid\tflow\tcase\trun with", echo(module), casesLegend)
	shown := 0
	for i, r := range rows {
		line := fmt.Sprintf("\nC%d\t%s\t%s\t%s", i+1, r[0], r[1], r[2])
		if b.Len()+len(line) > input.MaxAnswer-60 { // room for the count line
			break
		}
		b.WriteString(line)
		shown++
	}
	fmt.Fprintf(&b, "\n(%d cases", len(rows))
	if shown < len(rows) {
		fmt.Fprintf(&b, ", %d shown: ask for a narrower module", shown)
	}
	b.WriteString(")")
	return b.String()
}
