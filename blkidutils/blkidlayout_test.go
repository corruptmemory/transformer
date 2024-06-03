package blkidutils

import (
	"reflect"
	"testing"
)

const sampleBlkidOutput = `/dev/sdb: TYPE="isw_raid_member"
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

func TestParseBlkIdLayout(t *testing.T) {
	tests := []struct {
		name       string
		device     string
		in         string
		wantResult *BlkIdLayout
		wantErr    bool
	}{
		{
			name:   "Test 1",
			device: "/dev/md126",
			in:     sampleBlkidOutput,
			wantResult: &BlkIdLayout{
				Partitions: []BlkIdPartitionInfo{
					{
						Partition: "/dev/md126p1",
						UUID:      "0386-7550",
						PartUUID:  "87bac5ef-0e98-4402-be33-d9469df5950a",
						Type:      "vfat",
					},
					{
						Partition: "/dev/md126p2",
						UUID:      "e8f4cc8a-de54-4096-8838-e0f2dacc9c9d",
						PartUUID:  "a6c75110-02a4-4867-8d42-34fa925e13a5",
						Type:      "ext4",
					},
					{
						Partition: "/dev/md126p3",
						UUID:      "ce50ebb8-68cd-49ad-b885-20e7676e9718",
						PartUUID:  "eab72850-85e0-4132-8546-50b5a391b274",
						Type:      "crypto_LUKS",
					},
					{
						Partition: "/dev/md126p4",
						UUID:      "ccb2493d-7ac9-4a9a-8f30-97533a0705d8",
						PartUUID:  "9c564e76-291e-4e63-94db-fb430379ffa9",
						Type:      "ext4",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ParseBlkIdLayout(tt.device, tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBlkIdLayout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ParseBlkIdLayout() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
