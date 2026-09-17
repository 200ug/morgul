package util

import (
	"slices"
	"testing"
)

func TestRandomHostname(t *testing.T) {
	host := RandomHostname()
	if !slices.Contains(Hostnames, host) {
		t.Errorf("RandomHostname() returned %q, which is not in Hostnames list", host)
	}
}
