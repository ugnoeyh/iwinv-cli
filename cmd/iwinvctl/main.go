package main

import (
	"log"

	"iwinv-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}
