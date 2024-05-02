package commands

import (
	"log/slog"

	"transformer/cmdr"
)

type DiskArray struct {
	ArrayName  string `json:"arrayName"`
	DevicePath string `json:"devicePath"`
}

func FindDiskArray() (result DiskArray, err error) {
	pipe := cmdr.PipeCommands(
		cmdr.NewCommand("cat", "/proc/mdstat"),
		cmdr.NewCommand("grep", "-w", "active"),
		cmdr.NewCommand("grep", "-w", "raid1"),
	)
	pipe.SetLogger(slog.Default())
	err = pipe.Start()
	if err != nil {
		return result, err
	}
	err = pipe.Wait()
	if err != nil {
		return result, err
	}
	return result, nil
}
