package commands

import (
	"fmt"
	"transformer/blkidutils"
)

func (c *Commands) BLKIDLayout(diskArrayDevice string) (result *blkidutils.BlkIdLayout, err error) {
	return WithGroupInScopeReturningErr(c, "SFDiskData", func(c *Commands) (result *blkidutils.BlkIdLayout, err error) {
		blkidCmd := c.Cmd("blkid")
		err = c.RunCmd(blkidCmd)
		if err != nil {
			c.Errorf("STDERR: %s", blkidCmd.GetStderrAsString())
			c.Errorf("STDOUT: %s", blkidCmd.GetStdoutAsString())
			return nil, fmt.Errorf("error running blkid command: %v", err)
		}
		return blkidutils.ParseBlkIdLayout(diskArrayDevice, blkidCmd.GetStdoutAsString())
	})
}
