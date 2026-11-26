//go:build e2e
// +build e2e

package sensor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mintoolkit/mint/pkg/ipc/event"
	testsensor "github.com/mintoolkit/mint/pkg/test/e2e/sensor"
)

func TestExcludePattern(t *testing.T) {
	// Create a temporary directory for the sensor executable
	sensorBinDir := t.TempDir()
	t.Setenv("DSLIM_SENSOR_PATH", filepath.Join(sensorBinDir, testsensor.LocalBinFile))
	if err := os.WriteFile(filepath.Join(sensorBinDir, testsensor.LocalBinFile), []byte(""), 0755); err != nil {
		t.Fatalf("Failed to create dummy sensor executable: %v", err)
	}

	runID := newTestRun(t)
	ctx := context.Background()

	// Setup a temporary directory with a file to be excluded
	tmpDir := t.TempDir()
	excludeDir := filepath.Join(tmpDir, "exclude_me")
	err := os.Mkdir(excludeDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create exclude directory: %v", err)
	}

	excludeFile := filepath.Join(excludeDir, "file.txt")
	err = os.WriteFile(excludeFile, []byte("some data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create exclude file: %v", err)
	}

	sensor := testsensor.NewSensorOrFail(t, ctx, tmpDir, runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartControlledOrFail(t, ctx)

	sensor.SendStartCommandOrFail(t, ctx,
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppNameArgs("sh", "-c", "ls -R /"),
			testsensor.WithExcludes("/tmp/exclude_me/**"),
		),
	)
	sensor.ExpectEvent(t, event.StartMonitorDone)

	time.Sleep(5 * time.Second)

	sensor.SendStopCommandOrFail(t, ctx)
	sensor.ExpectEvent(t, event.StopMonitorDone)

	sensor.ShutdownOrFail(t, ctx)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)

	// Assert that the directory and file are not included in the report.
	sensor.AssertReportNotIncludesFiles(t,
		"/tmp/exclude_me",
		"/tmp/exclude_me/file.txt",
	)
}
