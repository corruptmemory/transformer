package commands

import (
	"fmt"
)

func (c *Commands) Mkfs(partition string) (err error) {
	return c.WithGroupInScopeErr("Mkfs", func(c *Commands) (err error) {
		mkfsCmd := c.Cmd("mkfs.ext4", partition)
		err = c.RunCmd(mkfsCmd)
		if err != nil {
			c.Errorf("Error running mkfs.ext4 command: %v", err)
			c.Errorf("STDOUT: %s", mkfsCmd.GetStdoutAsString())
			c.Errorf("STDERR: %s", mkfsCmd.GetStdoutAsString())
			return fmt.Errorf("error running sfdisk command: %v", err)
		}
		return nil
	})
}
