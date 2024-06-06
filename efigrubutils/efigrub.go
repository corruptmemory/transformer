package efigrubutils

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// search.fs_uuid 52dd4314-00ab-4362-8d84-74a1912f0285 root
// set prefix=($root)'/grub'
// configfile $prefix/grub.cfg

var searchFSUUIDRe = regexp.MustCompile(`search.fs_uuid\s+([0-9a-fA-F-]+)\s+root`)

type EFIGrubConfig struct {
	Original string
	Template *template.Template
}

func (e *EFIGrubConfig) Execute(fsUUID string) (string, error) {
	args := struct {
		FSUUID string
	}{
		FSUUID: fsUUID,
	}
	bb := &bytes.Buffer{}
	err := e.Template.Execute(bb, args)
	if err != nil {
		return "", err
	}
	return bb.String(), nil
}

func ParseEFIGrubConfigToTemplate(config string) (result *EFIGrubConfig, err error) {
	lines := strings.Split(config, "\n")
	var templateLines []string
	for _, line := range lines {
		matches := searchFSUUIDRe.FindStringSubmatch(line)
		if len(matches) > 0 {
			templateLines = append(templateLines, `search.fs_uuid {{ .FSUUID }} root`)
			continue
		}
		templateLines = append(templateLines, line)
	}
	jt := strings.Join(templateLines, "\n")
	tmpl, err := template.New("efigrub").Parse(jt)
	if err != nil {
		return nil, fmt.Errorf("error: could not construct template: %w -- given: %s", err, jt)
	}
	return &EFIGrubConfig{
		Original: config,
		Template: tmpl,
	}, nil
}
