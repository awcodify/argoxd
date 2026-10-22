package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFlagIsParsed(t *testing.T) {
	isolateConfig(t, "")

	options, err := parseOptions([]string{"--version"})
	if err != nil {
		t.Fatal(err)
	}

	if !options.version {
		t.Fatal("options.version = false, want true")
	}
}

func TestVersionStringPrefersReleaseVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.0.1"}}

	if got := versionString("0.4.1", info); got != "argoxd 0.4.1" {
		t.Fatalf("versionString() = %q, want release version", got)
	}
}

func TestVersionStringFallsBackToModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.4.1"}}

	if got := versionString("dev", info); got != "argoxd v0.4.1" {
		t.Fatalf("versionString() = %q, want module version from go install", got)
	}
}

func TestVersionStringReportsDevelopmentBuilds(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {Main: debug.Module{Version: "(devel)"}}} {
		if got := versionString("dev", info); got != "argoxd dev" {
			t.Fatalf("versionString() = %q, want dev", got)
		}
	}
}
