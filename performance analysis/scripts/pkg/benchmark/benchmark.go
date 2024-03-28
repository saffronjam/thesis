package benchmark

import (
	"log"
	"performance/models"
	"performance/pkg/app/pretty_log"
	"performance/pkg/vm_management_system"
	"performance/pkg/vm_management_system/kubevirt"
	"performance/pkg/vm_management_system/opennebula"
	"time"
)

func Run(environments []models.BenchmarkEnvironment) *models.BenchmarkResult {
	for _, environment := range environments {
		pretty_log.TaskGroup("Benchmarking " + environment.Name)

		pretty_log.TaskGroup("Setting up environment")
		vmms := getVmManagementSystem(&environment)
		if vmms == nil {
			log.Fatalln("Unknown VM management system: " + environment.Name)
		}

		pretty_log.TaskGroup("Setting up VM management system (Not benchmarked)")
		err := vmms.Setup()
		if err != nil {
			pretty_log.FailTask()
			log.Fatalln(err.Error())
		}

		timeStart := time.Now()

		time.Sleep(1 * time.Second)

		// Run benchmark
		pretty_log.TaskGroup("[%s] Running benchmark", environment.Name)
		time.Sleep(1 * time.Second)

		// TODO: Add a list of benchmarks to run

		timeEnd := time.Now()
		pretty_log.TaskResult("[%s] Benchmark complete (%s)", environment.Name, timeEnd.Sub(timeStart).String())

		// Save benchmark results
		pretty_log.TaskGroup("[%s] Saving benchmark results", environment.Name)
		time.Sleep(1 * time.Second)

		// Teardown benchmark
		pretty_log.TaskGroup("[%s] Tearing down benchmark", environment.Name)
		time.Sleep(1 * time.Second)
	}

	return &models.BenchmarkResult{}
}

func getVmManagementSystem(environment *models.BenchmarkEnvironment) vm_management_system.VmManagementSystem {
	switch environment.Name {
	case "OpenNebula":
		return opennebula.New(environment.Environment)
	case "KubeVirt":
		return kubevirt.New(environment.Environment)
	default:
		return nil
	}
}
