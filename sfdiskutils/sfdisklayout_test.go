package sfdiskutils

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

const sampleSFdiskOutput = `label: gpt
label-id: E853A34B-04E2-4185-AA37-778F397FA222
device: /dev/md126
unit: sectors
first-lba: 2048
last-lba: 937693150
sector-size: 512

/dev/md126p1 : start=        2048, size=     2201600, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=FD0F94BA-9DC1-4686-9C07-10BD5209819D
/dev/md126p2 : start=     2203648, size=     3145728, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=B1E70819-D0C7-4114-A581-F3B5716DA01C
/dev/md126p3 : start=     5349376, size=   722630656, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=3463082E-C8A5-4BFB-85DA-2E87553062D2
/dev/md126p4 : start=   727980032, size=   209711104, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=D0E8CCD9-54C1-49B8-BA74-BF3D7C3FA800`

const sampleSFdiskOuput1 = `label: gpt
label-id: 0044CD4E-49CE-4A27-8A2E-195DEF119571
device: /dev/nvme0n1
unit: sectors
first-lba: 34
last-lba: 1953525134
sector-size: 512

/dev/nvme0n1p1 : start=        2048, size=      204800, type=EBD0A0A2-B9E5-4433-87C0-68B6B72699C7, uuid=61E4439A-8668-4FE9-9371-4CD688886A8F, name="EFI system partition", attrs="GUID:63"
/dev/nvme0n1p2 : start=      206848, size=       32768, type=E3C9E316-0B5C-4DB8-817D-F92DF00215AE, uuid=8342B573-3019-49C4-8CBD-16E81B59B59D, name="Microsoft reserved partition", attrs="GUID:63"
/dev/nvme0n1p3 : start=      239616, size=  1950720000, type=EBD0A0A2-B9E5-4433-87C0-68B6B72699C7, uuid=BE9E4C3C-4FB2-45FA-B490-D6248950A486, name="Basic data partition"
/dev/nvme0n1p4 : start=  1952141312, size=     1380352, type=DE94BBA4-06D1-4D40-A16A-BFD50179D6AC, uuid=0FE884DA-F952-433F-88C3-0D485E0CA1B4, attrs="RequiredPartition GUID:63"
/dev/nvme0n1p5 : start=  1950959616, size=     1181696, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=9775F831-4FBC-4D1D-AAB2-A941E0864289, name="EFI system partition", attrs="GUID:63"`

const splitOutputTemplate = `label: gpt
label-id: E853A34B-04E2-4185-AA37-778F397FA222
device: /dev/md126
unit: sectors
first-lba: 2048
last-lba: 937693150
sector-size: 512

/dev/md126p1 : start=        2048, size=     2201600, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, uuid=FD0F94BA-9DC1-4686-9C07-10BD5209819D
/dev/md126p2 : start=     2203648, size=     3145728, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=B1E70819-D0C7-4114-A581-F3B5716DA01C
/dev/md126p3 : start=     5349376, size=   361315328, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=3463082E-C8A5-4BFB-85DA-2E87553062D2
/dev/md126p5 : start=   366664704, size=   361315328, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=%s
/dev/md126p4 : start=   727980032, size=   209711104, type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, uuid=D0E8CCD9-54C1-49B8-BA74-BF3D7C3FA800
`

func TestParseSFdiskLayout(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		wantResult SFDiskLayout
		wantErr    bool
	}{
		{
			name: "Linux text",
			args: sampleSFdiskOutput,
			wantResult: SFDiskLayout{
				Label:      "gpt",
				LabelID:    "E853A34B-04E2-4185-AA37-778F397FA222",
				Device:     "/dev/md126",
				Unit:       "sectors",
				FirstLBA:   2048,
				LastLBA:    937693150,
				SectorSize: 512,
				Partitions: []SFPartitionInfo{
					{
						Partition: "/dev/md126p1",
						Start:     2048,
						Size:      2201600,
						TypeUUID:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
						PartUUID:  "FD0F94BA-9DC1-4686-9C07-10BD5209819D",
					},
					{
						Partition: "/dev/md126p2",
						Start:     2203648,
						Size:      3145728,
						TypeUUID:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
						PartUUID:  "B1E70819-D0C7-4114-A581-F3B5716DA01C",
					},
					{
						Partition: "/dev/md126p3",
						Start:     5349376,
						Size:      722630656,
						TypeUUID:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
						PartUUID:  "3463082E-C8A5-4BFB-85DA-2E87553062D2",
					},
					{
						Partition: "/dev/md126p4",
						Start:     727980032,
						Size:      209711104,
						TypeUUID:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
						PartUUID:  "D0E8CCD9-54C1-49B8-BA74-BF3D7C3FA800",
					},
				},
			},
		},
		{
			name: "Windows text",
			args: sampleSFdiskOuput1,
			wantResult: SFDiskLayout{
				Label:      "gpt",
				LabelID:    "0044CD4E-49CE-4A27-8A2E-195DEF119571",
				Device:     "/dev/nvme0n1",
				Unit:       "sectors",
				FirstLBA:   34,
				LastLBA:    1953525134,
				SectorSize: 512,
				Partitions: []SFPartitionInfo{
					{
						Partition: "/dev/nvme0n1p1",
						Start:     2048,
						Size:      204800,
						TypeUUID:  "EBD0A0A2-B9E5-4433-87C0-68B6B72699C7",
						PartUUID:  "61E4439A-8668-4FE9-9371-4CD688886A8F",
						Name:      "EFI system partition",
						Attrs:     "GUID:63",
					},
					{
						Partition: "/dev/nvme0n1p2",
						Start:     206848,
						Size:      32768,
						TypeUUID:  "E3C9E316-0B5C-4DB8-817D-F92DF00215AE",
						PartUUID:  "8342B573-3019-49C4-8CBD-16E81B59B59D",
						Name:      "Microsoft reserved partition",
						Attrs:     "GUID:63",
					},
					{
						Partition: "/dev/nvme0n1p3",
						Start:     239616,
						Size:      1950720000,
						TypeUUID:  "EBD0A0A2-B9E5-4433-87C0-68B6B72699C7",
						PartUUID:  "BE9E4C3C-4FB2-45FA-B490-D6248950A486",
						Name:      "Basic data partition",
					},
					{
						Partition: "/dev/nvme0n1p5",
						Start:     1950959616,
						Size:      1181696,
						TypeUUID:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
						PartUUID:  "9775F831-4FBC-4D1D-AAB2-A941E0864289",
						Name:      "EFI system partition",
						Attrs:     "GUID:63",
					},
					{
						Partition: "/dev/nvme0n1p4",
						Start:     1952141312,
						Size:      1380352,
						TypeUUID:  "DE94BBA4-06D1-4D40-A16A-BFD50179D6AC",
						PartUUID:  "0FE884DA-F952-433F-88C3-0D485E0CA1B4",
						Attrs:     "RequiredPartition GUID:63",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ParseSFdiskLayout(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSFdiskLayout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ParseSFdiskLayout() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			fmt.Println(gotResult.String())
		})
	}
}

func TestSplitPartition(t *testing.T) {
	dl, err := ParseSFdiskLayout(sampleSFdiskOutput)
	if err != nil {
		t.Errorf("error parsing sfdisk output: %v", err)
	}
	testUUID := uuid.New().String()
	err = dl.SplitPartitionEqually(3, testUUID, "", "")
	if err != nil {
		t.Errorf("error splitting partition: %v", err)
	}
	testOutput := fmt.Sprintf(splitOutputTemplate, testUUID)
	if dl.String() != testOutput {
		t.Errorf("unexpected output: %v", dl.String())
	}
}
