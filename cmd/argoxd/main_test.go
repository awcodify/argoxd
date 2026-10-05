package main

import (
	"testing"
	"time"
)

func TestBuildSourceRequiresAPIServer(t *testing.T) {
	_, err := buildSource(options{source: "api"})
	if err == nil {
		t.Fatal("buildSource() error = nil, want missing server error")
	}
}

func TestBuildSourceRejectsUnknownMode(t *testing.T) {
	_, err := buildSource(options{source: "unknown"})
	if err == nil {
		t.Fatal("buildSource() error = nil, want invalid mode error")
	}
}

func TestRefreshFlagDefaultsToFiveSeconds(t *testing.T) {
	options := parseOptions(nil)

	if options.refresh != 5*time.Second {
		t.Fatalf("refresh = %v, want 5s", options.refresh)
	}
}
