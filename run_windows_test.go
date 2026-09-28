//go:build windows

package main

import "testing"

func TestSingleInstanceIDStableByNormalisedDataDir(t *testing.T) {
	a := singleInstanceID(`C:\\Stories\\Data`)
	b := singleInstanceID(`c:\\stories\\data\\.`)
	if a != b {
		t.Fatalf("ids differ: %q %q", a, b)
	}
	if a == singleInstanceID(`C:\\Stories\\Other`) {
		t.Fatal("different data dirs shared an id")
	}
}

func TestExportForWhitelist(t *testing.T) {
	for _, kind := range []string{"book", "outline", "proofread-report", "project-backup"} {
		if _, err := exportFor(kind, "demo"); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	if _, err := exportFor("anything", "demo"); err == nil {
		t.Fatal("unknown export was accepted")
	}
}
