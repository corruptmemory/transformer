package commands

import (
	"errors"
	"log/slog"

	"transformer/cmdr"
	"transformer/fsutils"
)

type DiskArray struct {
	ArrayName  string `json:"arrayName"`
	DevicePath string `json:"devicePath"`
}

var ErrMdStatNotFound = errors.New("/proc/mdstat not found")

func FindDiskArray() (result DiskArray, err error) {
	e, _, _, err := fsutils.WhatsThere("/proc/mdstat")
	if err != nil {
		return result, err
	}
	if !e {
		return result, ErrMdStatNotFound
	}
	pipe, err := cmdr.PipeCommands(
		cmdr.NewCommand("cat", "/proc/mdstat"),
		cmdr.NewCommand("grep", "-w", "active"),
		cmdr.NewCommand("grep", "-w", "raid1"),
	)
	if err != nil {
		return result, err
	}
	pipe.SetLogger(slog.Default())
	err = pipe.Run()
	if err != nil {
		return result, err
	}
	return result, nil
}
