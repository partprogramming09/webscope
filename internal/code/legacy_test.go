package code

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacySetup is a small legacy PHP tree: endpoints under a module folder called by jQuery from another folder, and the traps: a .env, a node_modules and a link out of the root.
func legacySetup(t *testing.T) *Tool {
	t.Helper()
	c := mapSetup(t)
	root, outside := t.TempDir(), t.TempDir()
	write := func(base, rel, body string) {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := "comercial/cotizadorX_new/"
	write(root, m+"server/grabar.php", "<?php\n// update the user list from the form\n$sql = \"INSERT INTO app_cotizaciones (a) VALUES (1) ON DUPLICATE KEY UPDATE valor = 2\";\n$db->query(\"UPDATE `app_cotiza_infogral` SET x=1\");\n$r = $db->query(\"SELECT * FROM app_clientes c\n  JOIN tm_users u ON 1\");\n$db->query(\"DELETE FROM app_cotiza_notas WHERE id=1\");\n$db->query(\"SELECT * FROM app_cotizaciones\");\n")
	write(root, m+"server/listar.php", "<?php\n$db->query('select * from app_cotizaciones');\n")
	write(root, m+"server/huerfano.php", "<?php\n$db->query('select * from tm_huerfana');\n")
	write(root, "js/cotizaciones/cotiza.js", "$.ajax({\n  url: 'cotizadorX_new/server/grabar.php',\n});\n$.post(\"cotizadorX_new/server/listar.php\", {});\n")
	write(root, m+"js/local.js", "$.get('server/listar.php');\n")
	write(root, "otro/server/listar.php", "<?php\n$db->query('select * from tm_otro_modulo');\n")
	write(root, "otro/js/o.js", "$.get('otro/server/listar.php');\n")
	write(root, ".env", "SECRET_IN_LEGACY_ENV=1\n")
	write(root, m+"server/node_modules/x.php", "<?php\n$db->query('select * from tm_secret_dep');\n")
	write(outside, "leak.php", "<?php\n$db->query('select * from tm_outside');\n")
	if err := os.Symlink(outside, filepath.Join(root, m+"linkdir")); err != nil {
		t.Fatal(err)
	}
	c.Legacy = root
	return c
}

func TestLegacyListsEndpointsWithTheirCallersAndTables(t *testing.T) {
	out, err := code(t, legacySetup(t), "legacy", map[string]any{"path": "comercial/cotizadorX_new"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Legacy of «comercial/cotizadorX_new»", "3 PHP files, 2 with a caller", "estimate",
		"server/grabar.php <- js/cotizaciones/cotiza.js:2", "W app_cotizaciones, app_cotiza_infogral, app_cotiza_notas", "R app_clientes, tm_users",
		"server/listar.php <- ", " (+1)", "no caller found: server/huerfano.php"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"tm_otro_modulo", "tm_secret_dep", "tm_outside", "SECRET", "W the", "R the", "form", "valor"} {
		if strings.Contains(out, not) {
			t.Errorf("must not contain %q:\n%s", not, out)
		}
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "server/grabar.php") && strings.Contains(l, "R app_cotizaciones") {
			t.Errorf("a table an endpoint writes and reads is listed once, as written:\n%s", l)
		}
	}
}

func TestLegacyComparesItsTablesWithThoseOfTheNewModule(t *testing.T) {
	out, err := code(t, legacySetup(t), "legacy", map[string]any{"path": "comercial/cotizadorX_new", "module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## against «crm/quotes»", "only in legacy: app_clientes, app_cotiza_infogral, app_cotiza_notas, tm_huerfana, tm_users", "only in new: -", "both: app_cotizaciones"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
}

func TestLegacyRefusesWhatItCannotReadAndSaysWhatItNeeds(t *testing.T) {
	c := legacySetup(t)
	for name, args := range map[string]map[string]any{
		"no path":       {},
		"out of root":   {"path": "../"},
		"absolute":      {"path": "/etc"},
		"missing":       {"path": "nope"},
		"inside a link": {"path": "comercial/cotizadorX_new/linkdir"},
	} {
		if _, err := code(t, c, "legacy", args); err == nil {
			t.Errorf("%s must fail", name)
		}
	}
	c.Legacy = ""
	if _, err := code(t, c, "legacy", map[string]any{"path": "x"}); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_LEGACY") {
		t.Errorf("want a failure that names WEBSCOPE_LEGACY: %v", err)
	}
}

func TestLegacyKeepsTheComparisonWhenThereAreManyEndpoints(t *testing.T) {
	c := legacySetup(t)
	dir := filepath.Join(c.Legacy, "comercial/cotizadorX_new/server")
	var calls strings.Builder
	for i := 0; i < 400; i++ {
		name := fmt.Sprintf("endpoint_with_a_long_name_number_%03d.php", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("<?php\n$db->query('select * from tm_t"+fmt.Sprint(i)+"');\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if i < 300 {
			calls.WriteString("$.get('cotizadorX_new/server/" + name + "');\n")
		}
	}
	if err := os.WriteFile(filepath.Join(c.Legacy, "js/cotizaciones/many.js"), []byte(calls.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "legacy", map[string]any{"path": "comercial/cotizadorX_new", "module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > 8200 || !strings.Contains(out, "only in legacy:") || !strings.Contains(out, "more lines of endpoints not shown") {
		t.Errorf("%d bytes: the comparison must survive and the cut must be said:\n...%s", len(out), out[max(0, len(out)-300):])
	}
	if strings.Count(out, "no caller found") != 1 || strings.Contains(out, "number_399") {
		t.Errorf("the files with no caller are one short line, not hundreds of names:\n...%s", out[max(0, len(out)-500):])
	}
}
