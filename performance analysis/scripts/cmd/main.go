package main

import (
	"log"
	"performance/pkg/app"
	"performance/pkg/benchmark"
	"performance/pkg/environment"
)

func main() {
	app.LoadConfig(nil)

	environments, err := environment.Setup(false, false)
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Only KubeVirt right now, index 1
	//environments = []models.BenchmarkEnvironment{environments[1]}

	// Run benchmark
	benchmark.Run(environments)

	err = environment.Shutdown(false, false)
	if err != nil {
		log.Fatalln(err.Error())
	}
}
