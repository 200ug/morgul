package tui

import (
	"testing"

	"codeberg.org/2ug/morgul/internal/config"
)

func TestParsePorts(t *testing.T) {
	ports, err := parsePorts("8080:80, 9090:90")
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 || ports[0] != (config.PortMap{Host: 8080, Container: 80}) {
		t.Errorf("got %v", ports)
	}
}

func TestParsePorts_Empty(t *testing.T) {
	ports, err := parsePorts("")
	if err != nil || ports != nil {
		t.Errorf("got %v, %v", ports, err)
	}
}

func TestParsePorts_Invalid(t *testing.T) {
	if _, err := parsePorts("8080"); err == nil {
		t.Error("expected error for single-segment port")
	}
	if _, err := parsePorts("abc:80"); err == nil {
		t.Error("expected error for non-numeric port")
	}
}

func TestParseVolumes(t *testing.T) {
	vols, err := parseVolumes("cache:/tmp/cache, data:/data")
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 2 || vols[1].Name != "data" || vols[1].Path != "/data" {
		t.Errorf("got %v", vols)
	}
}

func TestParseVolumes_PathWithColon(t *testing.T) {
	vols, err := parseVolumes("cache:/tmp/a:b")
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 1 || vols[0].Path != "/tmp/a:b" {
		t.Errorf("got %v", vols)
	}
}

func TestParseVolumes_Invalid(t *testing.T) {
	if _, err := parseVolumes("nopath"); err == nil {
		t.Error("expected error for missing path")
	}
}

func TestParseEnvFiles(t *testing.T) {
	envs, err := parseEnvFiles("/a/.env, /b/.env")
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 || envs[1] != "/b/.env" {
		t.Errorf("got %v", envs)
	}
}

func TestFuzzyFind(t *testing.T) {
	names := []string{"proj-alpha", "proj-beta", "other"}
	got := fuzzyFind("beta", names)
	if len(got) == 0 || names[got[0]] != "proj-beta" {
		t.Errorf("got %v", got)
	}
}
