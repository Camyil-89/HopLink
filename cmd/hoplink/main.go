package main

import (
	"flag"
	"fmt"
	"hoplink/internal/hoplink"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("hoplink", version)
		return
	}
	hoplink.Run()
}
