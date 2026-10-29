package code

import (
	"errors"
	"fmt"
	"strings"
)

// flows is the evidence for the user stories of a front module in one answer: its screens with their actions (map walkthrough) and the endpoints, controllers and tables behind them (map_back).
func (c *Tool) flows(args map[string]any) (string, error) {
	module, _ := args["module"].(string)
	if module == "" {
		return "", errors.New("flows needs a module, a part of the path such as crm/quotes")
	}
	screens, err := c.mapFront(map[string]any{"module": module, "walkthrough": true})
	if err != nil {
		return "", err
	}
	head, body, _ := strings.Cut(screens, "\n")
	if !strings.HasPrefix(head, "Frontend of") {
		return screens, nil
	}
	out := []string{fmt.Sprintf("Flows of «%s»: evidence, not user stories. Write the stories from it and ask a person to confirm them: the code may describe a mistake as a feature.", echo(module)), "## screens", body}
	back, err := c.mapBack(map[string]any{"query": module})
	if err != nil {
		return capOut(strings.Join(append(out, "## endpoints", "not available: "+err.Error()), "\n")), nil
	}
	lines := strings.Split(back, "\n")
	out = append(out, "## endpoints")
	for _, l := range lines[min(2, len(lines)):] {
		if strings.HasPrefix(l, "## front ") || strings.HasPrefix(l, "route ") || l == "## backend" {
			continue
		}
		out = append(out, l)
	}
	return capOut(strings.Join(out, "\n")), nil
}
