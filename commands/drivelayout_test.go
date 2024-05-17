package commands

import (
	"fmt"
	"regexp"
	"testing"

	"transformer/util"
)

const sampleFdiskOutput = `Disk /dev/loop0: 63.29 MiB, 66359296 bytes, 129608 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


Disk /dev/loop1: 63.97 MiB, 67051520 bytes, 130960 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


Disk /dev/loop2: 91.85 MiB, 96292864 bytes, 188072 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


Disk /dev/loop3: 49.86 MiB, 52260864 bytes, 102072 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


GPT PMBR size mismatch (937693183 != 937703087) will be corrected by write.
The backup GPT table is not on the end of the device. This problem will be corrected by write.
Disk /dev/sda: 447.13 GiB, 480103981056 bytes, 937703088 sectors
Disk model: KINGSTON SEDC500
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
Disklabel type: gpt
Disk identifier: 4858D11B-A6C9-4449-B115-4B0AE996C7C7

Device         Start   Sectors Type             UUID
/dev/sda1       2048   2201600 EFI System       87BAC5EF-0E98-4402-BE33-D9469DF5950A
/dev/sda2    2203648   3145728 Linux filesystem A6C75110-02A4-4867-8D42-34FA925E13A5
/dev/sda3    5349376 722630656 Linux filesystem EAB72850-85E0-4132-8546-50B5A391B274
/dev/sda4  727980032 209711104 Linux filesystem 9C564E76-291E-4E63-94DB-FB430379FFA9


GPT PMBR size mismatch (937693183 != 937703087) will be corrected by write.
The backup GPT table is not on the end of the device. This problem will be corrected by write.
Disk /dev/sdb: 447.13 GiB, 480103981056 bytes, 937703088 sectors
Disk model: KINGSTON SEDC500
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
Disklabel type: gpt
Disk identifier: 4858D11B-A6C9-4449-B115-4B0AE996C7C7

Device         Start   Sectors Type             UUID
/dev/sdb1       2048   2201600 EFI System       87BAC5EF-0E98-4402-BE33-D9469DF5950A
/dev/sdb2    2203648   3145728 Linux filesystem A6C75110-02A4-4867-8D42-34FA925E13A5
/dev/sdb3    5349376 722630656 Linux filesystem EAB72850-85E0-4132-8546-50B5A391B274
/dev/sdb4  727980032 209711104 Linux filesystem 9C564E76-291E-4E63-94DB-FB430379FFA9


Disk /dev/sdc: 931.53 GiB, 1000204886016 bytes, 1953525168 sectors
Disk model: CT1000MX500SSD1 
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 4096 bytes
I/O size (minimum/optimal): 4096 bytes / 4096 bytes
Disklabel type: gpt
Disk identifier: 0466B0FF-C406-4A44-A9A6-8A4642FA2516

Device       Start    Sectors Type             UUID
/dev/sdc1     2048    2201600 EFI System       5D6DC46A-6D80-44B2-96ED-5C68DD4983FE
/dev/sdc2  2203648    4194304 Linux filesystem C7CF9B42-3BFA-4CC4-8FFB-82A017B76157
/dev/sdc3  6397952 1947123712 Linux filesystem B54635A5-000C-4478-A98B-8980B7DCD1D7


Disk /dev/md126: 447.13 GiB, 480098910208 bytes, 937693184 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
Disklabel type: gpt
Disk identifier: 4858D11B-A6C9-4449-B115-4B0AE996C7C7

Device           Start   Sectors Type             UUID
/dev/md126p1      2048   2201600 EFI System       87BAC5EF-0E98-4402-BE33-D9469DF5950A
/dev/md126p2   2203648   3145728 Linux filesystem A6C75110-02A4-4867-8D42-34FA925E13A5
/dev/md126p3   5349376 722630656 Linux filesystem EAB72850-85E0-4132-8546-50B5A391B274
/dev/md126p4 727980032 209711104 Linux filesystem 9C564E76-291E-4E63-94DB-FB430379FFA9


Disk /dev/mapper/ubuntu--vg-ubuntu--lv: 928.47 GiB, 996923146240 bytes, 1947115520 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 4096 bytes
I/O size (minimum/optimal): 4096 bytes / 4096 bytes


`

const sampleTable = `Device           Start   Sectors Type             UUID
/dev/md126p1      2048   2201600 EFI System       87BAC5EF-0E98-4402-BE33-D9469DF5950A
/dev/md126p2   2203648   3145728 Linux filesystem A6C75110-02A4-4867-8D42-34FA925E13A5
/dev/md126p3   5349376 722630656 Linux filesystem EAB72850-85E0-4132-8546-50B5A391B274
/dev/md126p4 727980032 209711104 Linux filesystem 9C564E76-291E-4E63-94DB-FB430379FFA9`

const sampleBlkidOutput = `dev/sdb: TYPE="isw_raid_member"
/dev/sda: TYPE="isw_raid_member"
/dev/sdc1: UUID="3F17-2AE2" TYPE="vfat" PARTUUID="5d6dc46a-6d80-44b2-96ed-5c68dd4983fe"
/dev/sdc2: UUID="2d905233-a877-49d6-922c-15e11c96663f" TYPE="ext4" PARTUUID="c7cf9b42-3bfa-4cc4-8ffb-82a017b76157"
/dev/sdc3: UUID="3KnIDj-m1I5-txSH-juaX-D6w7-6JdO-djOa8C" TYPE="LVM2_member" PARTUUID="b54635a5-000c-4478-a98b-8980b7dcd1d7"
/dev/md126p1: UUID="0386-7550" TYPE="vfat" PARTUUID="87bac5ef-0e98-4402-be33-d9469df5950a"
/dev/md126p2: UUID="e8f4cc8a-de54-4096-8838-e0f2dacc9c9d" TYPE="ext4" PARTUUID="a6c75110-02a4-4867-8d42-34fa925e13a5"
/dev/md126p3: UUID="ce50ebb8-68cd-49ad-b885-20e7676e9718" TYPE="crypto_LUKS" PARTUUID="eab72850-85e0-4132-8546-50b5a391b274"
/dev/md126p4: UUID="ccb2493d-7ac9-4a9a-8f30-97533a0705d8" TYPE="ext4" PARTUUID="9c564e76-291e-4e63-94db-fb430379ffa9"
/dev/mapper/ubuntu--vg-ubuntu--lv: UUID="ec94f87b-6ac2-46bf-8f07-9f0fb02791e0" TYPE="ext4"
/dev/loop0: TYPE="squashfs"
/dev/loop1: TYPE="squashfs"
/dev/loop2: TYPE="squashfs"
/dev/loop3: TYPE="squashfs"
/dev/loop4: TYPE="squashfs"`

var expectedExtract = []fdiskRawExtract{
	{
		device:  "/dev/md126p1",
		start:   "2048",
		sectors: "2201600",
		tpe:     "EFI System",
		uuid:    "87BAC5EF-0E98-4402-BE33-D9469DF5950A",
	},
	{
		device:  "/dev/md126p2",
		start:   "2203648",
		sectors: "3145728",
		tpe:     "Linux filesystem",
		uuid:    "A6C75110-02A4-4867-8D42-34FA925E13A5",
	},
	{
		device:  "/dev/md126p3",
		start:   "5349376",
		sectors: "722630656",
		tpe:     "Linux filesystem",
		uuid:    "EAB72850-85E0-4132-8546-50B5A391B274",
	},
	{
		device:  "/dev/md126p4",
		start:   "727980032",
		sectors: "209711104",
		tpe:     "Linux filesystem",
		uuid:    "9C564E76-291E-4E63-94DB-FB430379FFA9",
	},
}

// sudo parted -s /dev/md126 print
const samplePartedOutput = `Model: Linux Software RAID Array (md)
Disk /dev/md126: 480GB
Sector size (logical/physical): 512B/512B
Partition Table: gpt
Disk Flags:

Number  Start   End     Size    File system  Name  Flags
 1      1049kB  1128MB  1127MB  fat32              boot, esp
 2      1128MB  2739MB  1611MB  ext4
 3      2739MB  373GB   370GB
 4      373GB   480GB   107GB   ext4`

var expectedPIS = []PartitionInfo{
	{
		PartitionNumber:            1,
		PartitionDevice:            "/dev/md126p1",
		Boot:                       true,
		StartSector:                2048,
		SectorSize:                 2201600,
		GenericPartitionTypeString: "EFI System",
		GenericPartitionType:       GPEFI,
		FSTypeString:               "vfat",
		FSType:                     FSVFAT,
		PartUUID:                   "87BAC5EF-0E98-4402-BE33-D9469DF5950A",
	},
	{
		PartitionNumber:            2,
		PartitionDevice:            "/dev/md126p2",
		Boot:                       false,
		StartSector:                2203648,
		SectorSize:                 3145728,
		GenericPartitionTypeString: "Linux filesystem",
		GenericPartitionType:       GPLinux,
		FSTypeString:               "ext4",
		FSType:                     FSExt4,
		PartUUID:                   "A6C75110-02A4-4867-8D42-34FA925E13A5",
	},
	{
		PartitionNumber:            3,
		PartitionDevice:            "/dev/md126p3",
		Boot:                       false,
		StartSector:                5349376,
		SectorSize:                 722630656,
		GenericPartitionTypeString: "Linux filesystem",
		GenericPartitionType:       GPLinux,
		FSTypeString:               "crypto_LUKS",
		FSType:                     FSCryptoLUKS,
		PartUUID:                   "EAB72850-85E0-4132-8546-50B5A391B274",
	},
	{
		PartitionNumber:            4,
		PartitionDevice:            "/dev/md126p4",
		Boot:                       false,
		StartSector:                727980032,
		SectorSize:                 209711104,
		GenericPartitionTypeString: "Linux filesystem",
		GenericPartitionType:       GPLinux,
		FSTypeString:               "ext4",
		FSType:                     FSExt4,
		PartUUID:                   "9C564E76-291E-4E63-94DB-FB430379FFA9",
	},
}

func Test_doRawExtract(t *testing.T) {
	lines := util.SplitLines(sampleTable)
	offsets, err := findColumnOffsets("/dev/md126", lines)
	if err != nil {
		t.Errorf("findColumnOffsets() error = %v, want nil", err)
		return
	}
	result := doRawExtract(lines, offsets)
	// fmt.Printf("result: %+v\n", result)
	if len(result) != 4 {
		t.Errorf("doRawExtract() = %v, want 4", len(result))
	}
	for i, r := range result {
		if r != expectedExtract[i] {
			t.Errorf("doRawExtract() = %v, want %v", r, expectedExtract[i])
		}
	}
}

func Test_doRawExtractFromTail(t *testing.T) {
	re := util.NewGrep(regexp.MustCompile(`/dev/md126:`))
	tail := re.MatchWithTail(sampleFdiskOutput)
	if len(tail) == 0 {
		t.Errorf("MatchWithTail() = %v, want non-empty", tail)
	}
	table := findTableInTail(tail)
	if len(table) == 0 {
		t.Errorf("findTableInTail() = %v, want non-nil", table)
		return
	}
	offsets, err := findColumnOffsets("/dev/md126", table)
	if err != nil {
		t.Errorf("findColumnOffsets() error = %v, want nil", err)
		return
	}
	result := doRawExtract(table, offsets)
	// fmt.Printf("result: %+v\n", result)
	if len(result) != 4 {
		t.Errorf("doRawExtract() = %v, want 4", len(result))
	}
	for i, r := range result {
		if r != expectedExtract[i] {
			t.Errorf("doRawExtract() = %v, want %v", r, expectedExtract[i])
		}
	}
}

func Test_FindPartitionInfoFromStrings(t *testing.T) {
	cmds, err := DefaultCommands(false)
	if err != nil {
		t.Errorf("DefaultCommands() error = %v, want nil", err)
		return
	}
	pis, err := cmds.FindPartitionInfoFromStrings("/dev/md126", sampleFdiskOutput, sampleBlkidOutput, samplePartedOutput)
	if err != nil {
		t.Errorf("FindPartitionInfoFromStrings() error = %v, want nil", err)
		return
	}
	if len(pis) != len(expectedPIS) {
		t.Errorf("Expected %d partitons, found: %v", len(expectedPIS), len(pis))
		return
	}
	for _, pi := range pis {
		matched := false
		for _, e := range expectedPIS {
			if pi == e {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("PartitionInfo not matched to expected value: %+v", pi)
		}
	}
}

func Test_FindPartitionInfo(t *testing.T) {
	cmds, err := DefaultCommands(true)
	if err != nil {
		t.Errorf("DefaultCommands() error = %v, want nil", err)
		return
	}
	pis, err := cmds.FindPartitionInfo("/dev/nvme1n1")
	if err != nil {
		t.Errorf("FindPartitionInfo() error = %v, want nil", err)
		return
	}
	fmt.Printf("pis: %+v\n", pis)
}
