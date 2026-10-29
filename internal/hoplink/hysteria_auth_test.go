package hoplink

import (
	"net/url"
	"testing"
)

func TestHy2PasswordAuth(t *testing.T) {
	a := hy2PasswordAuth{password: "relay-secret", users: []hy2User{{name: "alice", password: "short"}, {name: "bob", password: "longer-secret"}}}
	for _, tc := range []struct {
		password, name string
		ok             bool
	}{
		{"relay-secret", "relay", true},
		{"short", "alice", true},
		{"longer-secret", "bob", true},
		{"wrong", "", false},
		{"", "", false},
	} {
		ok, name := a.Authenticate(nil, tc.password, 0)
		if ok != tc.ok || name != tc.name {
			t.Errorf("Authenticate(%q) = (%v, %q), want (%v, %q)", tc.password, ok, name, tc.ok, tc.name)
		}
	}
}

func TestMakeHy2ShareLinkEscapesPassword(t *testing.T) {
	password := "a@b:c#d% e"
	link := makeHy2ShareLink(password, "example.com", "22033", "example.com", "pin", "HopLink")
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.User == nil || u.User.Username() != password || u.Host != "example.com:22033" {
		t.Fatalf("unexpected parsed link: %q", link)
	}
}
