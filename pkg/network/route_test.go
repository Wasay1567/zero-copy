package network

import (
	"net"
	"strings"
	"testing"
)

func TestBuildDefaultRoute(t *testing.T) {
	tests := []struct {
		name    string
		index   int
		gateway net.IP
		wantErr string
	}{
		{name: "valid gateway", index: 7, gateway: net.ParseIP("10.244.1.1")},
		{name: "missing gateway", index: 7, wantErr: "gateway IP is required"},
		{name: "IPv6 gateway", index: 7, gateway: net.ParseIP("2001:db8::1"), wantErr: "not an IPv4 address"},
		{name: "invalid link index", index: 0, gateway: net.ParseIP("10.244.1.1"), wantErr: "invalid link index"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			route, err := buildDefaultRoute(test.index, test.gateway)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("buildDefaultRoute() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildDefaultRoute() error = %v", err)
			}
			if route.Dst.String() != "0.0.0.0/0" {
				t.Fatalf("route destination = %s, want 0.0.0.0/0", route.Dst)
			}
			if !route.Gw.Equal(test.gateway.To4()) {
				t.Fatalf("route gateway = %s, want %s", route.Gw, test.gateway)
			}
		})
	}
}
