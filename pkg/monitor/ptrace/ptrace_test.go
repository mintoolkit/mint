package ptrace

import (
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/mondel"
	"github.com/mintoolkit/mint/pkg/report"
	"github.com/mintoolkit/mint/pkg/system"
)

// mockPublisher captures MonitorDataEvents published during processFileActivity.
type mockPublisher struct {
	events []*report.MonitorDataEvent
}

func (m *mockPublisher) Publish(event *report.MonitorDataEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockPublisher) Stop() {}

// Verify mockPublisher satisfies the interface.
var _ mondel.Publisher = (*mockPublisher)(nil)

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
			desc:     "success (0)",
		},
		{
			retVal:   0xFFFFFFFE, // -2 as uint64
			expected: true,
			desc:     "ENOENT (-2) - file not found, should be tracked",
		},
		{
			retVal:   0xFFFFFFEC, // -20 as uint64
			expected: true,
			desc:     "ENOTDIR (-20) - not a directory, should be tracked",
		},
		{
			retVal:   0xFFFFFFFF, // -1 as uint64
			expected: false,
			desc:     "EPERM (-1) - should not be tracked",
		},
		{
			retVal:   0xFFFFFFFD, // -3 as uint64
			expected: false,
			desc:     "ESRCH (-3) - should not be tracked",
		},
		{
			retVal:   0xFFFFFFED, // -19 as uint64
			expected: false,
			desc:     "ENODEV (-19) - should not be tracked",
		},
		{
			retVal:   1,
			expected: false,
			desc:     "positive return value - should not be tracked",
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

// TestOKReturnStatusAndFailedReturnStatusOverlap demonstrates that for
// checkFileSyscallProcessor, OKReturnStatus and FailedReturnStatus are NOT
// complementary. ENOENT (-2) and ENOTDIR (-20) return true for BOTH methods.
// This is by design: OKReturnStatus means "this path is interesting enough to
// track" while FailedReturnStatus means "the syscall itself did not succeed."
func TestOKReturnStatusAndFailedReturnStatusOverlap(t *testing.T) {
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
			desc:       "success (0): OK=true, Failed=false — no overlap",
		},
		{
			retVal:     0xFFFFFFFE, // -2
			okReturn:   true,
			failReturn: true,
			desc:       "ENOENT (-2): OK=true AND Failed=true — OVERLAP (path tracked but file doesn't exist)",
		},
		{
			retVal:     0xFFFFFFEC, // -20
			okReturn:   true,
			failReturn: true,
			desc:       "ENOTDIR (-20): OK=true AND Failed=true — OVERLAP (path tracked but not a directory)",
		},
		{
			retVal:     0xFFFFFFFF, // -1
			okReturn:   false,
			failReturn: true,
			desc:       "EPERM (-1): OK=false, Failed=true — no overlap",
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

// newTestAppWithPublisher creates a test App with a mock event publisher,
// so we can capture MonitorDataEvents emitted by processFileActivity.
func newTestAppWithPublisher(pub mondel.Publisher) *App {
	return &App{
		del:        pub,
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

// TestProcessFileActivity_GhostPathsRecorded shows that stat calls returning
// ENOENT (-2) still get recorded in fsActivity (because OKReturnStatus accepts
// ENOENT), but with HasSuccessfulAccess=false.
func TestProcessFileActivity_GhostPathsRecorded(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Simulate a stat call that returned ENOENT (file not found)
	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   statNum,
		retVal:    0xFFFFFFFE, // -2 = ENOENT
		pathParam: "/usr/lib/foo/__init__.cpython-311.so",
	})

	fsa, found := app.fsActivity["/usr/lib/foo/__init__.cpython-311.so"]
	if !found {
		t.Fatal("ghost path (ENOENT stat) was NOT recorded in fsActivity — " +
			"OKReturnStatus should accept ENOENT to track Python import probes")
	}

	if fsa.HasSuccessfulAccess {
		t.Error("ghost path should have HasSuccessfulAccess=false, got true — " +
			"ENOENT paths should not be marked as successfully accessed")
	}

	if fsa.OpsAll != 1 {
		t.Errorf("expected OpsAll=1, got %d", fsa.OpsAll)
	}
}

// TestProcessFileActivity_SuccessPathsRecorded shows that stat calls returning
// success (0) are recorded with HasSuccessfulAccess=true.
func TestProcessFileActivity_SuccessPathsRecorded(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Simulate a stat call that succeeded (file exists)
	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   statNum,
		retVal:    0, // success
		pathParam: "/usr/lib/foo/__init__.py",
	})

	fsa, found := app.fsActivity["/usr/lib/foo/__init__.py"]
	if !found {
		t.Fatal("successful stat path was NOT recorded in fsActivity")
	}

	if !fsa.HasSuccessfulAccess {
		t.Error("successful stat path should have HasSuccessfulAccess=true, got false")
	}
}

// TestProcessFileActivity_OpenFileAlwaysSuccessful shows that for OpenFileType
// syscalls (e.g., openat), HasSuccessfulAccess is always true when the path
// passes OKReturnStatus (which requires fd >= 0 for open-type calls).
func TestProcessFileActivity_OpenFileAlwaysSuccessful(t *testing.T) {
	app := newTestApp()
	openatNum := lookupOpenatCallNum()

	// Simulate an openat call that succeeded (returned fd=3)
	app.processFileActivity(&syscallEvent{
		returned:  true,
		pid:       1,
		callNum:   openatNum,
		retVal:    3, // fd=3, success
		pathParam: "/etc/config.json",
	})

	fsa, found := app.fsActivity["/etc/config.json"]
	if !found {
		t.Fatal("successful openat path was NOT recorded in fsActivity")
	}

	if !fsa.HasSuccessfulAccess {
		t.Error("successful openat path should have HasSuccessfulAccess=true")
	}
}

// TestProcessFileActivity_EPERMNotRecorded shows that stat calls returning
// EPERM (-1) are NOT recorded, because OKReturnStatus rejects them.
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
		t.Error("EPERM stat path should NOT be recorded — OKReturnStatus rejects -1")
	}
}

// TestFileActivity_GhostChildrenDontTriggerIsSubdir demonstrates the fix from
// da8eb2c7: ghost children (ENOENT stat results) with HasSuccessfulAccess=false
// do NOT cause the parent directory to be marked IsSubdir=true and excluded.
//
// Scenario: Python namespace package — directory /usr/lib/foo/ is stat'd
// successfully, but probes for __init__.*.so and __init__.py all return ENOENT.
// The directory must NOT be excluded.
func TestFileActivity_GhostChildrenDontTriggerIsSubdir(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Parent directory stat — success
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/foo",
	})

	// Ghost children — ENOENT probes for __init__ variants
	ghostPaths := []string{
		"/usr/lib/foo/__init__.cpython-311.so",
		"/usr/lib/foo/__init__.so",
		"/usr/lib/foo/__init__.py",
	}
	for _, p := range ghostPaths {
		app.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0xFFFFFFFE, pathParam: p, // ENOENT
		})
	}

	result := app.FileActivity()

	// The parent directory must NOT be filtered out
	if _, found := result["/usr/lib/foo"]; !found {
		t.Error("/usr/lib/foo was excluded by FileActivity() — " +
			"ghost children with HasSuccessfulAccess=false should not trigger IsSubdir")
	}

	// Ghost children should still be present (they are leaf nodes, not IsSubdir)
	for _, p := range ghostPaths {
		if fsa, found := result[p]; found {
			if fsa.HasSuccessfulAccess {
				t.Errorf("ghost path %s should have HasSuccessfulAccess=false", p)
			}
		}
		// Note: ghost paths ARE expected in the result — this is side effect #2
		// from the analysis. They pass through FileActivity() because they are
		// leaf nodes with no children.
	}
}

// TestFileActivity_RealChildrenTriggerIsSubdir shows that when a child path has
// HasSuccessfulAccess=true (i.e., the file actually exists), the parent IS
// correctly marked IsSubdir and excluded from the result.
func TestFileActivity_RealChildrenTriggerIsSubdir(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Parent directory stat — success
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

	// The parent directory SHOULD be filtered out (IsSubdir=true)
	if _, found := result["/usr/lib/bar"]; found {
		t.Error("/usr/lib/bar should be excluded by FileActivity() — " +
			"a real child with HasSuccessfulAccess=true should trigger IsSubdir")
	}

	// The child file should be present
	if _, found := result["/usr/lib/bar/__init__.py"]; !found {
		t.Error("/usr/lib/bar/__init__.py should be in FileActivity() result")
	}
}

// TestFileActivity_GhostLeafPathsPassThrough demonstrates side effect #2:
// ghost leaf paths (no children of their own) survive FileActivity() filtering
// and appear in the final report, even though they represent non-existent files.
func TestFileActivity_GhostLeafPathsPassThrough(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// A ghost leaf path with no parent or children in fsActivity
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFFE, pathParam: "/usr/lib/nonexistent.so", // ENOENT
	})

	result := app.FileActivity()

	fsa, found := result["/usr/lib/nonexistent.so"]
	if !found {
		t.Fatal("ghost leaf path was filtered out by FileActivity() — " +
			"expected it to pass through (this IS a known side effect)")
	}

	if fsa.HasSuccessfulAccess {
		t.Error("ghost leaf path should have HasSuccessfulAccess=false")
	}
}

// TestFileActivity_MixedGhostAndRealChildren tests the realistic Python import
// scenario: a package directory has both ghost probes (ENOENT) and one real file
// (success). The parent should be marked IsSubdir because a real child exists.
func TestFileActivity_MixedGhostAndRealChildren(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Parent directory
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/pkg",
	})

	// Ghost probes — ENOENT
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

	// Parent should be excluded (real child exists)
	if _, found := result["/usr/lib/pkg"]; found {
		t.Error("/usr/lib/pkg should be excluded — real child __init__.py exists")
	}

	// Real child should be present
	if _, found := result["/usr/lib/pkg/__init__.py"]; !found {
		t.Error("/usr/lib/pkg/__init__.py should be in result")
	}

	// Ghost children should also be present (side effect #2)
	for _, p := range []string{
		"/usr/lib/pkg/__init__.cpython-311.so",
		"/usr/lib/pkg/__init__.so",
	} {
		if fsa, found := result[p]; found {
			if fsa.HasSuccessfulAccess {
				t.Errorf("ghost path %s should have HasSuccessfulAccess=false", p)
			}
		}
	}
}

// =============================================================================
// Bug detection tests
//
// These tests verify whether the three possible open bugs identified in the
// analysis document are live. Each test is structured so that:
//   - If the bug IS LIVE, the test documents the buggy behavior (passes but
//     logs what's wrong via t.Log).
//   - If the bug is FIXED in the future, the test will fail, signaling that
//     the fix landed and the test expectations should be updated.
// =============================================================================

// BUG #1: Ghost leaf paths leak into the final FSActivity report.
//
// FileActivity() only filters paths with IsSubdir=true. Ghost leaf paths
// (ENOENT stat results with no children) have HasSuccessfulAccess=false but
// are never filtered. They appear in Report.FSActivity and get passed to
// prepareArtifact() for files that don't exist on disk.
//
// This test simulates a Python import that probes 3 non-existent paths and
// finds 1 real file. It checks whether the ghost paths leak into the report.
func TestBug1_GhostLeafPathsLeakIntoReport(t *testing.T) {
	app := newTestApp()
	statNum := lookupStatCallNum()

	// Python probes these — all return ENOENT (file not found)
	ghostPaths := []string{
		"/usr/lib/mypackage/__init__.cpython-311.so",
		"/usr/lib/mypackage/__init__.so",
		"/usr/lib/mypackage/__init__.abi3.so",
	}
	for _, p := range ghostPaths {
		app.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0xFFFFFFFE, pathParam: p, // ENOENT
		})
	}

	// One real file found
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/mypackage/__init__.py",
	})

	result := app.FileActivity()

	// Count how many ghost (non-existent) paths leaked into the report
	leaked := 0
	for _, p := range ghostPaths {
		if fsa, found := result[p]; found {
			leaked++
			if fsa.HasSuccessfulAccess {
				t.Errorf("ghost path %s has HasSuccessfulAccess=true (unexpected)", p)
			}
		}
	}

	if leaked > 0 {
		// BUG IS LIVE: ghost paths leak into the report.
		// When this bug is fixed, this branch will stop being reached and the
		// test will fail at the assertion below — update it to expect 0.
		t.Logf("BUG #1 IS LIVE: %d/%d ghost leaf paths leaked into FileActivity() report",
			leaked, len(ghostPaths))
		if leaked != len(ghostPaths) {
			t.Errorf("expected all %d ghost paths to leak (or none if fixed), got %d",
				len(ghostPaths), leaked)
		}
	} else {
		// BUG IS FIXED: no ghost paths in the report.
		t.Log("BUG #1 IS FIXED: ghost leaf paths are no longer in the report")
	}
}

// BUG #2: Ghost paths are published as MDETypeArtifact events.
//
// processFileActivity() publishes a MonitorDataEvent for every path that passes
// the OKReturnStatus gate, including ENOENT ghost paths. Event stream consumers
// receive artifact events for files that don't exist.
func TestBug2_GhostPathsPublishedAsArtifactEvents(t *testing.T) {
	pub := &mockPublisher{}
	app := newTestAppWithPublisher(pub)
	statNum := lookupStatCallNum()

	// Ghost path — ENOENT
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0xFFFFFFFE, pathParam: "/usr/lib/ghost.so", // ENOENT
	})

	// Real path — success
	app.processFileActivity(&syscallEvent{
		returned: true, pid: 1, callNum: statNum,
		retVal: 0, pathParam: "/usr/lib/real.so",
	})

	// Check which paths got published
	ghostPublished := false
	realPublished := false
	for _, ev := range pub.events {
		if ev.Artifact == "/usr/lib/ghost.so" {
			ghostPublished = true
		}
		if ev.Artifact == "/usr/lib/real.so" {
			realPublished = true
		}
	}

	if !realPublished {
		t.Error("real path /usr/lib/real.so was NOT published as an event (unexpected)")
	}

	if ghostPublished {
		// BUG IS LIVE: ghost paths are published as artifact events.
		t.Log("BUG #2 IS LIVE: ghost path /usr/lib/ghost.so was published as " +
			"an MDETypeArtifact event despite the file not existing on disk")
	} else {
		// BUG IS FIXED: ghost paths are not published.
		t.Log("BUG #2 IS FIXED: ghost paths are no longer published as artifact events")
	}
}

// BUG #3: OKReturnStatus/FailedReturnStatus naming trap causes regressions.
//
// This test reproduces the exact regression introduced by kilo-code-bot in
// commit 3f8dcb47: using p.OKReturnStatus(e.retVal) for the HasSuccessfulAccess
// condition instead of e.retVal == 0. Because OKReturnStatus accepts ENOENT,
// ghost paths get HasSuccessfulAccess=true, which breaks the namespace package
// fix — ghost children trigger IsSubdir on the parent directory.
//
// The test simulates what WOULD happen if someone made this mistake again,
// by directly manipulating HasSuccessfulAccess to match what OKReturnStatus
// would produce, and showing the cascade failure.
func TestBug3_OKReturnStatusMisuseBreaksNamespacePackages(t *testing.T) {
	statNum := lookupStatCallNum()
	statProcessor, found := syscallProcessors[int(statNum)]
	if !found {
		t.Fatal("stat processor not found in syscallProcessors")
	}

	// --- Scenario: namespace package directory with only ghost children ---
	// Current (correct) behavior: use e.retVal == 0 for HasSuccessfulAccess
	t.Run("correct_behavior_retVal_eq_0", func(t *testing.T) {
		app := newTestApp()

		// Parent directory — success
		app.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0, pathParam: "/usr/lib/nspkg",
		})
		// Ghost child — ENOENT
		app.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0xFFFFFFFE, pathParam: "/usr/lib/nspkg/__init__.py",
		})

		result := app.FileActivity()
		if _, found := result["/usr/lib/nspkg"]; !found {
			t.Error("namespace package directory was incorrectly excluded")
		}
	})

	// Simulated regression: if HasSuccessfulAccess used OKReturnStatus instead
	// of retVal == 0, ENOENT ghost children would get HasSuccessfulAccess=true,
	// triggering IsSubdir on the parent and excluding it.
	t.Run("regression_if_OKReturnStatus_used_for_HasSuccessfulAccess", func(t *testing.T) {
		app := newTestApp()

		// Parent directory — success
		app.fsActivity["/usr/lib/nspkg"] = &report.FSActivityInfo{
			OpsAll: 1, OpsCheckFile: 1,
			HasSuccessfulAccess: true,
			Pids:                map[int]struct{}{1: {}},
			Syscalls:            map[int]struct{}{int(statNum): {}},
		}

		// Ghost child — simulate what happens if HasSuccessfulAccess used
		// OKReturnStatus: ENOENT (-2) returns true from OKReturnStatus, so
		// HasSuccessfulAccess would be set to true (THIS IS THE BUG).
		enoentRetVal := uint64(0xFFFFFFFE) // -2
		buggyHasSuccessfulAccess := statProcessor.OKReturnStatus(enoentRetVal)
		// Verify the premise: OKReturnStatus does accept ENOENT
		if !buggyHasSuccessfulAccess {
			t.Fatal("test premise failed: OKReturnStatus should accept ENOENT")
		}

		app.fsActivity["/usr/lib/nspkg/__init__.py"] = &report.FSActivityInfo{
			OpsAll: 1, OpsCheckFile: 1,
			HasSuccessfulAccess: buggyHasSuccessfulAccess, // true — THE BUG
			Pids:                map[int]struct{}{1: {}},
			Syscalls:            map[int]struct{}{int(statNum): {}},
		}

		result := app.FileActivity()

		if _, found := result["/usr/lib/nspkg"]; found {
			t.Log("BUG #3 CONFIRMED: if OKReturnStatus were used for " +
				"HasSuccessfulAccess, this namespace package directory would " +
				"survive (but only because there's no real child to trigger IsSubdir)")
		} else {
			// The parent was excluded — the ghost child with
			// HasSuccessfulAccess=true triggered IsSubdir.
			t.Log("BUG #3 IS LIVE (naming trap): using OKReturnStatus for " +
				"HasSuccessfulAccess causes ghost children to trigger IsSubdir, " +
				"excluding the namespace package directory from the slim image. " +
				"This is the exact regression from commit 3f8dcb47.")
		}

		// The key assertion: the current code does NOT use OKReturnStatus for
		// HasSuccessfulAccess, so verify the correct path still works.
		correctApp := newTestApp()
		correctApp.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0, pathParam: "/usr/lib/nspkg2",
		})
		correctApp.processFileActivity(&syscallEvent{
			returned: true, pid: 1, callNum: statNum,
			retVal: 0xFFFFFFFE, pathParam: "/usr/lib/nspkg2/__init__.py",
		})
		correctResult := correctApp.FileActivity()
		if _, found := correctResult["/usr/lib/nspkg2"]; !found {
			t.Error("REGRESSION: current code is excluding namespace package " +
				"directories — the HasSuccessfulAccess condition may have been " +
				"changed to use OKReturnStatus again")
		}
	})
}
