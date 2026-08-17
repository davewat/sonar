package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// GeoIPInfo holds the subset of ip-api.com's response we display.
type GeoIPInfo struct {
	Country string
	Region  string
	City    string
	ISP     string
}

type geoIPResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
}

const geoIPTimeout = 2 * time.Second

var geoIPClient = &http.Client{Timeout: geoIPTimeout}

// lookupGeoIP queries ip-api.com's free, keyless endpoint for the given IP.
// It returns (nil, nil) for private/loopback/unspecified addresses, which
// are deliberately never sent out. Any network or API error is returned to
// the caller, which is expected to degrade gracefully (omit the section)
// rather than fail the request.
func lookupGeoIP(ctx context.Context, ip string) (*GeoIPInfo, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("geoip: invalid IP %q", ip)
	}
	if isPrivateOrLoopback(parsed) {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, geoIPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/"+ip, nil)
	if err != nil {
		return nil, err
	}

	resp, err := geoIPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geoip: unexpected status %d", resp.StatusCode)
	}

	var body geoIPResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("geoip: lookup failed: %s", body.Message)
	}

	return &GeoIPInfo{
		Country: body.Country,
		Region:  body.RegionName,
		City:    body.City,
		ISP:     body.ISP,
	}, nil
}

func isPrivateOrLoopback(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast()
}
