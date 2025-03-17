package main

import (
	"github.com/hayeah/fork2"
)

func main() {
	mainfn, err := fork2.InitMain()

	if err != nil {
		panic(err)
	}

	mainfn()
}
