package handlers

import "net"

// onSameLAN reports whether a check-in (clientIP) is "in the room" relative to the
// coordinator's device (coordinatorIP) — the network-proximity presence proof.
//
// The coordinator's laptop is the room's Wi-Fi (hotspot, no internet uplink), so
// the ONLY way to reach the system is from that local network. The rule, in order:
//  1. unparseable client → allow (never block on a parse failure)
//  2. identical egress IP (campus-NAT-to-cloud model) → same place
//  3. same /24 subnet (hotspot: coordinator 10.42.0.1, students 10.42.0.x) → same LAN
//  4. client is a private/loopback address → on a local network (the hotspot case,
//     robust even when the coordinator's own IP shows as loopback)
//  5. otherwise (a public/remote client) → reject as off-network proxy
func onSameLAN(clientIP, coordinatorIP string) bool {
	c := net.ParseIP(stripPort(clientIP))
	if c == nil {
		return true
	}
	k := net.ParseIP(stripPort(coordinatorIP))
	if k != nil {
		if c.Equal(k) {
			return true
		}
		if c4, k4 := c.To4(), k.To4(); c4 != nil && k4 != nil &&
			c4[0] == k4[0] && c4[1] == k4[1] && c4[2] == k4[2] {
			return true
		}
	}
	return c.IsPrivate() || c.IsLoopback() || c.IsLinkLocalUnicast()
}

// stripPort removes a trailing :port (and handles bracketed IPv6) if present.
func stripPort(addr string) string {
	if addr == "" {
		return addr
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
