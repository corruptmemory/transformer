package commands

import (
	"reflect"
	"testing"
)

func TestCommands_FindDiskArrayFromString(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		wantResult DiskArray
		wantErr    bool
	}{
		{
			name:    "empty string",
			args:    "",
			wantErr: true,
		},
		{
			name: "no disk array",
			args: `Personalities : [raid1] [linear] [multipath] [raid0] [raid6] [raid5] [raid4] [raid10]
md127 : inactive sda[1](S) sdb[0](S)
     9904 blocks super external:imsm

unused devices: <none>`,
			wantErr: true,
		},
		{
			name: "multiple disk arrays",
			args: `Personalities : [raid1] [linear] [multipath] [raid0] [raid6] [raid5] [raid4] [raid10]
md126 : active (auto-read-only) raid1 sda[1] sdb[0]
468846592 blocks super external:/md127/0 [2/2] [UU]

md127 : active (auto-read-only) raid1 sdc[1] sdd[0]
468846592 blocks super external:/md128/0 [2/2] [UU]

unused devices: <none>`,
			wantErr: true,
		},
		{
			name: "success",
			args: `Personalities : [raid1] [linear] [multipath] [raid0] [raid6] [raid5] [raid4] [raid10]
md126 : active (auto-read-only) raid1 sda[1] sdb[0]
     468846592 blocks super external:/md127/0 [2/2] [UU]

md127 : inactive sda[1](S) sdb[0](S)
     9904 blocks super external:imsm

unused devices: <none>`,
			wantResult: DiskArray{
				ArrayName:  "md126",
				DevicePath: "/dev/md126",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := DefaultCommands(false)
			if err != nil {
				t.Errorf("DefaultCommands() error = %v", err)
				return
			}
			gotResult, err := c.FindDiskArrayFromString(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindDiskArrayFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("FindDiskArrayFromString() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
