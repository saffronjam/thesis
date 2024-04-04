package benchmark

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"performance/models"
	"performance/pkg/app/pretty_log"
	"performance/pkg/vm_management_system"
	"performance/pkg/vm_management_system/kubevirt"
	"performance/pkg/vm_management_system/opennebula"
	"strings"
	"sync"
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
	pretty_log.TaskGroup(" === Running benchmark ===")

	vmmsMap := make(map[string]vm_management_system.VmManagementSystem)
	for _, environment := range environments {
		vmmsMap[environment.Name] = getVmManagementSystem(&environment)
		if vmmsMap[environment.Name] == nil {
			log.Fatalln("Unknown VM management system: " + environment.Name)
		}
	}

	// 1. Setup VM management systems synchronously
	for _, environment := range environments {
		vmms := vmmsMap[environment.Name]
		pretty_log.TaskGroup("[%s] Setting up VM management system (Not benchmarked)", environment.Name)
		err := vmms.Setup()
		if err != nil {
			log.Fatalln(err.Error())
		}

		pretty_log.TaskGroup("[%s] Cleaning up before test", environment.Name)
		err = vmms.DeleteAllVMs()
		if err != nil {
			log.Fatalln(fmt.Errorf("failed to clean up. details: %s", err.Error()))
		}
	}

	// 2. Run tests asynchronously
	wg := sync.WaitGroup{}
	for _, environment := range environments {
		wg.Add(1)
		go func(environment models.BenchmarkEnvironment) {
			defer wg.Done()

			vmms := vmmsMap[environment.Name]

			pretty_log.TaskGroup("[%s] Running benchmark", environment.Name)
			timeStart := time.Now()
			results := RunTests(environment.Name, NewBenchmark(environment, vmms).AllTests())
			timeEnd := time.Now()
			pretty_log.TaskGroup("[%s] Benchmark complete (%s)", environment.Name, timeEnd.Sub(timeStart).String())

			pretty_log.TaskGroup("[%s] Saving results", environment.Name)
			for _, taskResults := range results {
				for _, result := range taskResults {
					err := SaveResult(environment.Name, result)
					if err != nil {
						log.Fatalln(fmt.Errorf("failed to save results. details: %s", err.Error()))
					}
				}
			}

			pretty_log.TaskGroup("[%s] Cleaning up after test", environment.Name)
			err := vmms.DeleteAllVMs()
			if err != nil {
				log.Fatalln(fmt.Errorf("failed to clean up. details: %s", err.Error()))
			}

			pretty_log.TaskGroup("[%s] Completed", environment.Name)
		}(environment)
	}
	wg.Wait()

	return &models.BenchmarkResult{}
}

// SaveResult saves the result of a test to a file.
// It saves the result to results/{vmm}-{group}-{name}-{date}.json
func SaveResult(vmm string, result models.TestResult) error {
	dir := "results"
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s_%s_%s_%s.json", strings.ToLower(vmm), strings.ToLower(result.Group), strings.ToLower(result.Name), time.Now().Format("2006-01-02-15-04-05"))), bytes, os.ModePerm)
}

func getVmManagementSystem(environment *models.BenchmarkEnvironment) vm_management_system.VmManagementSystem {
	switch environment.Name {
	case "OpenNebula":
		return opennebula.New(environment.AzureEnvironment)
	case "KubeVirt":
		return kubevirt.New(environment.AzureEnvironment)
	default:
		return nil
	}
}
