package dockerclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mintoolkit/mint/pkg/app/master/config"
)

// daemonAnsweringVersion is a stand-in daemon that answers the unversioned
// "/version" probe and records the path of the next request it receives.
//
// The match on "/version" is exact on purpose: once the client knows an API
// version it prefixes every path, so a lenient match would take a later
// "/v1.44/version" for the probe and hide the very thing under test.
func daemonAnsweringVersion(t *testing.T, seen *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.44"}`))
			return
		}

		*seen = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestNewClientVersionsRequestsWithoutExplicitAPIVersion covers #95 on the
// plain host path.
//
// When the caller leaves DOCKER_API_VERSION / config.APIVersion empty - the
// default for a plain "mint build" - go-dockerclient never parses an API
// version into the client's requestedAPIVersion, and getURL() consults that
// field alone when it builds a request path. Every request the client sends
// therefore omits the "/vX.Y/" segment. A daemon reached through a proxy,
// which is what dind gives a CI pipeline, reads an unversioned request as
// coming from the oldest client it supports and rejects it with
// "client version ... is too old".
//
// The client's own /version negotiation does not help: its result is stored
// in expectedAPIVersion and never copied into requestedAPIVersion. So the
// request worth asserting on is the next one - here Info()'s GET /info -
// which is the one a proxying daemon actually rejects.
func TestNewClientVersionsRequestsWithoutExplicitAPIVersion(t *testing.T) {
	// New() writes DOCKER_HOST (and DOCKER_API_VERSION) into the process
	// environment. t.Setenv restores whatever was there when the test ends,
	// so a closed test server's URL cannot leak into later tests.
	t.Setenv(EnvDockerHost, "")
	t.Setenv(EnvDockerAPIVer, "")

	var infoPath string
	srv := daemonAnsweringVersion(t, &infoPath)
	defer srv.Close()

	cfg := &config.DockerClient{
		Host:   srv.URL,
		UseTLS: false,
		// Left empty on purpose: this is the path every report hit.
		APIVersion: "",
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with an empty APIVersion must succeed: %v", err)
	}

	if _, err := client.Info(); err != nil {
		t.Fatalf("Info() must succeed against a daemon that answers /version: %v", err)
	}

	if infoPath == "" {
		t.Fatal("expected the fake daemon to receive the /info request, got none")
	}

	if !strings.HasPrefix(infoPath, "/v1.44/") {
		t.Fatalf("the /info request path %q carries no API version segment although the daemon "+
			"answered the version probe; a daemon behind a proxy rejects such a request as coming "+
			"from a client that is too old (#95)", infoPath)
	}
}

// TestNewClientFromEnvVersionsRequests covers the same defect on the
// DOCKER_HOST path, which reaches the daemon through
// docker.NewClientFromEnv() instead of an explicit host.
func TestNewClientFromEnvVersionsRequests(t *testing.T) {
	var infoPath string
	srv := daemonAnsweringVersion(t, &infoPath)
	defer srv.Close()

	t.Setenv(EnvDockerHost, srv.URL)
	t.Setenv(EnvDockerAPIVer, "")
	t.Setenv(EnvDockerTLSVerify, "")

	cfg := &config.DockerClient{
		Env: map[string]string{EnvDockerHost: srv.URL},
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() from the environment must succeed: %v", err)
	}

	if _, err := client.Info(); err != nil {
		t.Fatalf("Info() must succeed against a daemon that answers /version: %v", err)
	}

	if !strings.HasPrefix(infoPath, "/v1.44/") {
		t.Fatalf("the /info request path %q carries no API version segment on the DOCKER_HOST "+
			"path (#95)", infoPath)
	}
}
