package desktop

import "testing"

func TestApplyServerLibraryKeepsMapLayout(t *testing.T) {
	lib := Server{ID: "a", Name: "new", Host: "203.0.113.1", SSHPort: 22, SSHUser: "root", SSHPassword: "x"}
	mapped := Server{ID: "a", Name: "old", Host: "198.51.100.1", X: 120, Y: 200, Role: "entry"}
	got := applyServerLibrary(lib, mapped)
	if got.Host != lib.Host || got.Name != lib.Name || got.X != 120 || got.Y != 200 || got.Role != "entry" {
		t.Fatalf("got %+v", got)
	}
}