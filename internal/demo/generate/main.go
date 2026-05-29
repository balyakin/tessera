package main

import (
	"log"

	"github.com/balyakin/tessera/internal/demo"
)

func main() {
	if err := demo.WriteDemoFiles("examples"); err != nil {
		log.Fatal(err)
	}
}
