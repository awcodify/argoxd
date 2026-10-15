package main

import "testing"

func TestReadOnlyCanBeSetByConfigOrFlag(t *testing.T) {
	isolateConfig(t, "readOnly: true\n")
	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !options.readOnly {
		t.Fatal("readOnly = false, want true from config")
	}

	options, err = parseOptions([]string{"--read-only=false"})
	if err != nil {
		t.Fatal(err)
	}
	if options.readOnly {
		t.Fatal("readOnly = true, want false from flag")
	}
}
