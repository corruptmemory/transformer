package dfutils

import (
	"reflect"
	"testing"
)

const sampleDFOuput = `Filesystem                        1K-blocks     Used Available Use% Mounted on
tmpfs                               1628220     1804   1626416   1% /run
efivarfs                                256       82       170  33% /sys/firmware/efi/efivars
/dev/mapper/ubuntu--vg-ubuntu--lv 102626232 25323168  72043800  27% /
tmpfs                               8141088        0   8141088   0% /dev/shm
tmpfs                                  5120        0      5120   0% /run/lock
/dev/sdc2                           1992552    97600   1773712   6% /boot
/dev/sdc1                           1098628     6284   1092344   1% /boot/efi
tmpfs                               1628216       12   1628204   1% /run/user/1000
`

func TestParseDFOutput(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		wantResult DiskUsageList
		wantErr    bool
	}{
		{
			name: "Test 1",
			args: sampleDFOuput,
			wantResult: DiskUsageList{
				{
					Filesystem:   "tmpfs",
					SizeInBlocks: 1628220,
					Used:         1804,
					MountedOn:    "/run",
				},
				{
					Filesystem:   "efivarfs",
					SizeInBlocks: 256,
					Used:         82,
					MountedOn:    "/sys/firmware/efi/efivars",
				},
				{
					Filesystem:   "/dev/mapper/ubuntu--vg-ubuntu--lv",
					SizeInBlocks: 102626232,
					Used:         25323168,
					MountedOn:    "/",
				},
				{
					Filesystem:   "tmpfs",
					SizeInBlocks: 8141088,
					Used:         0,
					MountedOn:    "/dev/shm",
				},
				{
					Filesystem:   "tmpfs",
					SizeInBlocks: 5120,
					Used:         0,
					MountedOn:    "/run/lock",
				},
				{
					Filesystem:   "/dev/sdc2",
					SizeInBlocks: 1992552,
					Used:         97600,
					MountedOn:    "/boot",
				},
				{
					Filesystem:   "/dev/sdc1",
					SizeInBlocks: 1098628,
					Used:         6284,
					MountedOn:    "/boot/efi",
				},
				{
					Filesystem:   "tmpfs",
					SizeInBlocks: 1628216,
					Used:         12,
					MountedOn:    "/run/user/1000",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ParseDFOutput(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDFOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ParseDFOutput() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
