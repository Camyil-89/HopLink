package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerSourceRootUsesDirectoryAboveExecutable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "hoplink"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", filepath.Join("cmd", "hoplink", "main.go")} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package main\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(root, "deploy", "HopLink.exe")
	got, err := serverSourceRoot(executable)
	if err != nil || got != root {
		t.Fatalf("source root = %q, %v; want %q", got, err, root)
	}
	if err := os.Remove(filepath.Join(root, "cmd", "hoplink", "main.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := serverSourceRoot(executable); err == nil {
		t.Fatal("missing source tree should produce an error")
	}
}
