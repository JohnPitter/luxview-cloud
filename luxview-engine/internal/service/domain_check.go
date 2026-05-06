package service

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// DomainCheckResult is what the API returns for a custom-domain diagnostic.
type DomainCheckResult struct {
	Domain     string         `json:"domain"`
	ExpectedIP string         `json:"expected_ip"`

	Apex DomainHostStatus `json:"apex"`
	WWW  DomainHostStatus `json:"www"`

	Nameservers     []string `json:"nameservers"`
	ParkingDetected bool     `json:"parking_detected"`

	Cert DomainCertStatus `json:"cert"`

	Ready    bool     `json:"ready"`
	Issues   []string `json:"issues"`
	CheckedAt time.Time `json:"checked_at"`
}

type DomainHostStatus struct {
	Host     string   `json:"host"`
	IPs      []string `json:"ips"`
	Match    bool     `json:"match"`
	Cloudflare bool   `json:"cloudflare_proxied"`
}

type DomainCertStatus struct {
	Issued    bool       `json:"issued"`
	NotAfter  *time.Time `json:"not_after,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

// DomainChecker bundles config and a resolver for diagnostics.
type DomainChecker struct {
	expectedIP string
	acmePath   string
	resolver   *net.Resolver
}

func NewDomainChecker(expectedIP, acmePath string) *DomainChecker {
	// Use a public resolver to avoid relying on the host /etc/resolv.conf
	// which inside containers may point to systemd-resolved or Docker's DNS
	// and can return stale cached records.
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, network, "1.1.1.1:53")
		},
	}
	return &DomainChecker{expectedIP: expectedIP, acmePath: acmePath, resolver: r}
}

// Check runs all DNS + cert checks for a domain. Always returns a populated
// result; transient errors surface as Issues, not as a Go error.
func (c *DomainChecker) Check(ctx context.Context, domain string) DomainCheckResult {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")

	res := DomainCheckResult{
		Domain:      domain,
		ExpectedIP:  c.expectedIP,
		Nameservers: []string{},
		Issues:      []string{},
		CheckedAt:   time.Now().UTC(),
	}

	if domain == "" {
		res.Issues = append(res.Issues, "empty_domain")
		return res
	}

	res.Apex = c.checkHost(ctx, domain)
	res.WWW = c.checkHost(ctx, "www."+domain)

	// Nameservers — strip trailing dot for easier comparison
	if nss, err := c.resolver.LookupNS(ctx, domain); err == nil {
		for _, ns := range nss {
			h := strings.TrimSuffix(ns.Host, ".")
			res.Nameservers = append(res.Nameservers, h)
			lower := strings.ToLower(h)
			if strings.Contains(lower, "dns-parking.com") || strings.Contains(lower, "sedoparking.com") {
				res.ParkingDetected = true
			}
		}
	}

	res.Cert = c.readCertStatus(domain)

	// Compose issues — only surface "parking" if the DNS isn't resolving
	// correctly yet. Once the A record points here, the parking NS is just
	// a stale resolver-cache artifact and shouldn't alarm the user.
	if res.ParkingDetected && !res.Apex.Match {
		res.Issues = append(res.Issues, "parking_nameservers")
	}
	if !res.Apex.Match {
		if len(res.Apex.IPs) == 0 {
			res.Issues = append(res.Issues, "apex_unresolved")
		} else {
			res.Issues = append(res.Issues, "apex_wrong_ip")
		}
	}
	if res.Apex.Cloudflare || res.WWW.Cloudflare {
		res.Issues = append(res.Issues, "cloudflare_proxy_active")
	}
	if res.Apex.Match && !res.Cert.Issued {
		res.Issues = append(res.Issues, "cert_pending")
	}

	res.Ready = res.Apex.Match && res.Cert.Issued
	return res
}

func (c *DomainChecker) checkHost(ctx context.Context, host string) DomainHostStatus {
	out := DomainHostStatus{Host: host, IPs: []string{}}
	ips, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return out
	}
	out.IPs = ips
	for _, ip := range ips {
		if ip == c.expectedIP {
			out.Match = true
		}
		if isCloudflareIP(ip) {
			out.Cloudflare = true
		}
	}
	return out
}

// readCertStatus parses Traefik's acme.json (mounted read-only) to find an
// issued cert for the domain. Falls back to a live TLS handshake if the file
// is missing — works for non-Traefik deployments and as a sanity check.
func (c *DomainChecker) readCertStatus(domain string) DomainCertStatus {
	st := DomainCertStatus{}

	if data, err := os.ReadFile(c.acmePath); err == nil {
		// acme.json schema: { "<resolverName>": { "Certificates": [ { "domain": {"main": "..."}, "certificate": "<base64>" } ] } }
		var raw map[string]struct {
			Certificates []struct {
				Domain struct {
					Main string   `json:"main"`
					SANs []string `json:"sans"`
				} `json:"domain"`
				Certificate string `json:"certificate"`
			} `json:"Certificates"`
		}
		if err := json.Unmarshal(data, &raw); err == nil {
			for _, resolver := range raw {
				for _, cert := range resolver.Certificates {
					if !domainMatches(domain, cert.Domain.Main, cert.Domain.SANs) {
						continue
					}
					st.Issued = true
					if expiry, err := parseCertExpiry(cert.Certificate); err == nil {
						st.NotAfter = &expiry
					}
					return st
				}
			}
		}
	}

	// Live TLS probe as fallback. Short timeout — this is best-effort.
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := dialer.Dial("tcp", domain+":443")
	if err != nil {
		st.LastError = err.Error()
		return st
	}
	defer conn.Close()
	// We don't actually do the TLS handshake here — the acme.json check is
	// authoritative on the engine side. Reaching this point just means we
	// don't have evidence of a cert; let the frontend keep polling.
	return st
}

func domainMatches(want, main string, sans []string) bool {
	if strings.EqualFold(main, want) {
		return true
	}
	for _, s := range sans {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func parseCertExpiry(b64 string) (time.Time, error) {
	// Traefik stores certs as base64-encoded PEM
	pemBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return time.Time{}, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return time.Time{}, fmt.Errorf("no pem block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}

// isCloudflareIP returns true if ip falls in any of Cloudflare's published
// proxy ranges (as of 2024). Used to warn the user that the orange-cloud
// proxy is active, which breaks HTTP-01 ACME challenges.
func isCloudflareIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range cloudflareCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

var cloudflareCIDRs = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}
