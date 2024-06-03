package fstabutils

import (
	"fmt"
	"strings"
	"text/template"
	"transformer/util"
)

//# /etc/fstab: static file system information.
//#
//# Use 'blkid' to print the universally unique identifier for a
//# device; this may be used with UUID= as a more robust way to name devices
//# that works even if disks are added and removed. See fstab(5).
//#
//# <file system> <mount point>   <type>  <options>       <dump>  <pass>
//# / was on /dev/mapper/dm_crypt-0 during curtin installation
///dev/disk/by-id/md-uuid-8f9c5523:047788e1:ebb0b2b0:25c43eb2-part3 / ext4 defaults 0 1
//# /boot was on /dev/md126p2 during curtin installation
///dev/disk/by-id/md-uuid-8f9c5523:047788e1:ebb0b2b0:25c43eb2-part2 /boot ext4 defaults 0 1
//# /boot/efi was on /dev/md126p1 during curtin installation
///dev/disk/by-id/md-uuid-8f9c5523:047788e1:ebb0b2b0:25c43eb2-part1 /boot/efi vfat defaults 0 1
//# /home/epicsystem/addons/view-path/public/recordings was on /dev/md126p4 during curtin installation
///dev/disk/by-id/md-uuid-8f9c5523:047788e1:ebb0b2b0:25c43eb2-part4 /home/epicsystem/addons/view-path/public/recordings ext4 defaults 0 1
///swap.img	none	swap	sw	0	0

type FstabProcessResult struct {
	Original                 string
	Template                 string
	CompiledTemplate         *template.Template
	OriginalRootDevice       string
	OriginalBootDevice       string
	OriginalEfiDevice        string
	OriginalRecordingsDevice string
}

func (f *FstabProcessResult) Execute(input FstabTemplateInput) (result string, err error) {
	resultBuffer := strings.Builder{}
	err = f.CompiledTemplate.Execute(&resultBuffer, input)
	if err != nil {
		return "", err
	}
	return resultBuffer.String(), nil
}

type FstabTemplateInput struct {
	RootDevice       string
	BootDevice       string
	EfiDevice        string
	RecordingsDevice string
}

func ProcessFstab(fstab string, requireRecordingsDeviceEntry bool) (result FstabProcessResult, err error) {
	templateBuffer := strings.Builder{}
	var originalRootDevice string
	var originalBootDevice string
	var originalEfiDevice string
	var originalRecordingsDevice string

	// Split into lines
	fstabLines := strings.Split(fstab, "\n")
	for _, line := range fstabLines {
		if strings.HasPrefix(line, "/dev/") {
			splits := util.RemoveEmptySplits(strings.Split(line, " "), true)
			switch splits[1] {
			case "/":
				originalRootDevice = splits[0]
				templateBuffer.WriteString("{{ .RootDevice }} " + strings.Join(splits[1:], " ") + "\n")
			case "/boot":
				originalBootDevice = splits[0]
				templateBuffer.WriteString("{{ .BootDevice }} " + strings.Join(splits[1:], " ") + "\n")
			case "/boot/efi":
				originalEfiDevice = splits[0]
				templateBuffer.WriteString("{{ .EfiDevice }} " + strings.Join(splits[1:], " ") + "\n")
			default:
				if strings.Contains(splits[1], "recordings") {
					originalRecordingsDevice = splits[0]
					templateBuffer.WriteString("{{ .RecordingsDevice }} " + strings.Join(splits[1:], " ") + "\n")
				}
			}
			continue
		}
		templateBuffer.WriteString(line + "\n")
	}
	if originalRootDevice == "" {
		return result, fmt.Errorf("Root device not found in fstab")
	}
	if originalBootDevice == "" {
		return result, fmt.Errorf("Boot device not found in fstab")
	}
	if originalEfiDevice == "" {
		return result, fmt.Errorf("Efi device not found in fstab")
	}
	if requireRecordingsDeviceEntry && originalRecordingsDevice == "" {
		return result, fmt.Errorf("Recordings device not found in fstab")
	}
	result.Original = fstab
	result.Template = templateBuffer.String()
	result.CompiledTemplate, err = template.New("fstab").Parse(result.Template)
	if err != nil {
		return FstabProcessResult{}, err
	}
	result.OriginalRootDevice = originalRootDevice
	result.OriginalBootDevice = originalBootDevice
	result.OriginalEfiDevice = originalEfiDevice
	result.OriginalRecordingsDevice = originalRecordingsDevice
	return result, nil
}
