package main

import (
	"fmt"
	"log"

	"transformer/cmdr"
)

func main() {
	cmd := cmdr.NewCommand("ls", "-la")
	gcmd := cmdr.NewCommand("grep", "main")
	err := cmd.PipeTo(gcmd)
	if err != nil {
		log.Fatalln(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Printf("ls start error: %v\n", err)
	}
	err = gcmd.Start()
	if err != nil {
		log.Printf("grep start error: %v\n", err)
	}
	err = cmd.Wait()
	if err != nil {
		log.Printf("ls wait error: %v\n", err)
	}
	err = gcmd.Wait()
	if err != nil {
		log.Printf("grep wait error: %v\n", err)
	}
	fmt.Printf("ls exit code: %d\n", cmd.GetExitCode())
	fmt.Printf("ls stdout: %s\n", cmd.GetStdoutAsString())
	fmt.Printf("ls stderr: %s\n", cmd.GetStderrAsString())
	fmt.Printf("grep exit code: %d\n", gcmd.GetExitCode())
	fmt.Printf("grep stdout: %s\n", gcmd.GetStdoutAsString())
	fmt.Printf("grep stderr: %s\n", gcmd.GetStderrAsString())
}
