package netio

import (
	"net/netip"
	"testing"
	"time"
)

// Echo reaches this machine's loopback, IPv4 and IPv6, with no privilege: the reply comes from
// where it was sent, with status 0, an IPv4 one with the TTL Windows gives a packet, and a round
// trip under a second.
func TestEcho_reachesTheLoopback(t *testing.T) {
	for _, target := range []string{"127.0.0.1", "::1"} {
		address := netip.MustParseAddr(target)
		pinger, err := NewPinger(address.Is6())
		if err != nil {
			t.Fatalf("NewPinger for %s: %v", target, err)
		}
		reply, err := pinger.Echo(address, 56, 0, 2*time.Second)
		pinger.Close()
		if err != nil || reply.Status != 0 || reply.From != address || reply.RTT <= 0 || reply.RTT > time.Second {
			t.Errorf("Echo(%s) = %+v, %v", target, reply, err)
		}
		if address.Is4() && reply.TTL <= 0 || address.Is6() && reply.TTL != -1 {
			t.Errorf("Echo(%s) has TTL %d", target, reply.TTL)
		}
	}
}
