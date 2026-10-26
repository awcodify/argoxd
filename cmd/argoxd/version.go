package main

import "runtime/debug"

// version is set by GoReleaser through -ldflags "-X main.version=...".
var version = "dev"

// releaseVersion names the build: the release version when GoReleaser set one,
// else the module version that go install records, else "dev".
func releaseVersion(release string, info *debug.BuildInfo) string {
	if release == "dev" && info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return release
}

func versionString(release string, info *debug.BuildInfo) string {
	return "argoxd " + releaseVersion(release, info)
}
