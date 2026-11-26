package dockerimage

import (
	"archive/tar"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestChangeTypeStringAndJSON(t *testing.T) {
	cases := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeUnknown, "U"},
		{ChangeDelete, "D"},
		{ChangeAdd, "A"},
		{ChangeModify, "M"},
	}

	for _, tc := range cases {
		if got := tc.ct.String(); got != tc.want {
			t.Errorf("ChangeType(%d).String() = %q, want %q", tc.ct, got, tc.want)
		}

		raw, err := json.Marshal(tc.ct)
		if err != nil {
			t.Fatalf("json.Marshal(%d) error: %v", tc.ct, err)
		}
		if wantJSON := `"` + tc.want + `"`; string(raw) != wantJSON {
			t.Errorf("json.Marshal(%d) = %s, want %s", tc.ct, raw, wantJSON)
		}

		var decoded ChangeType
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%s) error: %v", raw, err)
		}
		if decoded != tc.ct {
			t.Errorf("json.Unmarshal(%s) = %d, want %d", raw, decoded, tc.ct)
		}
	}

	if got := ChangeType(99).String(); got != "U" {
		t.Errorf("unknown ChangeType.String() = %q, want %q", got, "U")
	}

	raw, err := json.Marshal(ChangeType(99))
	if err != nil {
		t.Fatalf("json.Marshal(99) error: %v", err)
	}
	if string(raw) != `"O"` {
		t.Errorf("json.Marshal(99) = %s, want %q", raw, `"O"`)
	}

	var decoded ChangeType
	if err := json.Unmarshal([]byte(`"Z"`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(\"Z\") error: %v", err)
	}
	if decoded != ChangeUnknown {
		t.Errorf("json.Unmarshal(\"Z\") = %d, want ChangeUnknown", decoded)
	}
}

func TestLinkTargetToFullPath(t *testing.T) {
	cases := []struct {
		fullPath string
		target   string
		want     string
	}{
		{"/usr/bin/sh", "/bin/busybox", "/bin/busybox"},
		{"/usr/bin/sh", ".", ""},
		{"/usr/bin/sh", "dash", "/usr/bin/dash"},
		{"/usr/lib/libfoo.so", "../lib64/libfoo.so.1", "/usr/lib64/libfoo.so.1"},
	}

	for _, tc := range cases {
		if got := linkTargetToFullPath(tc.fullPath, tc.target); got != tc.want {
			t.Errorf("linkTargetToFullPath(%q, %q) = %q, want %q", tc.fullPath, tc.target, got, tc.want)
		}
	}
}

func TestHashes(t *testing.T) {
	input := "mintoolkit"
	bytesHash := getBytesHash([]byte(input))
	streamHash, err := getStreamHash(strings.NewReader(input))
	if err != nil {
		t.Fatalf("getStreamHash error: %v", err)
	}
	if bytesHash != streamHash {
		t.Errorf("getBytesHash=%q != getStreamHash=%q", bytesHash, streamHash)
	}
	if len(bytesHash) != 40 {
		t.Errorf("expected 40-char hex sha1, got %d chars (%q)", len(bytesHash), bytesHash)
	}
}

func TestIsValidArchitectureAndPlatform(t *testing.T) {
	if !IsValidArchitecture(ArchAMD64, "") {
		t.Error("expected amd64 with empty variant to be valid")
	}
	if !IsValidArchitecture(ArchARM, "v7") {
		t.Error("expected arm v7 to be valid")
	}
	if IsValidArchitecture(ArchAMD64, "v8") {
		t.Error("expected amd64 v8 to be invalid")
	}
	if IsValidArchitecture("mips", "") {
		t.Error("expected unknown arch to be invalid")
	}

	if !IsValidPlatform(OSLinux, ArchAMD64) {
		t.Error("expected linux/amd64 to be valid")
	}
	if !IsValidPlatform(OSLinux, ArchARM64) {
		t.Error("expected linux/arm64 to be valid")
	}
	if IsValidPlatform("windows", ArchAMD64) {
		t.Error("expected windows/amd64 to be invalid")
	}
	if IsValidPlatform(OSLinux, "sparc") {
		t.Error("expected linux/sparc to be invalid")
	}
}

func TestWhiteoutHelpers(t *testing.T) {
	if !IsDeletedFileObject("etc/.wh.hosts") {
		t.Error("expected etc/.wh.hosts to be detected as deleted")
	}
	if IsDeletedFileObject("etc/hosts") {
		t.Error("expected etc/hosts not to be detected as deleted")
	}

	p, deleted, isOpaque, err := NormalizeFileObjectLayerPath("etc/.wh..wh..opq")
	if err != nil || !deleted || !isOpaque || p != "etc/*" {
		t.Errorf("opaque normalize got (%q, %v, %v, %v), want (\"etc/*\", true, true, nil)", p, deleted, isOpaque, err)
	}

	p, deleted, isOpaque, err = NormalizeFileObjectLayerPath("etc/.wh.passwd")
	if err != nil || !deleted || isOpaque || p != "etc/passwd" {
		t.Errorf("whiteout normalize got (%q, %v, %v, %v), want (\"etc/passwd\", true, false, nil)", p, deleted, isOpaque, err)
	}

	p, deleted, isOpaque, err = NormalizeFileObjectLayerPath("etc/passwd")
	if err != nil || deleted || isOpaque || p != "etc/passwd" {
		t.Errorf("normal normalize got (%q, %v, %v, %v), want (\"etc/passwd\", false, false, nil)", p, deleted, isOpaque, err)
	}
}

func TestTarHelpersAndMediaTypes(t *testing.T) {
	if got := ObjectTypeFromTarType(tar.TypeDir); got != DirType {
		t.Errorf("ObjectTypeFromTarType(TypeDir) = %q, want %q", got, DirType)
	}
	if got := ObjectTypeFromTarType(tar.TypeReg); got != FileType {
		t.Errorf("ObjectTypeFromTarType(TypeReg) = %q, want %q", got, FileType)
	}
	if got := ObjectTypeFromTarType(tar.TypeSymlink); got != SymlinkType {
		t.Errorf("ObjectTypeFromTarType(TypeSymlink) = %q, want %q", got, SymlinkType)
	}
	if got := ObjectTypeFromTarType(tar.TypeLink); got != HardlinkType {
		t.Errorf("ObjectTypeFromTarType(TypeLink) = %q, want %q", got, HardlinkType)
	}

	if got := TarHeaderTypeName(tar.TypeFifo); got != "tar.TypeFifo" {
		t.Errorf("TarHeaderTypeName(TypeFifo) = %q, want \"tar.TypeFifo\"", got)
	}

	if !IsLayerMediaType(types.OCILayer) {
		t.Error("expected OCILayer to be recognized as layer media type")
	}
	if !IsLayerMediaType(types.DockerLayer) {
		t.Error("expected DockerLayer to be recognized as layer media type")
	}
	if IsLayerMediaType(types.OCIConfigJSON) {
		t.Error("expected OCIConfigJSON not to be recognized as layer media type")
	}
}
