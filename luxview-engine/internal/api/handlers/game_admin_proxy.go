package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

const adminProxyPrefix = "/api/apps/"

// ProxyAdminPanel reverse-proxies the OpenMU Blazor admin (loopback-only on the
// host) through the authenticated dashboard API so the owner never needs port 18080.
func (h *GameServerHandler) ProxyAdminPanel(w http.ResponseWriter, r *http.Request) {
	app, cfg, ok := h.loadGame(w, r)
	if !ok {
		return
	}
	upstream, ok := service.AdminPanelURL(app.Subdomain, cfg)
	if !ok {
		writeError(w, http.StatusNotFound, "este jogo não tem painel admin")
		return
	}
	target, err := url.Parse(upstream)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid admin upstream")
		return
	}
	prefix := adminPanelPrefix(app.ID.String())
	if ticket := r.URL.Query().Get("ticket"); ticket != "" && r.Method == http.MethodGet {
		setAdminPanelCookie(w, r, ticket, prefix)
		if r.URL.Path == prefix || r.URL.Path == prefix+"/" {
			q := r.URL.Query()
			q.Del("ticket")
			loc := prefix + "/"
			if enc := q.Encode(); enc != "" {
				loc += "?" + enc
			}
			http.Redirect(w, r, loc, http.StatusFound)
			return
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = 100 * time.Millisecond
	proxy.Director = adminProxyDirector(target, prefix, r.Host)
	proxy.ModifyResponse = func(resp *http.Response) error {
		return rewriteAdminResponse(resp, prefix, target.Host)
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		log := logger.With("game-admin")
		log.Warn().Err(err).Str("app", app.Subdomain).Msg("admin panel unreachable")
		http.Error(rw, "Painel admin indisponível. Confira se o servidor está online.", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func adminPanelPrefix(appID string) string {
	return adminProxyPrefix + appID + "/game-admin"
}

func setAdminPanelCookie(w http.ResponseWriter, r *http.Request, ticket, prefix string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AdminPanelCookie,
		Value:    ticket,
		Path:     prefix,
		MaxAge:   int((8 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func adminProxyDirector(target *url.URL, prefix, origHost string) func(*http.Request) {
	return func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = stripAdminPrefix(req.URL.Path, prefix)
		req.Host = origHost
		req.Header.Set("X-Forwarded-Host", origHost)
		req.Header.Set("X-Forwarded-Prefix", prefix)
		if req.Header.Get("X-Forwarded-Proto") == "" {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		q := req.URL.Query()
		q.Del("ticket")
		req.URL.RawQuery = q.Encode()
		req.Header.Del("Accept-Encoding")
	}
}

func stripAdminPrefix(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return "/"
	}
	if !strings.HasPrefix(rest, "/") {
		return "/" + rest
	}
	return rest
}

func rewriteAdminResponse(resp *http.Response, prefix, upstreamHost string) error {
	if resp.StatusCode == http.StatusSwitchingProtocols {
		return nil
	}
	rewriteAdminLocation(resp, prefix, upstreamHost)
	rewriteAdminCookies(resp, prefix)
	ct := resp.Header.Get("Content-Type")
	if !shouldRewriteAdminBody(ct) {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	if err != nil {
		return err
	}
	out := rewriteAdminHTML(body, prefix)
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", int64String(len(out)))
	resp.Header.Del("Content-Encoding")
	return nil
}

func shouldRewriteAdminBody(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "text/html")
}

func rewriteAdminLocation(resp *http.Response, prefix, upstreamHost string) {
	loc := resp.Header.Get("Location")
	if loc == "" {
		return
	}
	resp.Header.Set("Location", rewriteAdminLocationValue(loc, prefix, upstreamHost))
}

func rewriteAdminLocationValue(loc, prefix, upstreamHost string) string {
	if strings.HasPrefix(loc, "http://"+upstreamHost) {
		return prefix + strings.TrimPrefix(loc, "http://"+upstreamHost)
	}
	if strings.HasPrefix(loc, "https://"+upstreamHost) {
		return prefix + strings.TrimPrefix(loc, "https://"+upstreamHost)
	}
	if strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, prefix) {
		return prefix + loc
	}
	return loc
}

func rewriteAdminCookies(resp *http.Response, prefix string) {
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	resp.Header.Del("Set-Cookie")
	for _, c := range cookies {
		resp.Header.Add("Set-Cookie", rewriteCookiePath(c, prefix))
	}
}

func rewriteCookiePath(setCookie, prefix string) string {
	parts := strings.Split(setCookie, ";")
	found := false
	for i, p := range parts {
		trim := strings.TrimSpace(p)
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "path=") {
			val := strings.TrimSpace(trim[len("path="):])
			if val == "/" || val == "" || !strings.HasPrefix(val, prefix) {
				parts[i] = " Path=" + prefix + "/"
			}
			found = true
		}
	}
	if !found {
		parts = append(parts, " Path="+prefix+"/")
	}
	return strings.Join(parts, ";")
}

func rewriteAdminHTML(body []byte, prefix string) []byte {
	s := string(body)
	s = strings.ReplaceAll(s, `<base href="/" />`, `<base href="`+prefix+`/" />`)
	s = strings.ReplaceAll(s, `<base href="/">`, `<base href="`+prefix+`/">`)
	s = rewriteCSSURL(s, prefix)
	s = prefixQuotedRootPaths(s, '"', prefix)
	s = prefixQuotedRootPaths(s, '\'', prefix)
	return []byte(s)
}

func prefixQuotedRootPaths(s string, q byte, prefix string) string {
	needle := string([]byte{q, '/'})
	var b strings.Builder
	rest := s
	for {
		i := strings.Index(rest, needle)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:i])
		if i+2 < len(rest) && rest[i+2] == '/' {
			b.WriteString(needle)
			rest = rest[i+2:]
			continue
		}
		if strings.HasPrefix(rest[i+1:], prefix) {
			b.WriteString(needle)
			rest = rest[i+2:]
			continue
		}
		b.WriteString(string(q) + prefix + "/")
		rest = rest[i+2:]
	}
}

func rewriteCSSURL(s, prefix string) string {
	var b strings.Builder
	rest := s
	for {
		i := strings.Index(rest, "url(/")
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:i])
		if i+5 < len(rest) && rest[i+5] == '/' {
			b.WriteString("url(/")
			rest = rest[i+5:]
			continue
		}
		b.WriteString("url(" + prefix + "/")
		rest = rest[i+5:]
	}
}

func int64String(n int) string {
	return strconv.Itoa(n)
}
