package main

import (
	//"fmt"
	"log"

	"9fans.net/go/draw"
)

func main() {
	errors := make(chan error, 10)
	_, err := draw.Init(errors, "/lib/font/bit/Go-Regular/unicode.14.font", "9sheet", "1024x768")
	if err != nil {
		log.Fatalf("draw.Init: %s\n", err)
	}

	
}
