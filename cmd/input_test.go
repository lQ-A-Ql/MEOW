package cmd

import (
	"strings"
	"testing"
)

func TestReadBanner(t *testing.T) {
	got, err := readBanner(strings.NewReader("  Linux version 5.4.0-163-generic ...  \n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Linux version 5.4.0-163-generic ..." {
		t.Fatalf("got %q", got)
	}
}

func TestReadBannerEmpty(t *testing.T) {
	_, err := readBanner(strings.NewReader("\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}
