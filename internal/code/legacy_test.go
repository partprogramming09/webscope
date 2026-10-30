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
	c := legacySetup(t)
	if err := os.MkdirAll(filepath.Join(c.Backend, "app/Modules/Shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := "<?php\nDB::table('app_clientes')->get();\n// tm_users is read by the login\n$x = 'tm_users';\n"
	if err := os.WriteFile(filepath.Join(c.Backend, "app/Modules/Shared/Other.php"), []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = code(t, c, "legacy", map[string]any{"path": "comercial/cotizadorX_new", "module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## against «crm/quotes»", "only in legacy and named elsewhere in the backend: app_clientes (1 file), tm_users (1 file)", "only in legacy and not named in the backend: app_cotiza_infogral, app_cotiza_notas, tm_huerfana", "only in new: -", "both: app_cotizaciones"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
}

func TestLegacyListsTheColumnsItWritesThatTheBackendNeverNames(t *testing.T) {
	c := legacySetup(t)
	put := func(base, rel, body string) {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(c.Legacy, "comercial/cotizadorX_new/server/estado.php", "<?php\n$db->query(\"INSERT INTO app_cotizaciones (CODIGO, `chk_flete`, ESTADO,\n   {$extra}) VALUES (1,2,3)\");\n$db->query(\"UPDATE app_cotizaciones SET MOTIVO_RECHAZO = 'x', USUARIO=1 WHERE id=3\");\n// UPDATE app_cotizaciones SET comentada = 1\n$sql = \"INSERT  INTO  app_cotizaciones  (extra_col,\" .\n  \" otra_col, fecha_ingreso) VALUES (1,2,3)\";\n$sql = \"UPDATE app_cotizaciones SET \" .\n  \"campo_union = 1, \" .\n  \"fecha_ingreso = 2 WHERE id = 1\";\n$db->query(\"UPDATE app_clientes SET solo_legado = 1\");\n")
	put(c.Backend, "app/Modules/Crm/Quotes/Services/QuoteWriter.php", "<?php\n$q->update(['codigo' => 1, 'motivo_rechazo' => 'x']);\n$q->estado = 'A';\n$f = ['fecha_ingreso' => 1];\n$other = 'chk_flete_nuevo';\n")
	put(c.Backend, "app/Modules/Shared/Other.php", "<?php\n$x = ['usuario' => 1];\n")
	out, err := code(t, c, "legacy", map[string]any{"path": "comercial/cotizadorX_new", "module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	const outside = "named in the backend but not in the module's folders"
	_, cols, ok := strings.Cut(out, "## columns the legacy writes on shared tables")
	if !ok {
		t.Fatalf("the section is missing:\n%s", out)
	}
	nowhere, elsewhere, _ := strings.Cut(cols, outside)
	if !strings.Contains(cols, "module folders: app/Modules/Crm/Quotes/") {
		t.Errorf("the module folders must be named:\n%s", cols)
	}
	for _, want := range []string{"app_cotizaciones: ", "chk_flete", "extra_col", "otra_col", "campo_union"} {
		if !strings.Contains(nowhere, want) {
			t.Errorf("%q is written by the legacy and named nowhere in the backend:\n%s", want, nowhere)
		}
	}
	for _, not := range []string{"usuario", "codigo", "estado", "motivo_rechazo", "comentada", "fecha_ingreso", "{"} {
		if strings.Contains(nowhere, not) {
			t.Errorf("%q must not be in the «named nowhere» group (named in the backend, a comment or a variable):\n%s", not, nowhere)
		}
	}
	if !strings.Contains(elsewhere, "usuario") {
		t.Errorf("usuario is named only outside the module's folders:\n%s", elsewhere)
	}
	for _, not := range []string{"codigo", "estado", "motivo_rechazo", "chk_flete", "fecha_ingreso"} {
		if strings.Contains(elsewhere, not) {
			t.Errorf("%q is not a column named only outside the module:\n%s", not, elsewhere)
		}
	}
	if strings.Contains(out, "solo_legado") {
		t.Errorf("a table only the legacy has is not compared by column:\n%s", out)
	}
}

func TestLegacyComparisonWithoutABackendKeepsOneList(t *testing.T) {
	c := legacySetup(t)
	c.Backend = ""
	out, err := code(t, c, "legacy", map[string]any{"path": "comercial/cotizadorX_new", "module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not compared") {
		t.Errorf("the new module cannot be read without WEBSCOPE_BACKEND, and the answer must say so:\n%s", out)
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
	if len(out) > 8200 || !strings.Contains(out, "only in legacy and not named in the backend: ") || !strings.Contains(out, "(+") || !strings.Contains(out, "more lines of endpoints not shown") {
		t.Errorf("%d bytes: the comparison must survive and the cut must be said:\n...%s", len(out), out[max(0, len(out)-300):])
	}
	if strings.Count(out, "no caller found") != 1 || strings.Contains(out, "number_399") {
		t.Errorf("the files with no caller are one short line, not hundreds of names:\n...%s", out[max(0, len(out)-500):])
	}
}
