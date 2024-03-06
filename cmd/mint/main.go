package main

import (
	"os"

	"github.com/mintoolkit/mint/pkg/app/master"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mint" {
		//hack to handle plugin invocations
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	app.Run()
}
