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

func (s *SFPartitionInfo) String() string {
	base := fmt.Sprintf("%s : start=%12d, size=%12d, type=%s",
		s.Partition, s.Start, s.Size, s.TypeUUID)
	if s.PartUUID != "" {
		base += fmt.Sprintf(", uuid=%s", s.PartUUID)
	}
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
	return nil
}

type SFDiskLayout struct {
	Label      string
	LabelID    string
	Device     string
	Unit       string
	FirstLBA   int
	LastLBA    int
	SectorSize int
	Partitions []SFPartitionInfo
}

func (s *SFDiskLayout) AddPartition(start, size int, typeUUID, partUUID, name, attrs string) (err error) {
	end := start + size
	initialFreeSpaceStart := s.FirstLBA
	var initialFreeSpaceEnd int
	if initialFreeSpaceStart < 2048 {
		initialFreeSpaceStart = 2048
	}
	if len(s.Partitions) > 0 {
		initialFreeSpaceEnd = s.Partitions[0].Start
	} else {
		initialFreeSpaceEnd = s.LastLBA
	}
	nextAvailablePartition := func() (partition int, err error) {
		for i := 1; i < 100; i++ {
			found := false
			for _, part := range s.Partitions {
				if part.Partition == s.Device+"p"+strconv.Itoa(i) {
					found = true
					break
				}
			}
			if !found {
				return i, nil
			}
		}
		return 0, fmt.Errorf("error: no available partition number")
	}
	followingFreeInfo := func(partIdx int) (start int, size int, err error) {
		switch {
		case partIdx < len(s.Partitions)-1:
			fst := s.Partitions[partIdx].Start + s.Partitions[partIdx].Size
			fse := s.Partitions[partIdx+1].Start
			fs := fse - fst
			return fst, fs, nil
		case partIdx == len(s.Partitions)-1:
			fst := s.Partitions[partIdx].Start + s.Partitions[partIdx].Size
			fse := s.LastLBA
			fs := fse - fst
			return fst, fs, nil
		default:
			return 0, 0, fmt.Errorf("error: partIdx out of range")
		}
	}
	if start < initialFreeSpaceStart {
		return fmt.Errorf("error: partition defined before initial free space")
	}
	if len(s.Partitions) == 0 {
		if end > initialFreeSpaceEnd {
			return fmt.Errorf("error: partition does not fit in free space")
		}
		s.Partitions = append(s.Partitions, SFPartitionInfo{
			Partition: s.Device + "p1",
			Start:     start,
			Size:      size,
			TypeUUID:  typeUUID,
			PartUUID:  partUUID,
			Name:      name,
			Attrs:     attrs,
		})
		return nil
	}
	if start < s.Partitions[0].Start {
		if start+size > s.Partitions[0].Start {
			return fmt.Errorf("error: partition overlaps with existing partition")
		}
		pn, err := nextAvailablePartition()
		if err != nil {
			return err
		}
		s.Partitions = append(s.Partitions, SFPartitionInfo{
			Partition: s.Device + "p" + strconv.Itoa(pn),
			Start:     start,
			Size:      size,
			TypeUUID:  typeUUID,
			PartUUID:  partUUID,
			Name:      name,
			Attrs:     attrs,
		})
		s.EnsureSortedPartitions()
		return nil
	}
	for i, part := range s.Partitions {
		if start < part.Start {
			if start+size > part.Start {
				return fmt.Errorf("error: partition overlaps with existing partition")
			}
			fst, fs, err := followingFreeInfo(i - 1)
			if err != nil {
				return err
			}
			if start < fst || end > fst+fs {
				return fmt.Errorf("error: partition does not fit in free space")
			}
			pn, err := nextAvailablePartition()
			if err != nil {
				return err
			}
			s.Partitions = append(s.Partitions, SFPartitionInfo{
				Partition: s.Device + "p" + strconv.Itoa(pn),
				Start:     start,
				Size:      size,
				TypeUUID:  typeUUID,
				PartUUID:  partUUID,
				Name:      name,
				Attrs:     attrs,
			})
			s.EnsureSortedPartitions()
			return nil
		}
	}

	fst, fs, err := followingFreeInfo(len(s.Partitions) - 1)
	if err != nil {
		return err
	}
	if start < fst || end > fst+fs {
		return fmt.Errorf("error: partition does not fit in free space")
	}
	pn, err := nextAvailablePartition()
	if err != nil {
		return err
	}
	s.Partitions = append(s.Partitions, SFPartitionInfo{
		Partition: s.Device + "p" + strconv.Itoa(pn),
		Start:     start,
		Size:      size,
		TypeUUID:  typeUUID,
		PartUUID:  partUUID,
		Name:      name,
		Attrs:     attrs,
	})
	s.EnsureSortedPartitions()
	return nil
}

func (s *SFDiskLayout) DeletePartition(idx int) (err error) {
	targetPartitonName := s.Device + "p" + strconv.Itoa(idx)
	for i, part := range s.Partitions {
		if part.Partition == targetPartitonName {
			s.Partitions = append(s.Partitions[:i], s.Partitions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("error: partition not found")
}

func (s *SFDiskLayout) FindPartition(idx int) (pi *SFPartitionInfo, err error) {
	targetPartitonName := s.Device + "p" + strconv.Itoa(idx)
	for i, part := range s.Partitions {
		if part.Partition == targetPartitonName {
			return &s.Partitions[i], nil
		}
	}
	return nil, fmt.Errorf("error: partition not found")
}

func (s *SFDiskLayout) SplitPartitionEqually(idx int, partUUID, name, attrs string) (err error) {
	part, err := s.FindPartition(idx)
	if err != nil {
		return err
	}
	newSize := part.Size / 2
	if newSize < 1 {
		return fmt.Errorf("error: partition too small to split")
	}
	part.Size = newSize
	return s.AddPartition(part.Start+part.Size, newSize, part.TypeUUID, partUUID, name, attrs)
}

func (s *SFDiskLayout) String() string {
	base := fmt.Sprintf("label: %s\nlabel-id: %s\ndevice: %s\nunit: %s\nfirst-lba: %d\nlast-lba: %d\nsector-size: %d\n\n",
		s.Label, s.LabelID, s.Device, s.Unit, s.FirstLBA, s.LastLBA, s.SectorSize)
	for _, part := range s.Partitions {
		base += fmt.Sprintf("%s\n", part.String())
	}
	return base
}

func (s *SFDiskLayout) EnsureSortedPartitions() {
	slices.SortFunc(s.Partitions, func(a, b SFPartitionInfo) int {
		return a.Start - b.Start
	})
}

func (s *SFDiskLayout) validHeader() (err error) {
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

func ParseSFdiskLayout(in string) (result SFDiskLayout, err error) {
	lines := util.SplitLines(strings.TrimSpace(in))
	var i int
	var line string
headerDone:
	for i, line = range lines {
		parts := util.RemoveEmptySplits(strings.Split(line, ":"), true)
		if len(parts) > 2 {
			return SFDiskLayout{}, fmt.Errorf("error: found header line we weren't expecing: %s", line)
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
					return SFDiskLayout{}, fmt.Errorf("error parsing first-lba: %s", err)
				}
			case "last-lba":
				result.LastLBA, err = strconv.Atoi(parts[1])
				if err != nil {
					return SFDiskLayout{}, fmt.Errorf("error parsing last-lba: %s", err)
				}
			case "sector-size":
				result.SectorSize, err = strconv.Atoi(parts[1])
				if err != nil {
					return SFDiskLayout{}, fmt.Errorf("error parsing sector-size: %s", err)
				}
			}
		}
	}
	if err = result.validHeader(); err != nil {
		return SFDiskLayout{}, err
	}

	parsedLines, err := parsePartitionLines(lines[i+1:])
	if err != nil {
		return SFDiskLayout{}, err
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
