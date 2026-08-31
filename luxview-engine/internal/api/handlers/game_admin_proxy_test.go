package handlers

import (
	"strings"
	"testing"
)

func TestStripAdminPrefix(t *testing.T) {
	prefix := "/api/apps/abc/game-admin"
	cases := map[string]string{
		prefix:                               "/",
		prefix + "/":                         "/",
		prefix + "/_framework/blazor.web.js": "/_framework/blazor.web.js",
		prefix + "/_blazor":                  "/_blazor",
	}
	for in, want := range cases {
		if got := stripAdminPrefix(in, prefix); got != want {
			t.Fatalf("stripAdminPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRewriteAdminHTMLSetsBaseAndAssets(t *testing.T) {
	prefix := "/api/apps/abc/game-admin"
	in := `<base href="/" />
<link href="/_content/x.css" rel="stylesheet" />
<script src="/_framework/blazor.web.js"></script>
<script type="importmap">{"imports":{"./":"/"}}</script>`
	out := string(rewriteAdminHTML([]byte(in), prefix))
	if !strings.Contains(out, `<base href="`+prefix+`/" />`) {
		t.Fatalf("missing base href: %s", out)
	}
	if !strings.Contains(out, `href="`+prefix+`/_content/x.css"`) {
		t.Fatalf("css path not rewritten: %s", out)
	}
	if !strings.Contains(out, `src="`+prefix+`/_framework/blazor.web.js"`) {
		t.Fatalf("js path not rewritten: %s", out)
	}
	if !strings.Contains(out, `"./":"`+prefix+`/"`) {
		t.Fatalf("importmap root not rewritten: %s", out)
	}
}

func TestRewriteAdminHTMLSkipsProtocolRelative(t *testing.T) {
	prefix := "/api/apps/abc/game-admin"
	in := `<script src="//cdn.example/x.js"></script>`
	out := string(rewriteAdminHTML([]byte(in), prefix))
	if out != in {
		t.Fatalf("protocol-relative rewritten: %s", out)
	}
}

func TestRewriteAdminLocationValue(t *testing.T) {
	prefix := "/api/apps/abc/game-admin"
	got := rewriteAdminLocationValue("/login", prefix, "luxview-game-mu:18080")
	if got != prefix+"/login" {
		t.Fatalf("location = %q", got)
	}
	got = rewriteAdminLocationValue("http://luxview-game-mu:18080/Accounts", prefix, "luxview-game-mu:18080")
	if got != prefix+"/Accounts" {
		t.Fatalf("absolute location = %q", got)
	}
	got = rewriteAdminLocationValue(prefix+"/already", prefix, "luxview-game-mu:18080")
	if got != prefix+"/already" {
		t.Fatalf("already prefixed = %q", got)
	}
}

func TestRewriteCookiePath(t *testing.T) {
	prefix := "/api/apps/abc/game-admin"
	got := rewriteCookiePath("a=1; Path=/", prefix)
	if !strings.Contains(got, "Path="+prefix+"/") {
		t.Fatalf("cookie path = %q", got)
	}
	got = rewriteCookiePath("a=1", prefix)
	if !strings.Contains(got, "Path="+prefix+"/") {
		t.Fatalf("missing path = %q", got)
	}
}
