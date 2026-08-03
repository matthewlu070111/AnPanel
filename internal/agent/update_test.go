package agent

import "testing"

func TestUpdateInstallURL(t *testing.T) {
	for _, channel := range []string{"stable", "prerelease"} {
		if url, err := updateInstallURL(channel); err != nil || url == "" {
			t.Fatalf("%s: url=%q err=%v", channel, url, err)
		}
	}
	if _, err := updateInstallURL("nightly"); err == nil {
		t.Fatal("nightly channel accepted")
	}
}
