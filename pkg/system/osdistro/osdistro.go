package osdistro

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"log"
)

// Data file names
const (
	OSReleaseFile    = "/etc/os-release"
	OSReleaseFileNew = "/usr/lib/os-release"
	LSBReleaseFile   = "/etc/lsb-release"
	IssueFile        = "/etc/issue"
	IssueNetFile     = "/etc/issue.net"

	DebianVersionFile         = "/etc/debian_version"
	RedhatReleaseFile         = "/etc/redhat-release"
	CentosReleaseFile         = "/etc/centos-release"
	FedoraReleaseFile         = "/etc/fedora-release"
	OracleReleaseFile         = "/etc/oracle-release"
	SystemReleaseFile         = "/etc/system-release"     // Redhat/Centos/Fedora/Rocky/Alma/AmazonLinux/Oracle
	SystemReleaseCPEFile      = "/etc/system-release-cpe" // Redhat/Centos/Fedora/Rocky/Alma/AmazonLinux/Oracle
	RockyReleaseFile          = "/etc/rocky-release"
	RockyReleaseUpstream      = "/etc/rocky-release-upstream"
	AmazonLinuxReleaseFile    = "/etc/amazon-linux-release"
	AmazonLinuxReleaseCPEFile = "/etc/amazon-linux-release-cpe"
	AlpineReleaseFile         = "/etc/alpine-release" //only version number
	ArchReleaseFile           = "/etc/arch-release"   //can be empty
	SuseReleaseFile           = "/etc/SuSE-release"   //depricated
	SuseBrandFile             = "/etc/SUSE-brand"
	GentooReleaseFile         = "/etc/gentoo-release"
	SlackwareVersionFile      = "/etc/slackware-version"
	NixOSInfoFile             = "/etc/NIXOS"
)

// Linux distro IDs and names
const (
	DebianID   = "debian"
	DebianName = "Debian GNU/Linux"

	UbuntuID   = "ubuntu"
	UbuntuName = "Ubuntu"

	FedoraID   = "fedora"
	FedoraName = "Fedora Linux"

	RedhatID   = "rhel"
	RedhatName = "Red Hat Enterprise Linux"

	CentosID   = "centos"
	CentosName = "CentOS Linux"

	RockyLinuxID   = "rocky"
	RockyLinuxName = "Rocky Linux"

	AlmaLinuxID   = "almalinux" //can be in double quotes
	AlmaLinuxName = "AlmaLinux"

	AmazonLinuxID   = "amzn" //can be in double quotes
	AmazonLinuxName = "Amazon Linux"

	OracleLinuxID   = "ol" //can be in double quotes
	OracleLinuxName = "Oracle Linux Server"

	AlpineID   = "alpine"
	AlpineName = "Alpine Linux"

	ArchID   = "arch"
	ArchName = "Arch Linux"

	WolfiID   = "wolfi" //base wolfi images and Chainguard images
	WolfiName = "Wolfi"

	ClearLinuxID   = "clear-linux-os"
	ClearLinuxName = "Clear Linux OS"

	GentooID   = "gentoo"
	GentooName = "Gentoo"

	SlackwareID   = "slackware"
	SlackwareName = "Slackware"

	OpenSuseID   = "opensuse" //need to do a "contains" check (e.g., to match "opensuse-leap")
	OpenSuseName = "openSUSE"

	SlesID   = "sles" //could be in double quotes (need to strip double quotes if present)
	SlesName = "SLES" //SUSE Linux Enterprise Server

	BottlerocketID   = "bottlerocket"
	BottlerocketName = "Bottlerocket"

	FlatcarID   = "flatcar"
	FlatcarName = "Flatcar Container Linux by Kinvolk"

	CoreosID   = "coreos"
	CoreosName = "Container Linux by CoreOS"

	NixosID   = "nixos"
	NixosName = "NixOS"
)

var DistrosByID = map[string]string{
	DebianID:       DebianName,
	UbuntuID:       UbuntuName,
	FedoraID:       FedoraName,
	RedhatID:       RedhatName,
	CentosID:       CentosName,
	RockyLinuxID:   RockyLinuxName,
	AlmaLinuxID:    AlmaLinuxName,
	AmazonLinuxID:  AmazonLinuxName,
	OracleLinuxID:  OracleLinuxName,
	AlpineID:       AlpineName,
	ArchID:         ArchName,
	WolfiID:        WolfiName,
	ClearLinuxID:   ClearLinuxName,
	GentooID:       GentooName,
	SlackwareID:    SlackwareName,
	OpenSuseID:     OpenSuseName,
	SlesID:         SlesName,
	BottlerocketID: BottlerocketName,
	FlatcarID:      FlatcarName,
	CoreosID:       CoreosName,
	NixosID:        NixosName,
}

func IsKnownDistro(id string) bool {
	_, found := DistrosByID[id]
	if found {
		return true
	}

	return false
}

// A map of distro info files (todo: add a list of distros where the files are used)
var AllFiles = map[string]struct{}{
	OSReleaseFile:             {},
	OSReleaseFileNew:          {},
	IssueFile:                 {},
	IssueNetFile:              {},
	LSBReleaseFile:            {},
	DebianVersionFile:         {},
	RedhatReleaseFile:         {},
	CentosReleaseFile:         {},
	FedoraReleaseFile:         {},
	OracleReleaseFile:         {},
	SystemReleaseFile:         {},
	SystemReleaseCPEFile:      {},
	RockyReleaseFile:          {},
	RockyReleaseUpstream:      {},
	AmazonLinuxReleaseFile:    {},
	AmazonLinuxReleaseCPEFile: {},
	AlpineReleaseFile:         {},
	ArchReleaseFile:           {},
	SuseReleaseFile:           {},
	SuseBrandFile:             {},
	GentooReleaseFile:         {},
	SlackwareVersionFile:      {},
	NixOSInfoFile:             {},
}

func IsOSReleaseFile(filePath string) bool {
	switch filePath {
	case OSReleaseFile, OSReleaseFileNew:
		return true
	default:
		return false
	}
}

func IsDataFile(filePath string) bool {
	_, found := AllFiles[filePath]
	if found {
		return true
	}

	return false
}

//NOTE:
//copied from https://github.com/docker/machine/blob/master/libmachine/provision/os_release.go

// The /etc/os-release file contains operating system identification data
// See http://www.freedesktop.org/software/systemd/man/os-release.html for more details

// OsRelease reflects values in /etc/os-release
// Values in this struct must always be string
// or the reflection will not work properly.
type OsRelease struct {
	AnsiColor    string `osr:"ANSI_COLOR"`
	Name         string `osr:"NAME"`
	Version      string `osr:"VERSION"`
	Variant      string `osr:"VARIANT"`
	VariantID    string `osr:"VARIANT_ID"`
	ID           string `osr:"ID"`
	IDLike       string `osr:"ID_LIKE"`
	PrettyName   string `osr:"PRETTY_NAME"`
	VersionID    string `osr:"VERSION_ID"`
	HomeURL      string `osr:"HOME_URL"`
	SupportURL   string `osr:"SUPPORT_URL"`
	BugReportURL string `osr:"BUG_REPORT_URL"`
}

func stripQuotes(val string) string {
	if len(val) > 0 && val[0] == '"' {
		return val[1 : len(val)-1]
	}
	return val
}

func (osr *OsRelease) setIfPossible(key, val string) error {
	v := reflect.ValueOf(osr).Elem()
	for i := 0; i < v.NumField(); i++ {
		fieldValue := v.Field(i)
		fieldType := v.Type().Field(i)
		originalName := fieldType.Tag.Get("osr")
		if key == originalName && fieldValue.Kind() == reflect.String {
			fieldValue.SetString(val)
			return nil
		}
	}
	return fmt.Errorf("Couldn't set key %s, no corresponding struct field found", key)
}

func parseLine(osrLine string) (string, string, error) {
	if osrLine == "" {
		return "", "", nil
	}

	vals := strings.Split(osrLine, "=")
	if len(vals) != 2 {
		return "", "", fmt.Errorf("Expected %s to split by '=' char into two strings, instead got %d strings", osrLine, len(vals))
	}
	key := vals[0]
	val := stripQuotes(vals[1])
	return key, val, nil
}

func (osr *OsRelease) ParseOsRelease(osReleaseContents []byte) error {
	r := bytes.NewReader(osReleaseContents)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		key, val, err := parseLine(scanner.Text())
		if err != nil {
			log.Printf("Warning: got an invalid line error parsing /etc/os-release: %s", err)
			continue
		}
		if err := osr.setIfPossible(key, val); err != nil {
			//log.Printf("Info: %s\n",err)
			//note: ignore, printing these messages causes more confusion...
		}
	}
	return nil
}

func NewOsRelease(contents []byte) (*OsRelease, error) {
	osr := &OsRelease{}
	if err := osr.ParseOsRelease(contents); err != nil {
		return nil, err
	}
	return osr, nil
}

//TODO: Add LSB Release and Issue file parsers

/*
cat /etc/os-release

NAME="Ubuntu"
VERSION="14.04.5 LTS, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 14.04.5 LTS"
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"

cat /etc/os-release

NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.5.2
PRETTY_NAME="Alpine Linux v3.5"
HOME_URL="http://alpinelinux.org"
BUG_REPORT_URL="http://bugs.alpinelinux.org"


cat /etc/os-release

NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"
CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"

cat /etc/os-release

PRETTY_NAME="Debian GNU/Linux 9 (stretch)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (stretch)"
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
*/

//more os-release data:
//https://github.com/chef/os_release
