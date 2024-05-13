package util

import (
	"regexp"
	"strings"
	"testing"
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

const expectedTail = `Disk /dev/md126: 447.13 GiB, 480098910208 bytes, 937693184 sectors
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

func TestGrep_MatchWithTail(t *testing.T) {
	re := regexp.MustCompile(`/dev/md126:`)
	g := NewGrep(re)
	result := g.MatchWithTail(sampleFdiskOutput)
	if len(result) == 0 {
		t.Errorf("MatchWithTail() = %v, want non-empty", result)
	}
	if strings.Join(result, "\n") != expectedTail {
		t.Errorf("MatchWithTail() = %v, want %v", result, expectedTail)
	}
}
