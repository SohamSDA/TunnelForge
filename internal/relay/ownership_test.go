package relay

import (
	"context"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"testing"
	"time"
)

func TestResponseMustBelongToTunnel(t *testing.T) {
	p, _ := NewPendingRequests(10, time.Second)
	id, result, err := p.Register(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	response := protocol.HTTPResponse{StatusCode: 200}
	if err := p.ResolveFor("attacker", id, response); err == nil {
		t.Fatal("cross-tunnel response accepted")
	}
	if p.Count() != 1 {
		t.Fatal("request was consumed")
	}
	if err := p.ResolveFor("owner", id, response); err != nil {
		t.Fatal(err)
	}
	if (<-result).Response.StatusCode != 200 {
		t.Fatal("owner response lost")
	}
}
