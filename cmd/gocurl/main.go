package main

import (
	gcurl "github.com/glynnbird/gocurl/internal/app"
)

func main() {

	// create new gocurl struct
	goCurl, err := gcurl.New()
	if err != nil {
		panic(err)
	}

	// run it
	err = goCurl.Run()
	if err != nil {
		panic(err)
	}
}
