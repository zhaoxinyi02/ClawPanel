package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedAssetsAvailable(t *testing.T) {
	frontendDist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		t.Fatalf("fs.Sub(frontendFS) error = %v", err)
	}

	indexData, err := fs.ReadFile(frontendDist, "index.html")
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if len(indexData) == 0 {
		t.Fatal("embedded index.html is empty")
	}
	if !strings.Contains(strings.ToLower(string(indexData)), "<!doctype html") {
		t.Fatal("embedded index.html does not look like HTML")
	}
	if len(faqMD) == 0 {
		t.Fatal("embedded FAQ markdown is empty")
	}
}
