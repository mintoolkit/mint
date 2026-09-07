package dockercrtclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		t.Fatalf("docker.NewClientFromEnv: %v", err)
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

	b := NewBuilder(client, false) // showBuildLogs=false, matches the user's repro command (no --show-blogs)

	opts := imagebuilder.DockerfileBuildOptions{
		Dockerfile:   "Dockerfile",
		BuildContext: dir,
		ImagePath:    "el-mint87-repro:latest",
		// OutputStream intentionally left nil - this is the exact shape
		// buildFatImage() in pkg/app/master/command/build/image.go constructs.
	}

	buildErr := b.BuildImage(opts)
	defer client.RemoveImage("el-mint87-repro:latest")

	if buildErr != nil {
		if strings.Contains(buildErr.Error(), "missing output stream") {
			t.Fatalf("REPRODUCED issue #87: BuildImage failed with %q (options.OutputStream was nil and showBuildLogs=false, "+
				"so dockercrtclient never set buildOptions.OutputStream, and vendor/github.com/fsouza/go-dockerclient "+
				"image.go:538-540 rejects the whole build before running anything)", buildErr)
		}
		t.Fatalf("BuildImage failed for an unrelated reason: %v", buildErr)
	}

	// GREEN-path sanity: image must actually exist and the build log must be
	// non-empty even though showBuildLogs was false (fix always buffers into
	// ref.buildLog, matching the pattern already used by slimbuilder.go:267).
	if _, err := client.InspectImage("el-mint87-repro:latest"); err != nil {
		t.Fatalf("image was not built despite BuildImage returning nil error: %v", err)
	}
	if b.BuildOutputLog() == "" {
		t.Fatalf("expected non-empty build log to be captured even with showBuildLogs=false")
	}
}
