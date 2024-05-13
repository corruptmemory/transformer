package commands

import (
	"errors"

	"github.com/acobaugh/osrelease"
)

// NAME="Ubuntu"
// VERSION="20.04.6 LTS (Focal Fossa)"
// ID=ubuntu
// ID_LIKE=debian
// PRETTY_NAME="Ubuntu 20.04.6 LTS"
// VERSION_ID="20.04"
// HOME_URL="https://www.ubuntu.com/"
// SUPPORT_URL="https://help.ubuntu.com/"
// BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
// PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
// VERSION_CODENAME=focal
// UBUNTU_CODENAME=focal
//
// NAME="Arch GPLinux"
// PRETTY_NAME="Arch GPLinux"
// ID=arch
// BUILD_ID=rolling
// ANSI_COLOR="38;2;23;147;209"
// HOME_URL="https://archlinux.org/"
// DOCUMENTATION_URL="https://wiki.archlinux.org/"
// SUPPORT_URL="https://bbs.archlinux.org/"
// BUG_REPORT_URL="https://gitlab.archlinux.org/groups/archlinux/-/issues"
// PRIVACY_POLICY_URL="https://terms.archlinux.org/docs/privacy-policy/"
// LOGO=archlinux-logo

type OSRelease struct {
	Name            string `json:"name"`
	Version         string `json:"version,omitempty"`
	ID              string `json:"id"`
	PrettyName      string `json:"prettyName,omitempty"`
	VersionID       string `json:"versionID,omitempty"`
	VersionCodename string `json:"versionCodename,omitempty"`
	UbuntuCodename  string `json:"ubuntuCodename,omitempty"`
}

type ErrMissingField struct {
	Field string
}

func (e *ErrMissingField) Error() string {
	return "'os-release' file missing required field: " + e.Field
}

func missingFieldError(field string) error {
	return &ErrMissingField{
		Field: field,
	}
}

var ErrNoOSReleaseFound = errors.New("No 'os-release' file found")

const EtcOsRelease = osrelease.EtcOsRelease
const UsrLibOsRelease = osrelease.UsrLibOsRelease

var DefaultOSReleaseSources = []string{EtcOsRelease, UsrLibOsRelease}

func (c *Commands) GetOSRelease(sources []string) (result OSRelease, err error) {
	return DoWithSources(c, "GetOSRelease", ErrNoOSReleaseFound, sources, c.GetOSReleaseFromString)
}

func (c *Commands) GetOSReleaseFromString(data string) (result OSRelease, err error) {
	r, err := osrelease.ReadString(data)
	if err != nil {
		return result, err
	}
	var ok bool
	if result.Name, ok = r["NAME"]; !ok {
		return result, missingFieldError("NAME")
	}
	if result.ID, ok = r["ID"]; !ok {
		return result, missingFieldError("ID")
	}
	result.PrettyName, _ = r["PRETTY_NAME"]
	result.Version, _ = r["VERSION"]
	result.VersionID, _ = r["VERSION_ID"]
	result.VersionCodename, _ = r["VERSION_CODENAME"]
	result.UbuntuCodename, _ = r["UBUNTU_CODENAME"]
	return result, nil
}
