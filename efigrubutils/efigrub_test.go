package efigrubutils

import "testing"

const sampleEFIGrubConfig = `search.fs_uuid 52dd4314-00ab-4362-8d84-74a1912f0285 root
set prefix=($root)'/grub'
configfile $prefix/grub.cfg
`

const sampleUUID = "aabbccdd-eeff-0011-2233-445566778899"

const sampleEFIGrubConfigWithUUID = `search.fs_uuid ` + sampleUUID + ` root
set prefix=($root)'/grub'
configfile $prefix/grub.cfg
`

func TestParseEFIGrubConfigToTemplate(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		wantResult *EFIGrubConfig
		wantErr    bool
	}{
		{
			name: "TestParseEFIGrubConfigToTemplate",
			args: sampleEFIGrubConfig,
			wantResult: &EFIGrubConfig{
				Original: sampleEFIGrubConfig,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ParseEFIGrubConfigToTemplate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEFIGrubConfigToTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotResult.Original != tt.wantResult.Original {
				t.Errorf("ParseEFIGrubConfigToTemplate() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			out, err := gotResult.Execute(sampleUUID)
			if err != nil {
				t.Errorf("Execute returned an error: %v", err)
			}
			if out != sampleEFIGrubConfigWithUUID {
				t.Errorf("Execute() gotResult = %v, want %v", out, sampleEFIGrubConfigWithUUID)
			}
		})
	}
}
