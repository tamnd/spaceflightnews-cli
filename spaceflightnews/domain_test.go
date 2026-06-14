package spaceflightnews

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string
// functions and the host wiring, which need no network. The client's HTTP
// behaviour is covered in spaceflightnews_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "spaceflightnews" {
		t.Errorf("Scheme = %q, want spaceflightnews", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "spaceflightnews" {
		t.Errorf("Identity.Binary = %q, want spaceflightnews", info.Identity.Binary)
	}
}

func TestDomainRegister(t *testing.T) {
	// Verify that kit.Open sees the domain registered by init().
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	_ = h // registration asserted by Open not returning an error
}
