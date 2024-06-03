package commands

import (
	"fmt"
	"strings"
	"transformer/sfdiskutils"
)

func (c *Commands) SFDiskData(diskArrayDevice string) (result sfdiskutils.SFDiskLayout, err error) {
	return WithGroupInScopeReturningErr(c, "SFDiskData", func(c *Commands) (result sfdiskutils.SFDiskLayout, err error) {
		sfdiskCmd := c.Cmd("sfdisk", "-d", diskArrayDevice)
		err = c.RunCmd(sfdiskCmd)
		if err != nil {
			c.Errorf("Error running sfdisk command: %v", err)
			c.Errorf("STDOUT: %s", sfdiskCmd.GetStdoutAsString())
			c.Errorf("STDERR: %s", sfdiskCmd.GetStderrAsString())
			return result, fmt.Errorf("error running sfdisk command: %v", err)
		}
		result, err = sfdiskutils.ParseSFdiskLayout(sfdiskCmd.GetStdoutAsString())
		if err != nil {
			c.Errorf("Error parsing sfdisk output: %v", err)
			c.Errorf("Given: %s", sfdiskCmd.GetStdoutAsString())
			return result, fmt.Errorf("error parsing sfdisk output: %v", err)
		}
		return result, nil
	})
}

func (c *Commands) SFDiskApply(diskArrayDevice string, layout sfdiskutils.SFDiskLayout) (err error) {
	return c.WithGroupInScopeErr("SFDiskApply", func(c *Commands) (err error) {
		layoutString := layout.String()
		sfdiskCmd := c.Cmd("sfdisk", diskArrayDevice)
		r := strings.NewReader(layoutString)
		sfdiskCmd.SetStdin(r)
		err = c.RunCmd(sfdiskCmd)
		if err != nil {
			c.Errorf("Error running sfdisk command: %v", err)
			c.Errorf("STDIN: %s", layoutString)
			c.Errorf("STDOUT: %s", sfdiskCmd.GetStdoutAsString())
			c.Errorf("STDERR: %s", sfdiskCmd.GetStderrAsString())
			return fmt.Errorf("error running sfdisk command: %v", err)
		}
		return nil
	})
}
