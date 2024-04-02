package benchmark

import (
	"fmt"
	"log"
	"performance/models"
	"performance/pkg/app/pretty_log"
	"performance/pkg/vm_management_system"
	"performance/pkg/vm_management_system/kubevirt"
	"performance/pkg/vm_management_system/opennebula"
	"time"
)

type Benchmark struct {
	Environment models.BenchmarkEnvironment
	VMMS        vm_management_system.VmManagementSystem
}

func NewBenchmark(environment models.BenchmarkEnvironment, vmms vm_management_system.VmManagementSystem) *Benchmark {
	return &Benchmark{
		Environment: environment,
		VMMS:        vmms,
	}
}

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
			log.Fatalln(err.Error())
		}

		pretty_log.TaskGroup("[%s] Cleaning up before test", environment.Name)
		err = vmms.DeleteAllVMs()
		if err != nil {
			log.Fatalln(fmt.Errorf("failed to clean up. details: %s", err.Error()))
		}

		timeStart := time.Now()

		// Run benchmark
		pretty_log.TaskGroup("[%s] Running benchmark", environment.Name)

		_ = RunTests(environment.Name, NewBenchmark(environment, vmms).AllTests())
		//for group, taskResults := range results {
		//
		//}

		timeEnd := time.Now()
		pretty_log.TaskResult("[%s] Benchmark complete (%s)", environment.Name, timeEnd.Sub(timeStart).String())

		pretty_log.TaskGroup("[%s] Cleaning up after test", environment.Name)
		err = vmms.DeleteAllVMs()
		if err != nil {
			log.Fatalln(fmt.Errorf("failed to clean up. details: %s", err.Error()))
		}

		// Save benchmark results
		pretty_log.TaskGroup("[%s] Saving benchmark results", environment.Name)
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
