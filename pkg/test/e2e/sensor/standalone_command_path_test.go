package sensor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mintoolkit/mint/pkg/app/sensor/standalone/control"
)

func TestStartStandaloneUsesDefaultControlCommandFile(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "docker-args")
	execArgsFile := filepath.Join(dir, "docker-exec-args")
	// Only this POSIX fake is on PATH; no command can fall back to a real daemon.
	fake := `#!/bin/sh
case "$1:$2" in
container:create)
    printf '%s\000' "$@" > "$MINT_SENSOR_TEST_DOCKER_ARGS"
    printf 'fake-container\n'
    ;;
container:start)
    [ "$3" = "fake-container" ] || exit 64
    ;;
container:exec)
    printf '%s\000' "$@" > "$MINT_SENSOR_TEST_DOCKER_EXEC_ARGS"
    ;;
*) exit 64 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("MINT_SENSOR_TEST_DOCKER_ARGS", argsFile)
	t.Setenv("MINT_SENSOR_TEST_DOCKER_EXEC_ARGS", execArgsFile)

	// Bypass NewSensor: image pull/inspect are deliberately outside this unit test.
	sensor := &Sensor{
		contextDirPath: dir,
		contName:       "test-sensor",
		sensorExePath:  "/fixture/mint-sensor",
		user:           "root",
	}
	if err := json.Unmarshal(
		[]byte(`{"Id":"fake-image","Config":{"Cmd":["sleep","1"]}}`),
		&sensor.image,
	); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sensor.StartStandalone(ctx, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	hasPair := func(flag, value string) bool {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == flag && args[i+1] == value {
				return true
			}
		}
		return false
	}

	// This literal independently pins the sensor control CLI's default.
	// Do not reuse sensorCommandsFilePath here: the old helper must still compile
	// and fail these assertions, as must a change to only one side of the route.
	const wantPath = "/opt/_mint/commands.json"
	wantMount := filepath.Join(dir, CommandsFileName) + ":" + wantPath
	if !hasPair("--volume", wantMount) {
		t.Errorf("commands-file mount does not use the control default %q: %q", wantMount, args)
	}
	if !hasPair("-c", wantPath) {
		t.Errorf("commands-file flag does not use the control default %q: %q", wantPath, args)
	}
	if !hasPair("--entrypoint", "/opt/_slim/sensor") {
		t.Errorf("sensor executable path unexpectedly changed: %q", args)
	}

	if err := sensor.ExecuteControlCommand(ctx, control.StopTargetAppCommand); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(execArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	controlArgs := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	wantControlArgs := []string{
		"container", "exec", "fake-container", "/opt/_slim/sensor", "control", "stop-target-app",
	}
	if strings.Join(controlArgs, "\x00") != strings.Join(wantControlArgs, "\x00") {
		t.Errorf("default control invocation unexpectedly changed: got %q, want %q", controlArgs, wantControlArgs)
	}
}
