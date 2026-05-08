package cmd

import "testing"

func TestBuildCommandRemovedLegacyBackendFlags(t *testing.T) {
	build := Commands["build"]
	if build == nil {
		t.Fatalf("build command not registered")
	}
	if build.Flags.Lookup("backend") != nil {
		t.Fatalf("legacy --backend should be removed")
	}
	if build.Flags.Lookup("wsl-distro") != nil {
		t.Fatalf("legacy --wsl-distro should be removed")
	}
}

func TestDoctorCommandRemovedLegacyBackendFlags(t *testing.T) {
	doctor := Commands["doctor"]
	if doctor == nil {
		t.Fatalf("doctor command not registered")
	}
	if doctor.Flags.Lookup("backend") != nil {
		t.Fatalf("legacy --backend should be removed")
	}
	if doctor.Flags.Lookup("wsl-distro") != nil {
		t.Fatalf("legacy --wsl-distro should be removed")
	}
}
