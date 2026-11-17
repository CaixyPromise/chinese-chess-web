package auth

import "testing"

func TestClientAddressOnlyTrustsConfiguredImmediateProxy(t *testing.T) {
	for _, tc := range []struct{ name, proxy, peer, header, want string }{
		{"trusted", "192.0.2.3", "192.0.2.3", "203.0.113.42", "203.0.113.42"},
		{"direct spoof", "192.0.2.3", "203.0.113.9", "203.0.113.42", "203.0.113.9"},
		{"default ignores header", "", "192.0.2.3", "203.0.113.42", "192.0.2.3"},
		{"reject chain", "192.0.2.3", "192.0.2.3", "203.0.113.42, 203.0.113.9", "192.0.2.3"},
		{"mapped peer", "192.0.2.3", "::ffff:192.0.2.3", "2001:db8::1", "2001:db8::1"},
		{"invalid", "192.0.2.3", "192.0.2.3", "invalid", "192.0.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{config: Config{TrustedProxyIP: tc.proxy}}
			if got := s.ClientAddress(tc.peer, tc.header); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
