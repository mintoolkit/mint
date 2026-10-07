package dockerclient

import (
	"errors"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

// TestWithDiscoveredAPIVersionKeepsAnExplicitVersion asserts the branch every
// caller relied on before: a configured version is left alone, and the
// redundant self-check is skipped.
func TestWithDiscoveredAPIVersionKeepsAnExplicitVersion(t *testing.T) {
	client, err := docker.NewVersionedClient("http://127.0.0.1:1", "1.41")
	if err != nil {
		t.Fatalf("building the fixture client: %v", err)
	}

	rebuilt := false
	got := withDiscoveredAPIVersion(client, "1.41", func(string) (*docker.Client, error) {
		rebuilt = true
		return nil, nil
	})

	if rebuilt {
		t.Fatal("a configured API version must not trigger a probe or a rebuild")
	}

	if got != client {
		t.Fatal("a configured API version must leave the client as it is")
	}

	if !got.SkipServerVersionCheck {
		t.Fatal("a configured API version must skip the redundant server version check")
	}
}

// TestWithDiscoveredAPIVersionKeepsTheClientWhenTheDaemonIsUnreachable makes
// sure the probe never turns a connection problem into a different error: the
// caller gets the client it would have had, and fails where it always did.
func TestWithDiscoveredAPIVersionKeepsTheClientWhenTheDaemonIsUnreachable(t *testing.T) {
	// Port 1 refuses connections, which is what an absent daemon looks like.
	client, err := docker.NewVersionedClient("http://127.0.0.1:1", "")
	if err != nil {
		t.Fatalf("building the fixture client: %v", err)
	}

	rebuilt := false
	got := withDiscoveredAPIVersion(client, "", func(string) (*docker.Client, error) {
		rebuilt = true
		return nil, nil
	})

	if rebuilt {
		t.Fatal("an unreachable daemon must not produce a rebuilt client")
	}

	if got != client {
		t.Fatal("an unreachable daemon must leave the original client in place")
	}
}

// TestWithDiscoveredAPIVersionRebuildsThroughTheCallersConstructor is the
// TLS and environment paths' coverage: they differ from the plain path only
// in the constructor they hand over, so this asserts that the discovered
// version reaches that constructor and that its client is the one returned.
func TestWithDiscoveredAPIVersionRebuildsThroughTheCallersConstructor(t *testing.T) {
	var probePath string
	srv := daemonAnsweringVersion(t, &probePath)
	defer srv.Close()

	client, err := docker.NewVersionedClient(srv.URL, "")
	if err != nil {
		t.Fatalf("building the fixture client: %v", err)
	}

	replacement, err := docker.NewVersionedClient(srv.URL, "1.44")
	if err != nil {
		t.Fatalf("building the replacement client: %v", err)
	}

	var handed string
	got := withDiscoveredAPIVersion(client, "", func(apiVersion string) (*docker.Client, error) {
		handed = apiVersion
		return replacement, nil
	})

	if handed != "1.44" {
		t.Fatalf("the constructor was handed %q, expected the version the daemon reported", handed)
	}

	if got != replacement {
		t.Fatal("the client built by the caller's own constructor must be the one returned")
	}

	if !got.SkipServerVersionCheck {
		t.Fatal("the rebuilt client must skip the self-check that repeats the probe just made")
	}
}

// TestWithDiscoveredAPIVersionKeepsTheClientWhenTheRebuildFails covers the
// remaining branch: a constructor that fails leaves the working client in
// place rather than dropping the caller into a nil.
func TestWithDiscoveredAPIVersionKeepsTheClientWhenTheRebuildFails(t *testing.T) {
	var probePath string
	srv := daemonAnsweringVersion(t, &probePath)
	defer srv.Close()

	client, err := docker.NewVersionedClient(srv.URL, "")
	if err != nil {
		t.Fatalf("building the fixture client: %v", err)
	}

	got := withDiscoveredAPIVersion(client, "", func(string) (*docker.Client, error) {
		return nil, errors.New("no certificates here")
	})

	if got != client {
		t.Fatal("a failed rebuild must leave the original client in place")
	}
}
