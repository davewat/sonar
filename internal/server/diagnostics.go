package server

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"
)

// DiagnosticData is everything rendered onto the diagnostic page for a
// single request.
type DiagnosticData struct {
	Timestamp string

	ClientIP   string
	ClientPort string

	Method string
	Path   string
	Host   string
	Proto  string

	UserAgent    string
	BrowserGuess string
	OSGuess      string

	TLS *TLSInfo

	Headers []HeaderPair

	GeoIP *GeoIPInfo

	ProcessingTime   time.Duration
	ProcessingTimeMs float64
	GaugeWidthPx     float64
}

// TLSInfo summarizes the negotiated TLS connection, when present.
type TLSInfo struct {
	Version     string
	CipherSuite string
	ALPN        string
}

// HeaderPair is one row of the request-headers table.
type HeaderPair struct {
	Name  string
	Value string
}

// interestingHeaders is an allowlist of headers worth showing. We
// deliberately don't dump the full header set: it keeps the UI readable and
// limits how much attacker-controlled data gets reflected onto the page.
var interestingHeaders = []string{
	"Accept",
	"Accept-Language",
	"Accept-Encoding",
	"Referer",
	"Origin",
	"Connection",
	"Cache-Control",
	"DNT",
	"Sec-Fetch-Site",
	"Sec-Fetch-Mode",
	"Sec-Fetch-Dest",
	"Sec-Ch-Ua",
	"Sec-Ch-Ua-Platform",
	"X-Forwarded-For",
}

// gatherDiagnostics builds a DiagnosticData from the request, up to (but not
// including) GeoIP and timing, which the handler fills in afterward.
func gatherDiagnostics(r *http.Request) DiagnosticData {
	clientIP, clientPort := splitClientAddr(r.RemoteAddr)
	browser, os := guessBrowserAndOS(r.UserAgent())

	data := DiagnosticData{
		Timestamp:    time.Now().UTC().Format(time.RFC1123),
		ClientIP:     clientIP,
		ClientPort:   clientPort,
		Method:       r.Method,
		Path:         r.URL.Path,
		Host:         r.Host,
		Proto:        r.Proto,
		UserAgent:    r.UserAgent(),
		BrowserGuess: browser,
		OSGuess:      os,
		TLS:          tlsInfoFromRequest(r),
		Headers:      collectHeaders(r),
	}
	return data
}

func splitClientAddr(remoteAddr string) (ip, port string) {
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr, ""
	}
	return host, port
}

func tlsInfoFromRequest(r *http.Request) *TLSInfo {
	if r.TLS == nil {
		return nil
	}
	return &TLSInfo{
		Version:     tls.VersionName(r.TLS.Version),
		CipherSuite: tls.CipherSuiteName(r.TLS.CipherSuite),
		ALPN:        r.TLS.NegotiatedProtocol,
	}
}

func collectHeaders(r *http.Request) []HeaderPair {
	var headers []HeaderPair
	for _, name := range interestingHeaders {
		if v := r.Header.Get(name); v != "" {
			headers = append(headers, HeaderPair{Name: name, Value: v})
		}
	}
	return headers
}

// guessBrowserAndOS is a small, deliberately approximate heuristic — not a
// real user-agent parser. Order matters: Edge/Opera UAs also contain
// "Chrome/", and iOS UAs contain the literal substring "Mac OS X" inside
// "like Mac OS X", so both must be checked before their broader relatives.
func guessBrowserAndOS(ua string) (browser, os string) {
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	default:
		browser = "Unknown"
	}

	switch {
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "Mac OS X"):
		os = "macOS"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	default:
		os = "Unknown"
	}
	return browser, os
}

// gaugeWidthPx maps a processing duration onto a 0-200px SVG bar width. The
// scale differs by mode because the dynamic range differs wildly: GeoIP
// lookups (network-bound, up to the ~2s timeout) dwarf pure in-memory
// header gathering (sub-millisecond to a few ms).
func gaugeWidthPx(d time.Duration, geoipEnabled bool) float64 {
	const svgWidth = 200.0
	maxMs := 20.0
	if geoipEnabled {
		maxMs = 2000.0
	}

	pct := (float64(d) / float64(time.Millisecond) / maxMs) * 100
	if pct > 100 {
		pct = 100
	}
	return (pct / 100) * svgWidth
}
