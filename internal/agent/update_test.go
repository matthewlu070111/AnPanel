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

func TestCustomDockerSpec(t *testing.T) {
	spec, err := dockerAppSpec("custom", map[string]string{"container_name": "my-app", "image": "nginx:alpine", "host_port": "8080", "container_port": "80"})
	if err != nil || spec.name != "my-app" || spec.replace {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
	if _, err := dockerAppSpec("custom", map[string]string{"container_name": "bad name", "image": "nginx", "host_port": "8080", "container_port": "80"}); err == nil {
		t.Fatal("invalid container name accepted")
	}
}
