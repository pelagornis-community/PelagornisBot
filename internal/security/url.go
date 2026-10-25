package security

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func SafeURL(ctx context.Context, raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid URL")
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if e != nil {
		return nil, e
	}
	for _, v := range ips {
		if private(v.IP) {
			return nil, fmt.Errorf("private address blocked")
		}
	}
	return u, nil
}
func private(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || strings.HasPrefix(ip.String(), "169.254.169.254")
}
