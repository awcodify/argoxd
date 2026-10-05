package main

import "testing"

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
