package main

import (
	"log"
	"os"

	"github.com/rajibdpi/trynas/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
