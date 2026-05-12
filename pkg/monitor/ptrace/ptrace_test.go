package ptrace

import (
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/report"
	"github.com/mintoolkit/mint/pkg/system"
)

func TestGetIntVal(t *testing.T) {
	tt := []struct {
		input    uint64
		expected int
	}{
		{input: 0, expected: 0},
		{input: 0xFFFFFFFE, expected: -2},  // ENOENT
		{input: 0xFFFFFFEC, expected: -20}, // ENOTDIR
		{input: 1, expected: 1},
		{input: 0xFFFFFFFF, expected: -1}, // Generic error
	}

	for _, test := range tt {
		result := getIntVal(test.input)
		if result != test.expected {
			t.Errorf("getIntVal(0x%x) = %d, want %d", test.input, result, test.expected)
		}
	}
}

// TestCheckFileSyscallProcessorOKReturnStatus verifies that OKReturnStatus
// only accepts successful stat calls (retVal == 0). ENOENT, ENOTDIR, and
// other errors are rejected — only paths that actually exist get tracked.
func TestCheckFileSyscallProcessorOKReturnStatus(t *testing.T) {
	processor := &checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{},
	}

	tt := []struct {
		retVal   uint64
		expected bool
		desc     string
	}{
		{
			retVal:   0,
			expected: true,
			desc:     "success (0) - file exists, should be tracked",
		},
		{
			retVal:   0xFFFFFFFE, // -2 as uint64
			expected: false,
			desc:     "ENOENT (-2) - file not found, should NOT be tracked",
		},
		{
			retVal:   0xFFFFFFEC, // -20 as uint64
			expected: false,
			desc:     "ENOTDIR (-20) - not a directory, should NOT be tracked",
		},
		{
			retVal:   0xFFFFFFFF, // -1 as uint64
			expected: false,
			desc:     "EPERM (-1) - should NOT be tracked",
		},
		{
			retVal:   0xFFFFFFFD, // -3 as uint64
			expected: false,
			desc:     "ESRCH (-3) - should NOT be tracked",
		},
		{
			retVal:   1,
			expected: false,
			desc:     "positive return value - should NOT be tracked",
		},
	}

	for _, test := range tt {
		result := processor.OKReturnStatus(test.retVal)
		if result != test.expected {
			t.Errorf("OKReturnStatus(0x%x) [%s] = %v, want %v",
				test.retVal, test.desc, result, test.expected)
		}
	}
}

func TestCheckFileSyscallProcessorFailedReturnStatus(t *testing.T) {
	processor := &checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{},
	}

	tt := []struct {
		retVal   uint64
		expected bool
		desc     string
	}{
		{
			retVal:   0,
			expected: false,
			desc:     "success (0) - not failed",
		},
		{
			retVal:   0xFFFFFFFE, // -2 (ENOENT)
			expected: true,
			desc:     "ENOENT (-2) - failed",
		},
		{
			retVal:   0xFFFFFFFF, // -1 (EPERM)
			expected: true,
			desc:     "EPERM (-1) - failed",
		},
	}

	for _, test := range tt {
		result := processor.FailedReturnStatus(test.retVal)
		if result != test.expected {
			t.Errorf("FailedReturnStatus(0x%x) [%s] = %v, want %v",
				test.retVal, test.desc, result, test.expected)
		}
	}
}

// TestOKReturnStatusAndFailedReturnStatusAreComplements verifies that after
// reverting OKReturnStatus to success-only, OKReturnStatus and
// FailedReturnStatus are now proper complements (no overlap).
func TestOKReturnStatusAndFailedReturnStatusAreComplements(t *testing.T) {
	processor := &checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{},
	}

	tt := []struct {
		retVal     uint64
		okReturn   bool
		failReturn bool
		desc       string
	}{
		{
			retVal:     0,
			okReturn:   true,
			failReturn: false,
			desc:       "success (0)",
		},
		{
			retVal:     0xFFFFFFFE, // -2
			okReturn:   false,
			failReturn: true,
			desc:       "ENOENT (-2)",
		},
		{
			retVal:     0xFFFFFFEC, // -20
			okReturn:   false,
			failReturn: true,
			desc:       "ENOTDIR (-20)",
		},
		{
			retVal:     0xFFFFFFFF, // -1
			okReturn:   false,
			failReturn: true,
			desc:       "EPERM (-1)",
		},
	}

	for _, test := range tt {
		okResult := processor.OKReturnStatus(test.retVal)
		failResult := processor.FailedReturnStatus(test.retVal)
		if okResult != test.okReturn {
			t.Errorf("[%s] OKReturnStatus(0x%x) = %v, want %v",
				test.desc, test.retVal, okResult, test.okReturn)
		}
		if failResult != test.failReturn {
			t.Errorf("[%s] FailedReturnStatus(0x%x) = %v, want %v",
				test.desc, test.retVal, failResult, test.failReturn)
		}
		if okResult == failResult {
			t.Errorf("[%s] OKReturnStatus and FailedReturnStatus should be "+
				"complements but both returned %v", test.desc, okResult)
		}
	}
}

// newTestApp creates a minimal App suitable for testing processFileActivity
// and FileActivity without needing exec, ptrace, or signal infrastructure.
func newTestApp() *App {
	return &App{
		fsActivity: map[string]*report.FSActivityInfo{},
		logger:     log.WithField("test", true),
	}
}

// lookupStatCallNum returns the syscall number for "stat" (or "newfstatat" on
// architectures that don't have "stat"). Panics if neither is found, since
// init() should have registered them.
func lookupStatCallNum() uint32 {
	if num, found := system.LookupCallNumber("stat"); found {
		return num
	}
	if num, found := system.LookupCallNumber("newfstatat"); found {
		return num
	}
	panic("neither stat nor newfstatat syscall found — init() may not have run")
}

// lookupOpenatCallNum returns the syscall number for "openat".
func lookupOpenatCallNum() uint32 {
	num, found := system.LookupCallNumber("openat")
	if !found {
		panic("openat syscall not found")
	}
	return num
}

// TestProcessFileActivity_ENOENTNotRecorded verifies that stat calls returning
// ENOENT (-2) are NOT recorded in fsActivity. Only successful calls (retVal==0)
// pass the OKReturnStatus gate.
func TestProcessFileActivity_ENOENTNotRecorded(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   statNum,
		retVal:    0xFFFFFFFE, // -2 = ENOENT
		pathParam: "/usr/lib/foo/__init__.cpython-311.so",
	})

	if _, found := app.fsActivity["/usr/lib/foo/__init__.cpython-311.so"]; found {
		t.Error("ENOENT stat path should NOT be recorded — " +
			"OKReturnStatus should only accept retVal == 0")
	}
}

// TestProcessFileActivity_SuccessPathsRecorded verifies that stat calls
// returning success (0) are recorded in fsActivity.
func TestProcessFileActivity_SuccessPathsRecorded(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   statNum,
		retVal:    0, // success
		pathParam: "/usr/lib/foo/__init__.py",
	})

	if _, found := app.fsActivity["/usr/lib/foo/__init__.py"]; !found {
		t.Fatal("successful stat path was NOT recorded in fsActivity")
	}
}

// TestProcessFileActivity_OpenFileSuccessRecorded verifies that openat calls
// with a successful fd (>= 0) are recorded.
func TestProcessFileActivity_OpenFileSuccessRecorded(t *testing.T) {
	app := newTestApp()
	openatNum := lookupOpenatCallNum()

	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   openatNum,
		retVal:    3, // fd=3, success
		pathParam: "/etc/config.json",
	})

	if _, found := app.fsActivity["/etc/config.json"]; !found {
		t.Fatal("successful openat path was NOT recorded in fsActivity")
	}
}

// TestProcessFileActivity_EPERMNotRecorded verifies that stat calls returning
// EPERM (-1) are NOT recorded.
func TestProcessFileActivity_EPERMNotRecorded(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   statNum,
		retVal:    0xFFFFFFFF, // -1 = EPERM
		pathParam: "/root/secret",
	})

	if _, found := app.fsActivity["/root/secret"]; found {
		t.Error("EPERM stat path should NOT be recorded")
	}
}

// TestFileActivity_NamespacePackageDirectoryPreserved tests that a namespace
// package directory (no __init__.py) is preserved in FileActivity() when only
// the directory itself has a successful stat. Since ENOENT probes are no longer
// tracked, there are no ghost children to cause IsSubdir false positives.
func TestFileActivity_NamespacePackageDirectoryPreserved(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Only the directory stat succeeds — ENOENT probes are not tracked
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/foo",
	})

	// These ENOENT probes should be silently ignored by the gate
	for _, p := range []string{
		"/usr/lib/foo/__init__.cpython-311.so",
		"/usr/lib/foo/__init__.so",
		"/usr/lib/foo/__init__.py",
	} {
		app.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0xFFFFFFFE, pathParam: p, // ENOENT
		})
	}

	result := app.FileActivity()

	if _, found := result["/usr/lib/foo"]; !found {
		t.Error("/usr/lib/foo was excluded — namespace package directory " +
			"should be preserved since no ghost children can trigger IsSubdir")
	}

	// Verify no ghost paths leaked into the result
	if len(result) != 1 {
		t.Errorf("expected exactly 1 entry (the directory), got %d", len(result))
		for k := range result {
			t.Logf("  result key: %s", k)
		}
	}
}

// TestFileActivity_RealChildrenTriggerIsSubdir verifies that when a child path
// has a successful stat (file exists), the parent directory is correctly marked
// IsSubdir and excluded from the result.
func TestFileActivity_RealChildrenTriggerIsSubdir(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Parent directory — success
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/bar",
	})

	// Real child file — success
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/bar/__init__.py",
	})

	result := app.FileActivity()

	if _, found := result["/usr/lib/bar"]; found {
		t.Error("/usr/lib/bar should be excluded — " +
			"a real child should trigger IsSubdir")
	}

	if _, found := result["/usr/lib/bar/__init__.py"]; !found {
		t.Error("/usr/lib/bar/__init__.py should be in result")
	}
}

// TestFileActivity_NoGhostPathsInReport verifies that ENOENT stat results
// never appear in the FileActivity() output.
func TestFileActivity_NoGhostPathsInReport(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Mix of successful and failed stats
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/real.so",
	})
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFFE, pathParam: "/usr/lib/ghost.so", // ENOENT
	})
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFEC, pathParam: "/usr/lib/ghost2.so", // ENOTDIR
	})

	result := app.FileActivity()

	if _, found := result["/usr/lib/real.so"]; !found {
		t.Error("real path should be in result")
	}
	if _, found := result["/usr/lib/ghost.so"]; found {
		t.Error("ENOENT ghost path should NOT be in result")
	}
	if _, found := result["/usr/lib/ghost2.so"]; found {
		t.Error("ENOTDIR ghost path should NOT be in result")
	}
}

// TestFileActivity_MixedRealAndGhostChildren tests the realistic Python
// import scenario: a package directory has both ENOENT probes and one real
// file. Only the real file and its IsSubdir effect should matter.
func TestFileActivity_MixedRealAndGhostChildren(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Parent directory
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/pkg",
	})

	// ENOENT probes — should be silently ignored
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFFE, pathParam: "/usr/lib/pkg/__init__.cpython-311.so",
	})
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFFE, pathParam: "/usr/lib/pkg/__init__.so",
	})

	// Real file — success
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/pkg/__init__.py",
	})

	result := app.FileActivity()

	// Parent should be excluded (real child exists and triggers IsSubdir)
	if _, found := result["/usr/lib/pkg"]; found {
		t.Error("/usr/lib/pkg should be excluded — real child __init__.py " +
			"triggers IsSubdir")
	}

	// Real child should be present
	if _, found := result["/usr/lib/pkg/__init__.py"]; !found {
		t.Error("/usr/lib/pkg/__init__.py should be in result")
	}

	// No ghost paths should be present
	if len(result) != 1 {
		t.Errorf("expected exactly 1 entry (__init__.py), got %d", len(result))
		for k := range result {
			t.Logf("  result key: %s", k)
		}
	}
}
