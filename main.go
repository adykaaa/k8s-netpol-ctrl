package main

import (
	"log"

	"github.com/adykaaa/k8s-netpol-ctrl/app"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatalf("could not initialize app %v", err)
	}

	app.Run()
}
