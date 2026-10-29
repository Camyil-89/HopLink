package desktop

import "testing"

func TestLatestComponentUsesIndependentVersions(t *testing.T) {
	releases := []githubRelease{
		{Tag: "desktop-hoplink-v1.2.9"},
		{Tag: "hoplink-v3.0.1"},
		{Tag: "desktop-hoplink-latest"},
		{Tag: "desktop-hoplink-v1.10.0"},
		{Tag: "desktop-hoplink-v9.0.0", Draft: true},
	}
	_, version, err := latestComponent(releases, "desktop-hoplink")
	if err != nil || version != "1.10.0" {
		t.Fatalf("desktop version = %q, %v", version, err)
	}
	_, version, err = latestComponent(releases, "hoplink")
	if err != nil || version != "3.0.1" {
		t.Fatalf("service version = %q, %v", version, err)
	}
}
