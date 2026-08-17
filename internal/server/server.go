// Package server implements the sonar diagnostic HTTP server: request
// handling, in-memory TLS certificate generation, and the diagnostic page
// template.
package server

import (
	"log"
	"net/http"
	"time"
)

// New builds the http.Server that serves the diagnostic page. Timeouts
// are set deliberately: this is an internet-facing, unauthenticated tool, so
// ReadHeaderTimeout is the primary defense against slowloris-style
// connections that dribble in headers one byte at a time. Note that
// ReadTimeout/WriteTimeout are measured from connection accept (including
// the TLS handshake on HTTPS listeners), not from header-read completion —
// that's intentional, not a bug to "fix" into per-handler timeouts.
func New(geoipEnabled bool) *http.Server {
	return &http.Server{
		Handler:           diagnosticHandler(geoipEnabled),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// diagnosticHandler responds to every request, regardless of method or
// path, with the same diagnostic page.
func diagnosticHandler(geoipEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		data := gatherDiagnostics(r)

		if geoipEnabled {
			info, err := lookupGeoIP(r.Context(), data.ClientIP)
			if err != nil {
				log.Printf("geoip lookup failed for %s: %v", data.ClientIP, err)
			} else {
				data.GeoIP = info
			}
		}

		// "Processing time" deliberately covers everything up to this point
		// (header/IP gathering plus the optional GeoIP call) and stops
		// before template execution — the page can't include the time it
		// takes to render itself.
		data.ProcessingTime = time.Since(start)
		data.ProcessingTimeMs = float64(data.ProcessingTime) / float64(time.Millisecond)
		data.GaugeWidthPx = gaugeWidthPx(data.ProcessingTime, geoipEnabled)

		renderIndex(w, data)
	}
}
