package code

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/partprogramming09/webscope/internal/input"
)

// flowParts is the evidence for the user stories of a front module, uncut: its screens with their actions (map walkthrough) and the endpoints,
// controllers and tables behind them (map_back). found is false when no module matches; then screens holds the message.
func (c *Tool) flowParts(args map[string]any) (screens, endpoints string, found bool, err error) {
	module, _ := args["module"].(string)
	if module == "" {
		return "", "", false, errors.New("flows needs a module, a part of the path such as crm/quotes")
	}
	all, err := c.mapFrontRaw(map[string]any{"module": module, "walkthrough": true})
	if err != nil {
		return "", "", false, err
	}
	head, body, _ := strings.Cut(all, "\n")
	if !strings.HasPrefix(head, "Frontend of") {
		return all, "", false, nil
	}
	back, err := c.mapBack(map[string]any{"query": module, "uncut": true})
	if err != nil {
		return viewsFirst(body), "not available: " + err.Error(), true, nil
	}
	lines := strings.Split(back, "\n")
	var keep []string
	for _, l := range lines[min(2, len(lines)):] {
		if strings.HasPrefix(l, "## front ") || strings.HasPrefix(l, "route ") || l == "## backend" {
			continue
		}
		keep = append(keep, l)
	}
	return viewsFirst(body), strings.Join(keep, "\n"), true, nil
}

// cutLines keeps the lines of s that fit in n bytes and says how many were left out.
func cutLines(s string, n int, what string) string {
	if len(s) <= n {
		return s
	}
	cut := strings.LastIndex(s[:n], "\n")
	if cut < 0 {
		cut = 0
	}
	return s[:cut] + fmt.Sprintf("\n(… %d more lines of %s not shown: cut at %d bytes)", strings.Count(s[cut:], "\n"), what, n)
}

// flows is the evidence for the user stories of a front module in one answer; the screens and the endpoints each keep their share of the room.
func (c *Tool) flows(args map[string]any) (string, error) {
	screens, endpoints, found, err := c.flowParts(args)
	if err != nil || !found {
		return screens, err
	}
	module, _ := args["module"].(string)
	head := fmt.Sprintf("Flows of «%s»: evidence, not user stories. Write the stories from it and ask a person to confirm them: the code may describe a mistake as a feature.", echo(module))
	room := input.MaxAnswer - len(head) - 40
	screens = cutLines(screens, room*5/9, "screens")
	endpoints = cutLines(tablesFirst(endpoints), room-len(screens), "endpoints")
	return head + "\n## screens\n" + screens + "\n## endpoints\n" + endpoints, nil
}

// viewsFirst puts the screens under /views/ before the components they use, and the module-level lines (statuses) at the top, so a cut loses the parts first.
func viewsFirst(screens string) string {
	var head, views, rest, cur []string
	var curView bool
	flush := func() {
		if len(cur) == 0 {
			return
		}
		if curView {
			views = append(views, cur...)
		} else {
			rest = append(rest, cur...)
		}
		cur = nil
	}
	for _, l := range strings.Split(screens, "\n") {
		switch {
		case strings.HasPrefix(l, "### "):
			flush()
			curView = strings.Contains(l, "/views/")
			cur = []string{l}
		case cur == nil || strings.HasPrefix(l, "statuses "):
			head = append(head, l)
		default:
			cur = append(cur, l)
		}
	}
	flush()
	return strings.Join(append(append(head, views...), rest...), "\n")
}

var controllerPath = regexp.MustCompile(`  \S+/(\w+\.php:\d+)(?:  <- .*)?$`)

// tablesFirst puts the models with their table before the endpoints, and shortens each endpoint line to the controller file and line, so a cut loses endpoints last-first and never the tables.
func tablesFirst(endpoints string) string {
	var models, apis, rest []string
	for _, l := range strings.Split(endpoints, "\n") {
		switch {
		case strings.HasPrefix(l, "model "):
			models = append(models, l)
		case apiLine.MatchString(l):
			apis = append(apis, controllerPath.ReplaceAllString(l, "  $1"))
		default:
			rest = append(rest, l)
		}
	}
	return strings.Join(append(append(models, apis...), rest...), "\n")
}
