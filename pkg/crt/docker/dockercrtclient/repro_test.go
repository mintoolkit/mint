package dockercrtclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/mintoolkit/mint/pkg/imagebuilder"
)

// TestBuildImage_MissingOutputStream_Issue87 reproduces mintoolkit/mint#87
// ("Missing output stream"): building a Dockerfile-based ("fat") image
// through the internal build engine (dockercrtclient), WITHOUT passing an
// explicit OutputStream and WITHOUT --show-blogs (showBuildLogs=false),
// must not immediately fail with "missing output stream" coming from the
// vendored fsouza/go-dockerclient BuildImage(), which requires a non-nil
// OutputStream unconditionally.
func TestBuildImage_MissingOutputStream_Issue87(t *testing.T) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Skipf("no docker client available, skipping live repro: %v", err)
	}
	if err := client.Ping(); err != nil {
		t.Skipf("no docker daemon reachable, skipping live repro: %v", err)
	}

	dir := t.TempDir()
	dockerfile := "FROM scratch\nCOPY hello.txt /hello.txt\n"
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}

	// Unique per run so the test can never overwrite or delete an image that
	// happens to exist on the developer's machine, and so parallel runs on the
	// same daemon don't collide.
	imageTag := fmt.Sprintf("mint-issue87-repro-%d:test", time.Now().UnixNano())

	b := NewBuilder(client, false) // showBuildLogs=false, matches the report (no --show-blogs)

	opts := imagebuilder.DockerfileBuildOptions{
		Dockerfile:   "Dockerfile",
		BuildContext: dir,
		ImagePath:    imageTag,
		// OutputStream intentionally left nil - this is the exact shape
		// buildFatImage() in pkg/app/master/command/build/image.go constructs.
	}

	buildErr := b.BuildImage(opts)
	if buildErr == nil {
		// Only clean up an image this test actually created.
		t.Cleanup(func() {
			if err := client.RemoveImage(imageTag); err != nil {
				t.Logf("could not remove test image %s: %v", imageTag, err)
			}
		})
	}

	if buildErr != nil {
		if strings.Contains(buildErr.Error(), "missing output stream") {
			t.Fatalf("BuildImage failed with %q: options.OutputStream was nil and showBuildLogs=false, "+
				"so no output destination was set and go-dockerclient rejected the build before running it", buildErr)
		}
		t.Fatalf("BuildImage failed for an unrelated reason: %v", buildErr)
	}

	if _, err := client.InspectImage(imageTag); err != nil {
		t.Fatalf("image was not built despite BuildImage returning nil error: %v", err)
	}
	if b.BuildOutputLog() == "" {
		t.Fatalf("expected the build log to be captured even with showBuildLogs=false")
	}
}
