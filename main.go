package main

import (
	"github.com/glynnbird/gocurl/gcurl"
)

func main() {

	// create cloudant snap
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
