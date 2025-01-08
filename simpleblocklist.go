// Package simpleblocklist a Traefik plugin to block requests based on a list of IP addresses.
package simpleblocklist

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	xForwardedFor = "X-Forwarded-For"
	xRealIP       = "X-Real-IP"
	defaultDeniedRequestHTTPStatusCode = 403
)

var (
	infoLogger = log.New(os.Stdout, "INFO: SimpleBlocklist: ", log.Ldate|log.Ltime)
)

// Config the plugin configuration.
type Config struct {
	BlacklistPath              string `yaml:"blacklistPath"`
	AllowLocalRequests         bool   `yaml:"allowLocalRequests"`
	LogLocalRequests          bool   `yaml:"logLocalRequests"`
	HTTPStatusCodeDeniedRequest int   `yaml:"httpStatusCodeDeniedRequest"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		HTTPStatusCodeDeniedRequest: defaultDeniedRequestHTTPStatusCode,
		AllowLocalRequests: true,
		LogLocalRequests: false,
	}
}

// SimpleBlocklist a Traefik plugin.
type SimpleBlocklist struct {
	next                        http.Handler
	blacklistedIPs             []*net.IPNet
	allowLocalRequests         bool
	logLocalRequests          bool
	privateIPRanges           []*net.IPNet
	httpStatusCodeDeniedRequest int
	name                       string
}

// New created a new SimpleBlocklist plugin.
func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.BlacklistPath) == 0 {
		return nil, fmt.Errorf("no blacklist file path provided")
	}

	blacklistedIPs, err := loadBlacklistedIPs(config.BlacklistPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load blacklist: %v", err)
	}

	if config.HTTPStatusCodeDeniedRequest != 0 {
		if len(http.StatusText(config.HTTPStatusCodeDeniedRequest)) == 0 {
			return nil, fmt.Errorf("invalid denied request status code supplied")
		}
	} else {
		config.HTTPStatusCodeDeniedRequest = defaultDeniedRequestHTTPStatusCode
	}

	infoLogger.Printf("Loaded %d blacklisted IPs/Networks", len(blacklistedIPs))
	infoLogger.Printf("Allow local IPs: %t", config.AllowLocalRequests)
	infoLogger.Printf("Log local requests: %t", config.LogLocalRequests)
	infoLogger.Printf("Denied request status code: %d", config.HTTPStatusCodeDeniedRequest)

	return &SimpleBlocklist{
		next:                        next,
		blacklistedIPs:             blacklistedIPs,
		allowLocalRequests:         config.AllowLocalRequests,
		logLocalRequests:          config.LogLocalRequests,
		privateIPRanges:           initPrivateIPBlocks(),
		httpStatusCodeDeniedRequest: config.HTTPStatusCodeDeniedRequest,
		name:                       name,
	}, nil
}

func loadBlacklistedIPs(path string) ([]*net.IPNet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ips []*net.IPNet
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try parsing as CIDR first
		if _, ipNet, err := net.ParseCIDR(line); err == nil {
			ips = append(ips, ipNet)
			continue
		}

		// If not CIDR, try as single IP
		if ip := net.ParseIP(line); ip != nil {
			// Convert single IP to /32 CIDR
			ipNet := &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(32, 32),
			}
			ips = append(ips, ipNet)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ips, nil
}

func (a *SimpleBlocklist) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ipAddresses := a.collectRemoteIP(req)

	for _, ipStr := range ipAddresses {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			infoLogger.Printf("Failed to parse IP: %s", ipStr)
			continue
		}

		if isPrivateIP(ip, a.privateIPRanges) {
			if a.allowLocalRequests {
				if a.logLocalRequests {
					infoLogger.Printf("Local IP allowed: %s", ipStr)
				}
				a.next.ServeHTTP(rw, req)
			} else {
				if a.logLocalRequests {
					infoLogger.Printf("Local IP denied: %s", ipStr)
				}
				rw.WriteHeader(a.httpStatusCodeDeniedRequest)
			}
			return
		}

		for _, blacklistedNet := range a.blacklistedIPs {
			if blacklistedNet.Contains(ip) {
				infoLogger.Printf("%s: request denied [%s] - IP is blacklisted", a.name, ipStr)
				rw.WriteHeader(a.httpStatusCodeDeniedRequest)
				return
			}
		}
	}

	a.next.ServeHTTP(rw, req)
}

func (a *SimpleBlocklist) collectRemoteIP(req *http.Request) []string {
	var ipList []string

	// Get IPs from X-Forwarded-For
	xff := req.Header.Get(xForwardedFor)
	if xff != "" {
		for _, addr := range strings.Split(xff, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				ipList = append(ipList, addr)
			}
		}
	}

	// Get IP from X-Real-IP
	if xRealIP := req.Header.Get(xRealIP); xRealIP != "" {
		ipList = append(ipList, strings.TrimSpace(xRealIP))
	}

	// Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, try using RemoteAddr directly
		remoteAddr := strings.TrimSpace(req.RemoteAddr)
		if remoteAddr != "" {
			ipList = append(ipList, remoteAddr)
		}
	} else {
		ipList = append(ipList, ip)
	}

	return ipList
}

func initPrivateIPBlocks() []*net.IPNet {
	var privateIPBlocks []*net.IPNet
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
	return privateIPBlocks
}

func isPrivateIP(ip net.IP, privateIPBlocks []*net.IPNet) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}
