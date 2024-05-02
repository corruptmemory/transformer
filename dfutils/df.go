package dfutils

import (
	"fmt"
	"strconv"
	"strings"
	"transformer/util"
)

const BlockSize = 1024

type DiskUsage struct {
	Filesystem   string
	SizeInBlocks uint64
	Used         uint64
	MountedOn    string
}

func (d *DiskUsage) validate() error {
	if d.Filesystem == "" {
		return fmt.Errorf("filesystem is empty")
	}
	if d.SizeInBlocks == 0 {
		return fmt.Errorf("size in blocks is 0")
	}
	if d.MountedOn == "" {
		return fmt.Errorf("mounted on is empty")
	}
	return nil
}

func (d *DiskUsage) TotalSizeInBytes() uint64 {
	return d.SizeInBlocks * BlockSize
}

func (d *DiskUsage) UsedSizeInBytes() uint64 {
	return d.Used * BlockSize
}

func (d *DiskUsage) AvailableSizeInBytes() uint64 {
	return d.Available() * BlockSize
}

func (d *DiskUsage) PercentageUsed() float32 {
	return float32(d.Used) / float32(d.SizeInBlocks) * 100
}

func (d *DiskUsage) Available() uint64 {
	return d.SizeInBlocks - d.Used
}

func (d *DiskUsage) PercentageAvailable() float32 {
	return float32(d.Available()) / float32(d.SizeInBlocks) * 100
}

type DiskUsageList []DiskUsage

func (d DiskUsageList) IsEmpty() bool {
	return len(d) == 0
}

func (d DiskUsageList) FindByFilesystem(in string) (DiskUsage, bool) {
	for _, d := range d {
		if d.Filesystem == in {
			return d, true
		}
	}
	return DiskUsage{}, false
}

func (d DiskUsageList) FindByMountedOn(in string) (DiskUsage, bool) {
	for _, d := range d {
		if d.MountedOn == in {
			return d, true
		}
	}
	return DiskUsage{}, false
}

func parseDFLine(line string) (result DiskUsage, err error) {
	fields := util.RemoveEmptySplits(strings.Split(line, " "), true)
	if len(fields) != 6 {
		return DiskUsage{}, fmt.Errorf("error parsing df line: %s", line)
	}
	result.Filesystem = fields[0]
	sb, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return DiskUsage{}, err
	}
	result.SizeInBlocks = uint64(sb)
	u, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return DiskUsage{}, err
	}
	result.Used = uint64(u)
	result.MountedOn = fields[5]
	err = result.validate()
	if err != nil {
		return DiskUsage{}, err
	}
	return result, nil
}

func ParseDFOutput(in string) (result DiskUsageList, err error) {
	lines := util.RemoveEmptySplits(util.SplitLines(in), true)
	if len(lines) == 0 {
		return nil, nil
	}
	if strings.HasPrefix(lines[0], "Filesystem") {
		lines = lines[1:]
	}
	for _, line := range lines {
		d, err := parseDFLine(line)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, nil
}
