package main

import (
	"fmt"

	"transformer/commands"
)

func main() {
	var foo commands.OSRelease
	foo, err := foo.GetOSRelease()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", foo)
	_, err = commands.FindDiskArray()
	if err != nil {
		panic(err)
	}
}
