package code

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyRejectsListsTheLinesThatStopARequest: with rejects, each file shows its throw, die, exit and error-status lines (comments left out), and
// without it the answer has no such section.
func TestLegacyRejectsListsTheLinesThatStopARequest(t *testing.T) {
	c := legacySetup(t)
	body := "<?php\n// throw new Exception('commented out');\nif ($n > 0) {\n  throw new Exception(\"No se puede borrar: tiene ventas\");\n}\nhttp_response_code(422);\nif (!$ok) die('sin permiso');\n"
	if err := os.WriteFile(filepath.Join(c.Legacy, "comercial/cotizadorX_new/server/borrar.php"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"path": "comercial/cotizadorX_new"}
	if out, err := code(t, c, "legacy", args); err != nil || strings.Contains(out, "rejections") {
		t.Errorf("without rejects there is no section: %v\n%s", err, out)
	}
	args["rejects"] = true
	out, err := code(t, c, "legacy", args)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## rejections", "server/borrar.php: 4: throw new Exception(\"No se puede borrar: tiene ventas\")", "6: http_response_code(422);", "7: if (!$ok) die('sin permiso');"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "commented out") || strings.Contains(out, "server/listar.php: ") {
		t.Errorf("a comment and a file with no rejection are left out:\n%s", out)
	}
}
