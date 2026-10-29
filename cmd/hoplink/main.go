package main

import (
	"fmt"
	"os"

	"hoplink/internal/hoplink"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Println("hoplink", version)
		return
	}
	hoplink.Run()
}
