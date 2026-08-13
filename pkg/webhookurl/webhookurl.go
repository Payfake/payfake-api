// Package webhookurl validates and dials merchant-controlled webhook URLs.
// Webhook destinations are an SSRF boundary because the API server, not the
// merchant's browser, makes the outbound request.
package webhookurl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Validate(rawURL string, allowPrivate bool) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https")
	}
	if u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("webhook URL must contain a host and no embedded credentials")
	}
	if allowPrivate {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = resolvePublicIPs(ctx, u.Hostname())
	return err
}

// NewClient validates the address at dial time as well as at configuration
// time. This closes the DNS-rebinding gap where a hostname is public when
// saved but later resolves to 127.0.0.1 or a cloud metadata address.
func NewClient(allowPrivate bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if allowPrivate {
				return dialer.DialContext(ctx, network, address)
			}
			ips, err := resolvePublicIPs(ctx, host)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}

	client := &http.Client{Transport: transport, Timeout: timeout}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("stopped after 5 webhook redirects")
		}
		return Validate(req.URL.String(), allowPrivate)
	}
	return client
}

func resolvePublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	if strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("private webhook destinations are not allowed")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve webhook host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("webhook host resolved to no addresses")
	}

	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
			return nil, fmt.Errorf("private webhook destinations are not allowed")
		}
		ips = append(ips, ip)
	}
	return ips, nil
}
