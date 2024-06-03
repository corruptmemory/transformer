package commands

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"transformer/blkidutils"

	"transformer/util"
)

type GenericPartitionType int

const (
	GPUnknown GenericPartitionType = iota
	GPEFI
	GPLinux
	GPLinuxSwap
)

func fromStringToGenericPartitionType(s string) GenericPartitionType {
	switch strings.ToLower(s) {
	case "efi", "efi system":
		return GPEFI
	case "linux filesystem":
		return GPLinux
	case "linux swap":
		return GPLinuxSwap
	default:
		return GPUnknown
	}
}

type FSType int

const (
	FSUnknown FSType = iota
	FSExt4
	FSCryptoLUKS
	FSSwap
	FSVFAT
	FSF2FS
)

func fromStringToFSType(s string) FSType {
	switch strings.ToLower(s) {
	case "ext4":
		return FSExt4
	case "crypto_luks":
		return FSCryptoLUKS
	case "swap":
		return FSSwap
	case "vfat":
		return FSVFAT
	case "f2fs":
		return FSF2FS
	default:
		return FSUnknown
	}
}

type PartitionInfo struct {
	PartitionNumber            int
	PartitionDevice            string
	Boot                       bool
	StartSector                int
	SectorSize                 int
	GenericPartitionTypeString string
	GenericPartitionType       GenericPartitionType
	FSTypeString               string
	FSType                     FSType
	UUID                       string
	PartUUID                   string
	HasGrubConfig              bool
	GrubCfgPath                string
	GrubCfgContents            string
	HasEPICApplication         bool
	EPICApplicationPath        string
	HasFstab                   bool
	FstabContents              string
	HasOSRelease               bool
	OSRelease                  OSRelease
}

func (p *PartitionInfo) updateWithBlkidInfo(in blkidutils.BlkIdPartitionInfo) {
	p.FSType = fromStringToFSType(in.Type)
	p.UUID = in.UUID
	p.FSTypeString = in.Type
}

func (p *PartitionInfo) updateWithPartedInfo(in partedInfo) {
	p.Boot = in.boot
}

func (p *PartitionInfo) updateWithPartitionDetails(in PartitionContentDetails) {
	p.HasGrubConfig = true
	p.GrubCfgPath = in.GrubCfgPath
	p.GrubCfgContents = in.GrubCfgContents
	p.HasOSRelease = in.OSReleaseFound
	p.OSRelease = in.OSRelease
	p.HasEPICApplication = in.EPICApplicationFound
	p.EPICApplicationPath = in.EPICApplicationPath
	p.HasFstab = in.FstabFound
	p.FstabContents = in.FstabContents
}

type DriveLayout struct {
	EFIPartition PartitionInfo
}

func findTableInTail(lines []string) (table []string) {
	collecting := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if collecting {
			if line == "" {
				break
			}
			table = append(table, line)
			continue
		}
		if line == "" {
			collecting = true
		}
	}
	return
}

type fdiskRawExtract struct {
	device  string
	start   string
	sectors string
	tpe     string
	uuid    string
}

func (r fdiskRawExtract) initialPartitionInfo() (PartitionInfo, error) {
	var partitionNumber string
	for i := len(r.device) - 1; i >= 0; i-- {
		if r.device[i] == 'p' {
			partitionNumber = r.device[i+1:]
			break
		}
	}
	if len(partitionNumber) == 0 {
		return PartitionInfo{}, fmt.Errorf("partition number not found")
	}
	pn, err := strconv.Atoi(partitionNumber)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf("partition number not found: %v", err)
	}
	st, err := strconv.Atoi(r.start)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf("start sector not found: %v", err)
	}
	ss, err := strconv.Atoi(r.sectors)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf("sectors not found: %v", err)
	}
	return PartitionInfo{
		PartitionNumber:            pn,
		PartitionDevice:            r.device,
		StartSector:                st,
		SectorSize:                 ss,
		GenericPartitionTypeString: r.tpe,
		GenericPartitionType:       fromStringToGenericPartitionType(r.tpe),
		PartUUID:                   r.uuid,
	}, nil
}

type partedInfo struct {
	partitionNumber int
	boot            bool
}

var bootRe = regexp.MustCompile(`^.+\bboot\b.*$`)
var partNumberRe = regexp.MustCompile(`^\s*([0-9]+)\s+.+$`)

func parsePartedString(in string) (result []partedInfo, err error) {
	lines := util.SplitLines(in)
	lines = findTableInTail(lines)
	// We can skip the header line
	for _, line := range lines[1:] {
		var pi partedInfo
		num := partNumberRe.FindStringSubmatch(line)
		if len(num) == 2 {
			parsedNum, err := strconv.Atoi(num[1])
			if err != nil {
				continue
			}
			pi.partitionNumber = parsedNum
			if bootRe.MatchString(line) {
				pi.boot = true
			}
			result = append(result, pi)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no partitions found")
	}
	slices.SortStableFunc(result, func(a, b partedInfo) int {
		return a.partitionNumber - b.partitionNumber
	})
	return
}

type fdiskColumnOffsets struct {
	device  int
	start   int
	sectors int
	tpe     int
	uuid    int
}

func (c *fdiskColumnOffsets) validate() error {
	if c.device == -1 {
		return fmt.Errorf("device column not found")
	}
	if c.start == -1 {
		return fmt.Errorf("start column not found")
	}
	if c.sectors == -1 {
		return fmt.Errorf("sectors column not found")
	}
	if c.tpe == -1 {
		return fmt.Errorf("type column not found")
	}
	if c.uuid == -1 {
		return fmt.Errorf("uuid column not found")
	}
	return nil
}

func (c *fdiskColumnOffsets) extract(line string) fdiskRawExtract {
	return fdiskRawExtract{
		device:  strings.TrimSpace(line[c.device : c.start-1]),
		start:   strings.TrimSpace(line[c.start:c.sectors]),
		sectors: strings.TrimSpace(line[c.sectors:c.tpe]),
		tpe:     strings.TrimSpace(line[c.tpe:c.uuid]),
		uuid:    strings.TrimSpace(line[c.uuid:]),
	}
}

func (c fdiskColumnOffsets) String() string {
	return fmt.Sprintf("device: %d, start: %d, sectors: %d, type: %d, uuid: %d", c.device, c.start, c.sectors, c.tpe, c.uuid)
}

func findColumnOffsets(device string, table []string) (offsets fdiskColumnOffsets, err error) {
	deviceWithPartition := device + "p1" // assumes fewer than 10 partitions -- This is just an example to compute the length of the device name with partition number
	lengthOfDeviceWithPartition := len(deviceWithPartition)
	headerLine := table[0]
	offsets.device = strings.Index(headerLine, "Device")
	offsets.start = lengthOfDeviceWithPartition + 1
	sectorsHeaderOffset := strings.Index(headerLine, "Start") + len("Start")
	offsets.sectors = sectorsHeaderOffset
	typeHeaderOffset := strings.Index(headerLine, "Type")
	offsets.tpe = typeHeaderOffset
	uuidHeaderOffset := strings.Index(headerLine, "UUID")
	offsets.uuid = uuidHeaderOffset
	if err = offsets.validate(); err != nil {
		return
	}
	return offsets, nil
}

func doRawExtract(table []string, offsets fdiskColumnOffsets) (result []fdiskRawExtract) {
	for _, line := range table[1:] {
		result = append(result, offsets.extract(line))
	}
	return
}

func (c *Commands) FindPartitionInfoFromStrings(diskArrayDevice, fdiskOutput, blkidOutput, partedOutput string) ([]PartitionInfo, error) {
	re, err := regexp.Compile(diskArrayDevice + ":")
	if err != nil {
		return nil, fmt.Errorf("regexp.Compile() error = %v", err)
	}
	dadGrep := util.NewGrep(re)
	tail := dadGrep.MatchWithTail(fdiskOutput)
	if len(tail) == 0 {
		c.Debugf("Failed to find %s in fdisk output:\n%s\n", diskArrayDevice, fdiskOutput)
		return nil, fmt.Errorf("could not find fdisk information for device %s", diskArrayDevice)
	}
	table := findTableInTail(tail)
	if len(table) == 0 {
		c.Debugf("Failed to find table in tail:\n%s\n", strings.Join(tail, "\n"))
		return nil, fmt.Errorf("no information available for device %s", diskArrayDevice)
	}
	offsets, err := findColumnOffsets(diskArrayDevice, table)
	if err != nil {
		c.Debugf("Failed to find column offsets for fdisk output:\n%s\n", strings.Join(table, "\n"))
		return nil, fmt.Errorf("could not determine column offsets for fdisk output: %v", err)
	}
	rawExtract := doRawExtract(table, offsets)
	blkidOutputs, err := blkidutils.ParseBlkIdLayout(diskArrayDevice, blkidOutput)
	if err != nil {
		return nil, fmt.Errorf("ParseBlkIdLayout() error = %v", err)
	}
	po, err := parsePartedString(partedOutput)
	if err != nil {
		return nil, fmt.Errorf("parsePartedString() error = %v", err)
	}
	var result []PartitionInfo
	for _, r := range rawExtract {
		pi, err := r.initialPartitionInfo()
		if err != nil {
			return nil, fmt.Errorf("initialPartitionInfo() error = %v", err)
		}
		result = append(result, pi)
	}
	for _, blkid := range blkidOutputs.Partitions {
		for i, r := range result {
			pn, err := blkid.PartitionNumber()
			if err != nil {
				return nil, fmt.Errorf("PartitionNumber() error = %v", err)
			}
			if pn == r.PartitionNumber {
				result[i].updateWithBlkidInfo(blkid)
			}
		}
	}
	for _, ptd := range po {
		for i, r := range result {
			if ptd.partitionNumber == r.PartitionNumber {
				result[i].updateWithPartedInfo(ptd)
			}
		}
	}
	return result, nil
}

func (c *Commands) FindPartitionInfo(diskArrayDevice string) ([]PartitionInfo, error) {
	return WithGroupInScopeReturningErr(c, "FindPartitionInfo", func(c *Commands) ([]PartitionInfo, error) {
		fdiskCmd := c.Cmd("fdisk", "-l", "-o", "Device,Start,Sectors,Type,UUID")
		err := c.RunCmd(fdiskCmd)
		if err != nil {
			return nil, err
		}
		fdiskOutput := fdiskCmd.GetStdoutAsString()
		blkidCmd := c.Cmd("blkid")
		err = c.RunCmd(blkidCmd)
		if err != nil {
			return nil, err
		}
		blkidOut := blkidCmd.GetStdoutAsString()
		partedCmd := c.Cmd("parted", "-s", diskArrayDevice, "print")
		err = c.RunCmd(partedCmd)
		if err != nil {
			return nil, err
		}
		partedOut := partedCmd.GetStdoutAsString()
		pis, err := c.FindPartitionInfoFromStrings(diskArrayDevice, fdiskOutput, blkidOut, partedOut)
		if err != nil {
			return nil, err
		}
		var devices []string
		for _, pi := range pis {
			switch pi.FSType {
			case FSSwap, FSCryptoLUKS, FSUnknown:
				continue
			default:
				devices = append(devices, pi.PartitionDevice)
			}
		}
		partitionDetails, err := c.CollectPartitionDetailsForPartitions(devices)
		if err != nil {
			return nil, err
		}
		for _, detail := range partitionDetails {
			for i, pi := range pis {
				if detail.Device == pi.PartitionDevice {
					pis[i].updateWithPartitionDetails(detail)
				}
			}
		}
		return pis, nil
	})
}
