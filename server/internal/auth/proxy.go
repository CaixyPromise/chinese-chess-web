package auth

import "net"

// Only a configured immediate proxy may supply one canonical client address.
// Forwarded chains and headers from direct clients never change the limiter key.
func (s *Service) ClientAddress(peer, forwarded string) string {
	trusted, source, client := net.ParseIP(s.config.TrustedProxyIP), net.ParseIP(peer), net.ParseIP(forwarded)
	if trusted != nil && source != nil && trusted.Equal(source) && client != nil {
		return client.String()
	}
	return peer
}
