package main

import (
	"log"
	"performance/pkg/config"
	"performance/pkg/environment"
)

func main() {
	config.LoadConfig(nil)

	err := environment.Setup(true, false, 2)
	if err != nil {
		log.Fatalln(err.Error())
	}

	err = environment.Shutdown(true, false)
	if err != nil {
		log.Fatalln(err.Error())
	}
}
