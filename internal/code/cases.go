package code

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const maxCases = 120

var (
	actionLine = regexp.MustCompile(`^action \d+ (.*?)(?: if (.*?))?(?: «(.*)»)?$`)
	canRe      = regexp.MustCompile(`can\('([^']+)'`)
	apiLine    = regexp.MustCompile(`^api (GET|POST|PUT|PATCH|DELETE) (\S+) ->`)
)

// cases turns the flows of a module into a list of test cases to run, each with what runs it (page) and what checks it (page, check, query). It runs nothing.
func (c *Tool) cases(args map[string]any) (string, error) {
	module, _ := args["module"].(string)
	if module == "" {
		return "", errors.New("cases needs a module, a part of the path such as crm/quotes")
	}
	evidence, err := c.flows(args)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(evidence, "Flows of") {
		return evidence, nil
	}
	var rows [][4]string
	add := func(flow, kase, run, verify string) { rows = append(rows, [4]string{flow, kase, run, verify}) }
	screen, seen := "", map[string]bool{}
	once := func(k string) bool { d := seen[k]; seen[k] = true; return !d }
	for _, l := range strings.Split(evidence, "\n") {
		switch {
		case strings.HasPrefix(l, "### "):
			screen, _, _ = strings.Cut(strings.TrimPrefix(l, "### "), " [")
			screen = path.Base(screen)
		case strings.HasPrefix(l, "statuses "):
			list, _, _ := strings.Cut(strings.TrimPrefix(l, "statuses "), "  ")
			add(echo(module), "status: "+strings.Join(strings.Fields(list), ", "), "on a row of each status, page state", "each status is shown and its actions follow it; query count by status")
		case strings.HasPrefix(l, "action "):
			m := actionLine.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			label := m[1]
			run := "page click {selector} on the button that calls " + m[1]
			if m[3] != "" {
				label += " «" + m[3] + "»"
				run = "page click «" + m[3] + "»"
			}
			add(screen, "happy: "+label, run, "page payloads, then check against the saved row (ZZ_ rows only)")
			if p := canRe.FindStringSubmatch(m[2]); p != nil && once(screen+p[1]) {
				add(screen, "permission: "+p[1], "page state as a user without it", "the action is not offered and its endpoint is refused")
			}
		case strings.HasPrefix(l, "permission "):
			if p := canRe.FindStringSubmatch(l); p != nil && once(screen+p[1]) {
				add(screen, "permission: "+p[1], "page state as a user without it", "what it guards is not shown")
			}
		case strings.HasPrefix(l, "fields "):
			_, list, _ := strings.Cut(strings.TrimPrefix(l, "fields "), " ")
			for _, f := range strings.Split(list, ", ") {
				if name, ok := strings.CutSuffix(f, "*"); ok {
					add(screen, "required empty: "+name, "page fill every field but "+name+", then page click the save button", "page state shows the error under it; page payloads shows no request")
				}
			}
		case apiLine.MatchString(l):
			m := apiLine.FindStringSubmatch(l)
			flow := m[1] + " " + m[2]
			if m[1] != "GET" {
				match, _, _ := strings.Cut(m[2], "{") // an id in the address is a {} here and a number in the request
				add(flow, "http error", "page simulate {match: \""+match+"\", status: 500}, then repeat the action", "an error is shown and query count of the table is unchanged")
			} else if !strings.Contains(m[2], "{") {
				add(flow, "empty", "page fill a filter that matches nothing, then click search", "page state shows 0 rows and the empty message, no error")
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cases of «%s»: a list to run, not a result. Write only with the user's approval, on ZZ_ rows. Run them with page; confirm with check and query.\nid\tflow\tcase\trun with\tverify with", echo(module))
	shown := rows
	if len(shown) > maxCases {
		shown = shown[:maxCases]
	}
	for i, r := range shown {
		fmt.Fprintf(&b, "\nC%d\t%s\t%s\t%s\t%s", i+1, r[0], r[1], r[2], r[3])
	}
	fmt.Fprintf(&b, "\n(%d cases", len(rows))
	if len(shown) < len(rows) {
		fmt.Fprintf(&b, ", %d shown: ask for a narrower module", len(shown))
	}
	b.WriteString(")")
	return capOut(b.String()), nil
}
