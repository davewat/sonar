package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/davewat/sonar/internal/server"
)

const description = `sonar is a lightweight diagnostic web server. It responds to every
HTTP request with a single page showing connection and request metadata
detected from your request: client address, TLS details, request headers,
a best-guess browser/OS, and server-side request-processing time. Useful
for verifying connectivity, testing TLS termination, and inspecting what
a server sees about an incoming client.

Ports below 1024 (including the default, 443) require root privileges or
CAP_NET_BIND_SERVICE on Linux/macOS. See README.md for details.`

func main() {
	var (
		port   = flag.Int("port", 443, "Port to listen on. Binding to ports below 1024 requires root or CAP_NET_BIND_SERVICE — see README.")
		useTLS = flag.Bool("tls", false, "Serve HTTPS using an in-memory, self-signed certificate generated at startup. Never written to disk. Off by default (plain HTTP).")
		geoip  = flag.Bool("geoip", false, "Enable outbound GeoIP lookups (ip-api.com) to show country/region/city/ISP for public client IPs. Off by default; adds a network dependency and a short per-request delay.")
	)

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, description)
		fmt.Fprintln(os.Stderr, "\nUsage:\n  sonar [flags]\n\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		reportListenError(err, *port)
		os.Exit(1)
	}

	srv := server.New(*geoip)

	if *useTLS {
		cert, err := server.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("sonar: failed to generate self-signed certificate: %v", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2", "http/1.1"},
		}

		log.Printf("sonar: serving HTTPS (self-signed cert) on port %d", *port)
		log.Printf("sonar: browsers will show an untrusted-certificate warning for this connection — that's expected")
		if *geoip {
			log.Printf("sonar: GeoIP lookups enabled (outbound calls to ip-api.com)")
		}
		err = srv.ServeTLS(ln, "", "")
	} else {
		log.Printf("sonar: serving HTTP on port %d", *port)
		if *geoip {
			log.Printf("sonar: GeoIP lookups enabled (outbound calls to ip-api.com)")
		}
		err = srv.Serve(ln)
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("sonar: server error: %v", err)
	}
}

func reportListenError(err error, port int) {
	switch {
	case errors.Is(err, syscall.EACCES), errors.Is(err, fs.ErrPermission):
		fmt.Fprintf(os.Stderr, "sonar: permission denied binding to port %d.\n\n", port)
		fmt.Fprintf(os.Stderr, "Ports below 1024 require elevated privileges. Try one of:\n\n")
		fmt.Fprintf(os.Stderr, "  sudo sonar --port %d ...\n", port)
		fmt.Fprintf(os.Stderr, "  sudo setcap 'cap_net_bind_service=+ep' \"$(which sonar)\"\n")
		fmt.Fprintf(os.Stderr, "  sonar --port 8443              # or any unprivileged port\n")
	case errors.Is(err, syscall.EADDRINUSE):
		fmt.Fprintf(os.Stderr, "sonar: port %d is already in use by another process.\n", port)
		fmt.Fprintf(os.Stderr, "Choose a different port with --port, or stop the process using it.\n")
	default:
		fmt.Fprintf(os.Stderr, "sonar: failed to bind :%d: %v\n", port, err)
	}
}
