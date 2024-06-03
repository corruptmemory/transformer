package blkidutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"transformer/util"
)

var BlkIdLayoutRe = regexp.MustCompile(`([a-zA-Z]+)="([-a-zA-Z0-9_]+)"`)

type BlkIdPartitionInfo struct {
	Partition string
	UUID      string
	PartUUID  string
	Type      string
}

func (b *BlkIdPartitionInfo) PartitionNumber() (int, error) {
	for i := len(b.Partition) - 1; i >= 0; i-- {
		if b.Partition[i] < '0' || b.Partition[i] > '9' {
			return strconv.Atoi(b.Partition[i+1:])
		}
	}
	return 0, fmt.Errorf("error parsing partition number from %s", b.Partition)
}

type BlkIdLayout struct {
	Partitions []BlkIdPartitionInfo
}

func ParseBlkIdPartitionInfo(partition, line string) (result BlkIdPartitionInfo, err error) {
	result.Partition = partition
	matches := BlkIdLayoutRe.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) == 3 {
			switch match[1] {
			case "UUID":
				result.UUID = match[2]
			case "PARTUUID":
				result.PartUUID = match[2]
			case "TYPE":
				result.Type = match[2]
			}
		}
	}
	return result, nil
}

func ParseBlkIdLayout(device, in string) (result *BlkIdLayout, err error) {
	layout := &BlkIdLayout{}
	blkRe, err := regexp.Compile("(" + device + "p[0-9]+):")
	if err != nil {
		return nil, err
	}
	lines := util.RemoveEmptySplits(util.SplitLines(in), true)
	for _, line := range lines {
		matched := blkRe.FindStringSubmatch(line)
		if len(matched) > 0 {
			withoutMatch := strings.TrimSpace(blkRe.ReplaceAllString(line, ""))
			pi, err := ParseBlkIdPartitionInfo(matched[1], withoutMatch)
			if err != nil {
				return nil, err
			}
			layout.Partitions = append(layout.Partitions, pi)
		}
	}
	return layout, nil
}
