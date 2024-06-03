package commands

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"transformer/util"
)

type DiskArray struct {
	ArrayName  string `json:"arrayName"`
	DevicePath string `json:"devicePath"`
}

const ProcMdstat = "/proc/mdstat"

var DefaultFindDiskArraySources = []string{ProcMdstat}
var ErrDiskArrayNotFound = fmt.Errorf("disk array not found")

type ErrAmbiguousDiskArray struct {
	ArrayStrings []string
}

func (e ErrAmbiguousDiskArray) Error() string {
	return fmt.Sprintf("ambiguous disk array: %v", e.ArrayStrings)
}

// Personalities : [raid1] [linear] [multipath] [raid0] [raid6] [raid5] [raid4] [raid10]
// md126 : active (auto-read-only) raid1 sda[1] sdb[0]
//      468846592 blocks super external:/md127/0 [2/2] [UU]
//
// md127 : inactive sda[1](S) sdb[0](S)
//      9904 blocks super external:imsm
//
// unused devices: <none>

var lineMatcher = regexp.MustCompile(`^.+:\s*\bactive\b.+\braid1\b.+$`)

func (c *Commands) FindDiskArray(sources []string) (result DiskArray, err error) {
	return DoWithSourcesWithErrorProcessor(
		c,
		"FindDiskArray",
		ErrDiskArrayNotFound,
		sources,
		c.FindDiskArrayFromString,
		func(in error) (shouldReturn bool) {
			if errors.Is(in, ErrDiskArrayNotFound) {
				return false
			}
			return true
		})
}

func (c *Commands) FindDiskArrayFromString(s string) (result DiskArray, err error) {
	ng := util.NewGrep(lineMatcher)
	lines := ng.MatchLinesInString(s)
	if len(lines) == 0 {
		return result, ErrDiskArrayNotFound
	}
	if len(lines) > 1 {
		return result, ErrAmbiguousDiskArray{ArrayStrings: lines}
	}
	device := util.RemoveEmptySplits(strings.Split(lines[0], ":"), true)[0]
	return DiskArray{
		ArrayName:  device,
		DevicePath: fmt.Sprintf("/dev/%s", device),
	}, nil
}
