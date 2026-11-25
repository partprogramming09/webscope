package code

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/partprogramming09/webscope/internal/db"
)

// doctor checks the setup in one answer: each variable (set, and what it points at), the database with SELECT 1, and the tenants it knows.
// It reads only: it creates nothing, writes nothing and never prints a password.
func (c *Tool) doctor(ctx context.Context) string {
	var lines, fixes []string
	add := func(name, status, detail string) {
		l := name + ": " + status
		if detail != "" {
			l += " · " + detail
		}
		lines = append(lines, l)
	}
	fix := func(s string) { fixes = append(fixes, s) }

	switch {
	case c.Src == nil || c.Src.URL == "":
		add("WEBSCOPE_DB_URL", "MISSING", "query, check and code fields need it")
		fix("set WEBSCOPE_DB_URL to the read-only database (mysql://user:password@host:3306/db)")
	default:
		if err := c.Src.Ping(ctx); err != nil {
			msg := db.Redact(err.Error(), db.Password(c.Src.URL))
			kind := map[string]string{"db_env": "the URL is malformed", "db_conn": "the database is not reachable at that address", "db_perm": "the user or password was refused"}[db.ConnKind(msg)]
			if kind == "" {
				kind = msg
			}
			add("WEBSCOPE_DB_URL", "FAIL", kind)
			fix("fix the connection: " + kind)
		} else {
			add("WEBSCOPE_DB_URL", "ok", "SELECT 1 answered"+tenantNote(ctx, c.Src))
		}
	}

	appURL := os.Getenv("WEBSCOPE_APP_URL")
	switch {
	case appURL == "":
		add("WEBSCOPE_APP_URL", "MISSING", "page needs the address of the app")
		fix("set WEBSCOPE_APP_URL to the app's address, e.g. http://localhost:5173")
	default:
		if bad := badAddress(appURL); bad != "" {
			add("WEBSCOPE_APP_URL", "FAIL", bad)
			fix("write each address of WEBSCOPE_APP_URL as scheme://host[:port], separated by commas")
		} else {
			add("WEBSCOPE_APP_URL", "ok", appURL)
		}
	}

	folder := func(name, path, needs string, required bool) {
		switch fi, err := os.Stat(path); {
		case path == "" && required:
			add(name, "not set", needs)
		case path == "":
			add(name, "not set", "optional, "+needs)
		case err != nil || !fi.IsDir():
			add(name, "FAIL", path+" is not a folder")
			fix("point " + name + " at an existing folder")
		default:
			add(name, "ok", path)
		}
	}
	folder("WEBSCOPE_BACKEND", c.Backend, "needs it: code consumers, map_back, contract, legacy columns", true)
	folder("WEBSCOPE_FRONT", c.Front, "needs it: code map, contract, flows", true)
	folder("WEBSCOPE_LEGACY", c.Legacy, "needs it: code legacy", true)
	for _, u := range strings.Split(os.Getenv("WEBSCOPE_UPLOADS"), ",") {
		if u = strings.TrimSpace(u); u != "" {
			folder("WEBSCOPE_UPLOADS", u, "", false)
		}
	}
	if d := os.Getenv("WEBSCOPE_DOWNLOADS"); d != "" {
		folder("WEBSCOPE_DOWNLOADS", d, "", false)
	}
	if l := os.Getenv("WEBSCOPE_LOG"); l != "" {
		if fi, err := os.Stat(l); err != nil || fi.IsDir() {
			add("WEBSCOPE_LOG", "FAIL", l+" is not a file")
			fix("point WEBSCOPE_LOG at the backend log file")
		} else {
			add("WEBSCOPE_LOG", "ok", l)
		}
	}

	out := strings.Join(lines, "\n")
	if len(fixes) == 0 {
		return out + "\nReady: nothing to fix. (A variable marked not set only disables the actions listed next to it.)"
	}
	return out + "\nNext: " + strings.Join(fixes, "; ") + ". Then restart the server and call code(doctor) again."
}

// badAddress says what is wrong with a comma-separated list of app addresses, or "" when each is scheme://host.
func badAddress(list string) string {
	for _, a := range strings.Split(list, ",") {
		a = strings.TrimSpace(a)
		u, err := url.Parse(a)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Sprintf("%q is not scheme://host (http or https)", a)
		}
	}
	return ""
}

// tenantNote lists up to 4 host names of the central database (Hyn Tenancy), or "" when there are none or it cannot tell.
func tenantNote(ctx context.Context, s *db.Source) string {
	if names := s.TenantNames(ctx); len(names) > 0 {
		return "; tenants: " + strings.Join(names, ", ")
	}
	return ""
}
