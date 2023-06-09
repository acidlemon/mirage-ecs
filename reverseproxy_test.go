package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReverseProxy(t *testing.T) {
	cfg, err := NewConfig(&ConfigParams{
		Domain: "example.net",
	})
	if err != nil {
		t.Error(err)
	}
	cfg.Listen.HTTP = []PortMap{
		{ListenPort: 80, TargetPort: 80},
		{ListenPort: 8080, TargetPort: 8080},
	}
	rp := NewReverseProxy(cfg)

	if ds := rp.Subdomains(); len(ds) != 0 {
		t.Errorf("invalid subdomains %#v", ds)
	}
	rp.AddSubdomain("bbb", "192.168.1.2", 80)
	rp.AddSubdomain("aaa", "192.168.1.1", 80)
	rp.AddSubdomain("ccc", "192.168.1.3", 80)
	if diff := cmp.Diff(rp.Subdomains(), []string{"bbb", "aaa", "ccc"}); diff != "" {
		t.Errorf("invalid subdomains %s", diff)
	}
	for _, d := range rp.Subdomains() {
		if !rp.Exists(d) {
			t.Errorf("subdomain %s not found", d)
		}
	}

	// add same subdomain
	rp.AddSubdomain("aaa", "192.168.1.1", 80)
	if diff := cmp.Diff(rp.Subdomains(), []string{"bbb", "aaa", "ccc"}); diff != "" {
		t.Errorf("after added same: invalid subdomains %s", diff)
	}

	// add same subdomain with different port
	rp.AddSubdomain("aaa", "192.168.1.1", 8080)
	if diff := cmp.Diff(rp.Subdomains(), []string{"bbb", "aaa", "ccc"}); diff != "" {
		t.Errorf("after added same with different port: invalid subdomains %s", diff)
	}

	for _, port := range []int{80, 8080} {
		h := rp.findHandler("aaa", port)
		if h == nil {
			t.Errorf("handler not found for aaa:%d", port)
		}
	}

	// remove subdomain
	rp.RemoveSubdomain("aaa")
	if diff := cmp.Diff(rp.Subdomains(), []string{"bbb", "ccc"}); diff != "" {
		t.Errorf("after removed: invalid subdomains %s", diff)
	}

	// wildcard
	rp.AddSubdomain("foo-*", "10.0.0.1", 80)
	rp.AddSubdomain("foo-bar-*", "10.0.0.2", 80)
	rp.AddSubdomain("*-baz", "10.0.0.3", 80)
	for _, name := range []string{"foo-111", "foo-bar-222", "111-baz"} {
		if !rp.Exists(name) {
			t.Errorf("subdomain %s not found", name)
		}
	}

	{
		h1 := rp.findHandler("foo-999", 80)
		if h1 == nil {
			t.Errorf("handler not found for foo-999")
		}
		h2 := rp.findHandler("foo-baz", 80) // foo-baz is matched with "foo-*"
		if h2 != h1 {
			t.Errorf("handler not matched for foo-*")
		}
	}
	{
		h1 := rp.findHandler("foo-bar-999", 80)
		if h1 == nil {
			t.Errorf("handler not found for foo-bar-999")
		}
		h2 := rp.findHandler("foo-bar-baz", 80) // foo-bar-baz is matched with "foo-*"
		if h2 != h1 {
			t.Errorf("handler not matched for foo-*")
		}
	}
}
