package fsutil

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/mintoolkit/mint/pkg/pdiscover"
	"github.com/mintoolkit/mint/pkg/util/errutil"

	"github.com/bmatcuk/doublestar"
	log "github.com/sirupsen/logrus"
)

// File permission bits (execute bits only)
const (
	FilePermUserExe  = 0100
	FilePermGroupExe = 0010
	FilePermOtherExe = 0001
)

// Native FileMode special bits mask
const FMSpecialBits = os.ModeSticky | os.ModeSetgid | os.ModeSetuid

// Native FileModes for extra flags
const (
	FMSticky = 01000
	FMSetgid = 02000
	FMSetuid = 04000
)

// Native FileMode bits for extra flags
const (
	BitSticky = 1
	BitSetgid = 2
	BitSetuid = 4
)

// Directory and file related errors
var (
	ErrNoFileData                = errors.New("no file data")
	ErrNoSrcDir                  = errors.New("no source directory path")
	ErrNoDstDir                  = errors.New("no destination directory path")
	ErrSameDir                   = errors.New("source and destination directories are the same")
	ErrSrcDirNotExist            = errors.New("source directory doesn't exist")
	ErrSrcNotDir                 = errors.New("source is not a directory")
	ErrSrcNotRegularFile         = errors.New("source is not a regular file")
	ErrUnsupportedFileObjectType = errors.New("unsupported file object type")
)

// FileModeExtraUnix2Go converts the standard unix filemode for the extra flags to the Go version
func FileModeExtraUnix2Go(mode uint32) os.FileMode {
	switch mode {
	case FMSticky:
		return os.ModeSticky
	case FMSetgid:
		return os.ModeSetgid
	case FMSetuid:
		return os.ModeSetuid
	}

	return 0
}

// FileModeExtraBitUnix2Go converts the standard unix filemode bit for the extra flags to the filemode in Go
func FileModeExtraBitUnix2Go(bit uint32) os.FileMode {
	switch bit {
	case BitSticky:
		return os.ModeSticky
	case BitSetgid:
		return os.ModeSetgid
	case BitSetuid:
		return os.ModeSetuid
	}

	return 0
}

// FileModeExtraBitsUnix2Go converts the standard unix filemode bits for the extra flags to the filemode flags in Go
func FileModeExtraBitsUnix2Go(bits uint32) os.FileMode {
	var mode os.FileMode

	if bits&BitSticky != 0 {
		mode |= os.ModeSticky
	}

	if bits&BitSetgid != 0 {
		mode |= os.ModeSetgid
	}

	if bits&BitSetuid != 0 {
		mode |= os.ModeSetuid
	}

	return mode
}

// FileModeIsSticky checks if FileMode has the sticky bit set
func FileModeIsSticky(mode os.FileMode) bool {
	if mode&os.ModeSticky != 0 {
		return true
	}

	return false
}

// FileModeIsSetgid checks if FileMode has the setgid bit set
func FileModeIsSetgid(mode os.FileMode) bool {
	if mode&os.ModeSetgid != 0 {
		return true
	}

	return false
}

// FileModeIsSetuid checks if FileMode has the setuid bit set
func FileModeIsSetuid(mode os.FileMode) bool {
	if mode&os.ModeSetuid != 0 {
		return true
	}

	return false
}

const (
	rootStateKey           = ".mint-state"
	releasesStateKey       = "releases"
	imageStateBaseKey      = "images"
	imageStateArtifactsKey = "artifacts"
	stateArtifactsPerms    = 0777
	releaseArtifactsPerms  = 0740
)

var badInstallPaths = [...]string{
	"/usr/local/bin",
	"/usr/local/sbin",
	"/usr/bin",
	"/usr/sbin",
	"/bin",
	"/sbin",
}

const (
	tmpPath        = "/tmp"
	stateTmpPath   = "/tmp/mint-state"
	sensorFileName = "mint-sensor"
)

// AccessInfo provides the file object access properties
type AccessInfo struct {
	Flags     os.FileMode
	PermsOnly bool
	UID       int
	GID       int
}

func NewAccessInfo() *AccessInfo {
	return &AccessInfo{
		Flags: 0,
		UID:   -1,
		GID:   -1,
	}
}

// Remove removes the artifacts generated during the current application execution
func Remove(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}

// Touch creates the target file or updates its timestamp
func Touch(target string) error {
	targetDirPath := FileDir(target)
	if _, err := os.Stat(targetDirPath); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(targetDirPath, 0777)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	tf, err := os.OpenFile(target, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	tf.Close()

	tnow := time.Now().UTC()
	err = os.Chtimes(target, tnow, tnow)
	if err != nil {
		return err
	}

	return nil
}

// Exists returns true if the target file system object exists
func Exists(target string) bool {
	if _, err := os.Stat(target); err != nil {
		return false
	}

	return true
}

// DirExists returns true if the target exists and it's a directory
func DirExists(target string) bool {
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return true
	}

	return false
}

// IsDir returns true if the target file system object is a directory
func IsDir(target string) bool {
	info, err := os.Stat(target)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// IsRegularFile returns true if the target file system object is a regular file
func IsRegularFile(target string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

// IsSymlink returns true if the target file system object is a symlink
func IsSymlink(target string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeSymlink) == os.ModeSymlink
}

// IsTarFile returns true if the target file system object is a tar archive
func IsTarFile(target string) bool {
	tf, err := os.Open(target)
	if err != nil {
		log.Debugf("fsutil.IsTarFile(%s): error - %v", target, err)
		return false
	}

	defer tf.Close()
	tr := tar.NewReader(tf)
	_, err = tr.Next()
	if err != nil {
		log.Debugf("fsutil.IsTarFile(%s): error - %v", target, err)
		return false
	}

	return true
}

func HasReadAccess(dst string) (bool, error) {
	//a placeholder
	return true, nil
}

func HasWriteAccess(dst string) (bool, error) {
	//a placeholder
	return true, nil
}

// SetAccess updates the access permissions on the destination
func SetAccess(dst string, access *AccessInfo) error {
	//a placeholder
	return nil
}

// CopyFile copies the source file system object to the desired destination
func CopyFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopyFile(%v,%v,%v,%v)", clone, src, dst, makeDir)
	//a placeholder
	return nil
}

// CopySymlinkFile copies a symlink file
func CopySymlinkFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopySymlinkFile(%v,%v,%v)", src, dst, makeDir)
	//a placeholder
	return nil
}

// CopyRegularFile copies a regular file
func CopyRegularFile(clone bool, src, dst string, makeDir bool) error {
	log.Debugf("CopyRegularFile(%v,%v,%v,%v)", clone, src, dst, makeDir)
	//a placeholder
	return nil
}

// CopyAndObfuscateFile copies a regular file and performs basic file reference obfuscation
func CopyAndObfuscateFile(
	clone bool,
	src string,
	dst string,
	makeDir bool) error {
	log.Debugf("CopyAndObfuscateFile(%v,%v,%v,%v)", clone, src, dst, makeDir)
	//a placeholder
	return nil
}

// AppendToFile appends the provided data to the target file
func AppendToFile(target string, data []byte, preserveTimes bool) error {
	//a placeholder
	return nil
}

type ReplaceInfo struct {
	PathSuffix   string
	IsMatchRegex string
	Match        string
	Replace      string
}

// ReplaceFileData replaces the selected file bytes with the caller provided bytes
func ReplaceFileData(target string, replace []ReplaceInfo, preserveTimes bool) error {
	//a placeholder
	return nil
}

type DataUpdaterFn func(target string, data []byte) ([]byte, error)

// UpdateFileData updates all file data in target file using the updater function
func UpdateFileData(target string, updater DataUpdaterFn, preserveTimes bool) error {
	//a placeholder
	return nil
}

// CopyDirOnly copies a directory without any files
func CopyDirOnly(clone bool, src, dst string) error {
	log.Debugf("CopyDirOnly(%v,%v,%v)", clone, src, dst)
	//a placeholder
	return nil
}

// CopyDir copies a directory
func CopyDir(clone bool,
	src string,
	dst string,
	copyRelPath bool,
	skipErrors bool,
	excludePatterns []string,
	ignoreDirNames map[string]struct{},
	ignoreFileNames map[string]struct{}) (error, []error) {
	log.Debugf("CopyDir(%v,%v,%v,%v,%#v,...)", src, dst, copyRelPath, skipErrors, excludePatterns)
	//a placeholder
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////

func FileMode(fileName string) string {
	//a placeholder
	return ""
}

// ExeDir returns the directory information for the application
func ExeDir() string {
	//a placeholder
	return ""
}

// FileDir returns the directory information for the given file
func FileDir(fileName string) string {
	abs, err := filepath.Abs(fileName)
	errutil.FailOn(err)
	return filepath.Dir(abs)
}

// PreparePostUpdateStateDir ensures that the updated sensor is copied to the state directory if necessary
func PreparePostUpdateStateDir(statePrefix string) {
	log.Debugf("PreparePostUpdateStateDir(%v)", statePrefix)
	//a placeholder
}

// ResolveImageStateBasePath resolves the base path for the state path
func ResolveImageStateBasePath(statePrefix string) string {
	log.Debugf("ResolveImageStateBasePath(%s)", statePrefix)
	//a placeholder
	return statePrefix
}

// PrepareImageStateDirs ensures that the required application directories exist
func PrepareImageStateDirs(statePrefix, imageID string) (string, string, string, string) {
	//a placeholder
	return "", "", "", ""
}

// PrepareReleaseStateDirs ensures that the required app release directories exist
func PrepareReleaseStateDirs(statePrefix, version string) (string, string) {
	//prepares the app release directories (used to update the app binaries)
	//creating the root state directory if it doesn't exist
	log.Debugf("PrepareReleaseStateDirs(%v,%v)", statePrefix, version)
	//a placeholder
	return "", ""
}

///////////////////////////////////////////////////////////////////////////////

func ArchiveFiles(afname string,
	files []string,
	removePath bool,
	addPrefix string) error {
	//a placeholder
	return nil
}

func ArchiveDir(afname string,
	d2aname string,
	trimPrefix string,
	addPrefix string) error {
	//a placeholder
	return nil
}

///////////////////////////////////////////////////////////////////////////////

// UpdateFileTimes updates the atime and mtime timestamps on the target file
func UpdateFileTimes(target string, atime, mtime syscall.Timespec) error {
	//a placeholder
	return nil
}

// UpdateSymlinkTimes updates the atime and mtime timestamps on the target symlink
func UpdateSymlinkTimes(target string, atime, mtime syscall.Timespec) error {
	//a placeholder
	return nil
}

// LoadStructFromFile creates a struct from a file
func LoadStructFromFile(filePath string, out interface{}) error {
	//a placeholder
	return nil
}
