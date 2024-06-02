package sfdiskutils

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"transformer/util"
)

type SFPartitionInfo struct {
	Partition string
	Start     int
	Size      int
	TypeUUID  string
	PartUUID  string
	Name      string
	Attrs     string
}

// /dev/nvme0n1p1 : start=        2048, size=      204800, type=EBD0A0A2-B9E5-4433-87C0-68B6B72699C7, uuid=61E4439A-8668-4FE9-9371-4CD688886A8F, name="EFI system partition", attrs="GUID:63"
func (s *SFPartitionInfo) String() string {
	base := fmt.Sprintf("%s : start=%12d, size=%12d, type=%s, uuid=%s",
		s.Partition, s.Start, s.Size, s.TypeUUID, s.PartUUID)
	if s.Name != "" {
		base += fmt.Sprintf(", name=%q", s.Name)
	}
	if s.Attrs != "" {
		base += fmt.Sprintf(", attrs=%q", s.Attrs)
	}
	return base
}

func (s *SFPartitionInfo) validPartition() error {
	if s.Partition == "" {
		return fmt.Errorf("error: partition is empty")
	}
	if s.Start == 0 {
		return fmt.Errorf("error: start is empty")
	}
	if s.Size == 0 {
		return fmt.Errorf("error: size is empty")
	}
	if s.TypeUUID == "" {
		return fmt.Errorf("error: type-uuid is empty")
	}
	if s.PartUUID == "" {
		return fmt.Errorf("error: part-uuid is empty")
	}
	return nil
}

// SFdiskLayout represents the layout of a disk as reported by sfdisk.
type SFdiskLayout struct {
	Label      string
	LabelID    string
	Device     string
	Unit       string
	FirstLBA   int
	LastLBA    int
	SectorSize int
	Partitions []SFPartitionInfo
}

func (s *SFdiskLayout) String() string {
	base := fmt.Sprintf("label: %s\nlabel-id: %s\ndevice: %s\nunit: %s\nfirst-lba: %d\nlast-lba: %d\nsector-size: %d\n\n",
		s.Label, s.LabelID, s.Device, s.Unit, s.FirstLBA, s.LastLBA, s.SectorSize)
	for _, part := range s.Partitions {
		base += fmt.Sprintf("%s\n", part.String())
	}
	return base
}

func (s *SFdiskLayout) EnsureSortedPartitions() {
	slices.SortFunc(s.Partitions, func(a, b SFPartitionInfo) int {
		return a.Start - b.Start
	})
}

func (s *SFdiskLayout) validHeader() (err error) {
	if s.Label == "" {
		return fmt.Errorf("error: label is empty")
	}
	if s.LabelID == "" {
		return fmt.Errorf("error: label-id is empty")
	}
	if s.Device == "" {
		return fmt.Errorf("error: device is empty")
	}
	if s.Unit == "" {
		return fmt.Errorf("error: unit is empty")
	}
	if s.FirstLBA == 0 {
		return fmt.Errorf("error: first-lba is empty")
	}
	if s.LastLBA == 0 {
		return fmt.Errorf("error: last-lba is empty")
	}
	if s.SectorSize == 0 {
		return fmt.Errorf("error: sector-size is empty")
	}
	return nil
}

func ParseSFdiskLayout(in string) (result SFdiskLayout, err error) {
	lines := util.SplitLines(strings.TrimSpace(in))
	var i int
	var line string
headerDone:
	for i, line = range lines {
		parts := util.RemoveEmptySplits(strings.Split(line, ":"), true)
		if len(parts) > 2 {
			return SFdiskLayout{}, fmt.Errorf("error: found header line we weren't expecing: %s", line)
		}
		if len(parts) == 0 {
			break headerDone
		}
		if len(parts) == 2 {
			switch parts[0] {
			case "label":
				result.Label = parts[1]
			case "label-id":
				result.LabelID = parts[1]
			case "device":
				result.Device = parts[1]
			case "unit":
				result.Unit = parts[1]
			case "first-lba":
				result.FirstLBA, err = strconv.Atoi(parts[1])
				if err != nil {
					return SFdiskLayout{}, fmt.Errorf("error parsing first-lba: %s", err)
				}
			case "last-lba":
				result.LastLBA, err = strconv.Atoi(parts[1])
				if err != nil {
					return SFdiskLayout{}, fmt.Errorf("error parsing last-lba: %s", err)
				}
			case "sector-size":
				result.SectorSize, err = strconv.Atoi(parts[1])
				if err != nil {
					return SFdiskLayout{}, fmt.Errorf("error parsing sector-size: %s", err)
				}
			}
		}
	}
	if err = result.validHeader(); err != nil {
		return SFdiskLayout{}, err
	}

	parsedLines, err := parsePartitionLines(lines[i+1:])
	if err != nil {
		return SFdiskLayout{}, err
	}
	result.Partitions = parsedLines
	result.EnsureSortedPartitions()
	return result, nil
}

func stripQuotes(in string) string {
	if len(in) < 2 {
		return in
	}
	if in[0] == '"' && in[len(in)-1] == '"' {
		return in[1 : len(in)-1]
	}
	return in
}

func parsePartitionLines(lines []string) (result []SFPartitionInfo, err error) {
	for _, line := range lines {
		parts := util.RemoveEmptySplits(strings.Split(line, " : "), true)
		if len(parts) == 0 {
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("error: unexpected partition line: %s", line)
		}
		part := SFPartitionInfo{Partition: parts[0]}
		for _, p := range strings.Split(parts[1], ",") {
			partParts := util.RemoveEmptySplits(strings.Split(p, "="), true)
			if len(partParts) != 2 {
				return nil, fmt.Errorf("error: unexpected partition part: %s", p)
			}
			switch partParts[0] {
			case "start":
				part.Start, err = strconv.Atoi(partParts[1])
				if err != nil {
					return nil, fmt.Errorf("error parsing start: %s", err)
				}
			case "size":
				part.Size, err = strconv.Atoi(partParts[1])
				if err != nil {
					return nil, fmt.Errorf("error parsing size: %s", err)
				}
			case "type":
				part.TypeUUID = partParts[1]
			case "uuid":
				part.PartUUID = partParts[1]
			case "name":
				part.Name = stripQuotes(partParts[1])
			case "attrs":
				part.Attrs = stripQuotes(partParts[1])
			}
		}
		if err = part.validPartition(); err != nil {
			return nil, err
		}
		result = append(result, part)
	}
	return result, nil
}
