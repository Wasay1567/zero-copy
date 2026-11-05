package network

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestFirstUsableIP(t *testing.T) {
	tests := []struct {
		cidr    string
		want    string
		wantErr string
	}{
		{cidr: "10.244.1.0/24", want: "10.244.1.1"},
		{cidr: "10.244.1.37/24", want: "10.244.1.1"},
		{cidr: "10.244.1.0/31", wantErr: fmt.Sprintf("network CIDR %q is too small", "10.244.1.0/31")},
		{cidr: "10.244.1.1/32", wantErr: fmt.Sprintf("network CIDR %q is too small", "10.244.1.1/32")},
		{cidr: "2001:db8::/64", wantErr: "not IPv4"},
		{cidr: "not-a-cidr", wantErr: "failed to parse"},
	}

	for _, test := range tests {
		t.Run(test.cidr, func(t *testing.T) {
			gateway, err := firstUsableIP(test.cidr)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("firstUsableIP() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("firstUsableIP() error = %v", err)
			}
			if !gateway.Equal(net.ParseIP(test.want)) {
				t.Fatalf("firstUsableIP() = %s, want %s", gateway, test.want)
			}
		})
	}
}

func TestIPAMGatewayRequiresPrefix(t *testing.T) {
	var ipam *IPAM
	if _, err := ipam.Gateway(); err == nil {
		t.Fatal("Gateway() unexpectedly succeeded with nil IPAM")
	}
	if _, err := (&IPAM{}).Gateway(); err == nil {
		t.Fatal("Gateway() unexpectedly succeeded with nil prefix")
	}
}
