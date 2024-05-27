package main

import (
	"fmt"
	"os"

	"github.com/joshmeranda/marina-operator/cmd"
)

func main() {
	app := cmd.App()
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
