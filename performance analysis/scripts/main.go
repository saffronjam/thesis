package main

import (
	"log"
	"performance/pkg/config"
	"performance/pkg/environment"
)

func main() {
	config.LoadConfig(nil)

	err := environment.Setup(true, false, false)
	if err != nil {
		log.Fatalln(err.Error())
	}

	err = environment.Shutdown(false, false, false)
	if err != nil {
		log.Fatalln(err.Error())
	}
}
