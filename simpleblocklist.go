// Package simpleblocklist a Traefik plugin to block requests based on a list of IP addresses.
package simpleblocklist

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	blacklistedIPs             []net.IP
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

	infoLogger.Printf("Loaded %d blacklisted IPs", len(blacklistedIPs))
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

func loadBlacklistedIPs(path string) ([]net.IP, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ips []net.IP
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := net.ParseIP(strings.TrimSpace(scanner.Text()))
		if ip != nil {
			ips = append(ips, ip)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ips, nil
}

func (a *SimpleBlocklist) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	reqIPAddr, err := a.collectRemoteIP(req)
	if err != nil {
		infoLogger.Println(err)
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	for _, ipAddress := range reqIPAddr {
		ipAddressString := ipAddress.String()
		privateIP := isPrivateIP(*ipAddress, a.privateIPRanges)

		if privateIP {
			if a.allowLocalRequests {
				if a.logLocalRequests {
					infoLogger.Println("Local ip allowed: ", ipAddress)
				}
				a.next.ServeHTTP(rw, req)
			} else {
				if a.logLocalRequests {
					infoLogger.Println("Local ip denied: ", ipAddress)
				}
				rw.WriteHeader(a.httpStatusCodeDeniedRequest)
			}
			return
		}

		for _, blacklistedIP := range a.blacklistedIPs {
			if ipAddress.Equal(blacklistedIP) {
				infoLogger.Printf("%s: request denied [%s] - IP is blacklisted", a.name, ipAddressString)
				rw.WriteHeader(a.httpStatusCodeDeniedRequest)
				return
			}
		}
	}

	a.next.ServeHTTP(rw, req)
}

func (a *SimpleBlocklist) collectRemoteIP(req *http.Request) ([]*net.IP, error) {
	var ipList []*net.IP

	splitFn := func(c rune) bool {
		return c == ','
	}

	xForwardedForValue := req.Header.Get(xForwardedFor)
	xForwardedForIPs := strings.FieldsFunc(xForwardedForValue, splitFn)

	xRealIPValue := req.Header.Get(xRealIP)
	xRealIPList := strings.FieldsFunc(xRealIPValue, splitFn)

	for _, value := range xForwardedForIPs {
		ipAddress, err := parseIP(value)
		if err != nil {
			return ipList, fmt.Errorf("parsing failed: %s", err)
		}

		ipList = append(ipList, &ipAddress)
	}

	for _, value := range xRealIPList {
		ipAddress, err := parseIP(value)
		if err != nil {
			return ipList, fmt.Errorf("parsing failed: %s", err)
		}

		ipList = append(ipList, &ipAddress)
	}

	return ipList, nil
}

func parseIP(addr string) (net.IP, error) {
	ipAddress := net.ParseIP(strings.TrimSpace(addr))
	if ipAddress == nil {
		return nil, fmt.Errorf("unable parse IP address from address [%s]", addr)
	}
	return ipAddress, nil
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
