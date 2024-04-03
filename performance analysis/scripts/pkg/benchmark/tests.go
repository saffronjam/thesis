package benchmark

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"performance/models"
	"performance/pkg/app"
	"performance/pkg/app/pretty_log"
	"performance/utils"
	"strconv"
	"sync"
	"time"
)

const (
	TimerStarted   = "started"
	TimerFinished  = "finished"
	TimerVmRunning = "vm-running"
)

func (b *Benchmark) AllTests() []models.TestDefinition {
	return []models.TestDefinition{
		{
			Name: "CreateEachType",
			Func: b.CreateEachType,
		},
		{
			Name: "CreateManyTinyVMs",
			Func: b.CreateManyTinyVMs,
		},
		{
			Name: "CreateSnapshot",
			Func: b.CreateSnapshot,
		},
		{
			Name: "LiveMigrate",
			Func: b.LiveMigrate,
		},
		{
			Name: "ScaleUpCluster",
			Func: b.ScaleUpCluster,
		},
		{
			Name: "ScaleDownCluster",
			Func: b.ScaleDownCluster,
		},
		{
			Name: "ScaleDownClusterWithVMs",
			Func: b.ScaleDownClusterWithVMs,
		},
	}
}

func RunTests(vmm string, tests []models.TestDefinition) map[string][]models.TestResult {
	results := make(map[string][]models.TestResult)

	for idx, test := range tests {
		pretty_log.TaskGroup("[%s] Running test %d/%d: %s", vmm, idx+1, len(tests), test.Name)

		res := test.Func()
		if _, ok := results[test.Name]; !ok {
			results[test.Name] = make([]models.TestResult, 0)
		}

		groupResults := results[test.Name]
		groupResults = append(groupResults, res...)
		results[test.Name] = groupResults
	}

	return results
}

// StartMetricScrapers starts the metric scrapers on all nodes.
// It also deletes the previous output.json file.
func (b *Benchmark) StartMetricScrapers() error {
	mut := sync.RWMutex{}
	wg := sync.WaitGroup{}
	var anyErr error

	// Delete /home/ + config.Config.Azure.Username + /run and /home/ + config.Config.Azure.Username + /output.json
	commands := []string{
		// 1. Stop the scraper
		"rm -f /home/" + app.Config.Azure.Username + "/run",
		// 2. Sleep to ensure the scraper is stopped
		"sleep 1",
		// 3. Delete old data
		"rm -f /home/" + app.Config.Azure.Username + "/output.json",
		// 4. Start the scraper
		"echo \"\" > /home/" + app.Config.Azure.Username + "/run",
	}

	ips := []string{b.Environment.AzureEnvironment.ControlNode.PublicIP}
	for _, worker := range b.Environment.AzureEnvironment.WorkerNodes {
		ips = append(ips, worker.PublicIP)
	}

	// Start the metric scrapers
	for _, ip := range ips {
		i := ip
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()

			for _, command := range commands {
				_, err := utils.SshCommand(ip, []string{command})
				if err != nil {
					mut.Lock()
					anyErr = err
					mut.Unlock()
					return
				}
			}
		}(i)
	}

	wg.Wait()

	return anyErr
}

// StopMetricScrapers stops the metric scrapers on all nodes and downloads the output.json file from each node.
func (b *Benchmark) StopMetricScrapers() ([]models.NodeMetrics, error) {
	var res []models.NodeMetrics
	mut := sync.RWMutex{}
	wg := sync.WaitGroup{}
	var anyErr error

	stopCommands := []string{
		// 1. Stop the scraper
		"rm -f /home/" + app.Config.Azure.Username + "/run",
		// 2. Sleep to ensure the scraper is stopped
		"sleep 1",
	}

	cleanUpCommands := []string{
		// 3. Delete old data
		"rm -f /home/" + app.Config.Azure.Username + "/output.json",
	}

	ips := []string{b.Environment.AzureEnvironment.ControlNode.PublicIP}
	for _, worker := range b.Environment.AzureEnvironment.WorkerNodes {
		ips = append(ips, worker.PublicIP)
	}

	for _, ip := range ips {
		i := ip
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()

			for _, command := range stopCommands {
				_, err := utils.SshCommand(ip, []string{command})
				if err != nil {
					mut.Lock()
					anyErr = err
					mut.Unlock()
					return
				}
			}

			workerMetrics, err := GetMetrics(ip)
			if err != nil {
				mut.Lock()
				anyErr = err
				mut.Unlock()
				return
			}

			mut.Lock()
			res = append(res, workerMetrics...)
			mut.Unlock()

			// Clean up
			for _, command := range cleanUpCommands {
				_, err := utils.SshCommand(ip, []string{command})
				if err != nil {
					mut.Lock()
					anyErr = err
					mut.Unlock()
					return
				}

			}
		}(i)
	}

	wg.Wait()

	if anyErr != nil {
		return nil, anyErr
	}

	return res, nil
}

func GetMetrics(ip string) ([]models.NodeMetrics, error) {
	outputFile := "scripts/output-" + fmt.Sprintf("%d", rand.Intn(10000)) + ".json"

	err := utils.SshDownload(ip, "/home/"+app.Config.Azure.Username+"/output.json", outputFile)
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, err
	}

	var metrics []models.NodeMetrics
	err = json.Unmarshal(file, &metrics)
	if err != nil {
		return nil, err
	}

	err = os.Remove(outputFile)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (b *Benchmark) CreateEachType() []models.TestResult {
	vms := map[string]*models.VM{
		"tiny":   TinyVM(),
		"small":  SmallVM(),
		"medium": MediumVM(),
		"large":  LargeVM(),
	}

	var res []models.TestResult
	for name, vm := range vms {
		testRes := models.TestResult{
			Name:  "create-" + name,
			Group: "create-each-type",
		}

		pretty_log.TaskGroup("[%s] Setting up metrics for %s VM", b.Environment.Name, name)
		err := b.StartMetricScrapers()
		if err != nil {
			pretty_log.TaskResultBad("[%s] Failed to set up metrics for %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}

		pretty_log.TaskGroup("[%s] Creating %s VM", b.Environment.Name, name)
		start := time.Now()
		err = b.VMMS.CreateVM(vm)
		if err != nil {
			pretty_log.TaskResultBad("[%s] Failed to create %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}

		pretty_log.TaskGroup("[%s] Waiting for %s VM to be running", b.Environment.Name, name)
		err = b.VMMS.WaitForRunningVM(vm.Name)
		if err != nil {
			pretty_log.TaskResultBad("[%s] Failed to wait for %s VM to be running: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}
		running := time.Now()

		pretty_log.TaskGroup("[%s] Deleting %s VM", b.Environment.Name, name)
		err = b.VMMS.DeleteVM(vm.Name)
		if err != nil {
			pretty_log.TaskResultBad("[%s] Failed to delete %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}
		end := time.Now()

		pretty_log.TaskGroup("[%s] Getting metrics for %s VM", b.Environment.Name, name)
		metrics, err := b.StopMetricScrapers()
		if err != nil {
			pretty_log.TaskResultBad("[%s] Failed to get metrics for %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}

		testRes.Timers = map[string]time.Time{
			TimerStarted:   start,
			TimerFinished:  end,
			TimerVmRunning: running,
		}
		testRes.Metrics = metrics

		res = append(res, testRes)
	}

	return res
}

func (b *Benchmark) CreateManyTinyVMs() []models.TestResult {
	n := 20

	vms := make([]*models.VM, n)
	for i := 0; i < len(vms); i++ {
		vms[i] = TinyVM()
	}

	pretty_log.TaskGroup("[%s] Setting up metrics for %d tiny VMs", b.Environment.Name, n)
	err := b.StartMetricScrapers()
	if err != nil {
		return []models.TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   err,
			},
		}
	}

	pretty_log.TaskGroup("[%s] Creating %d tiny VMs", b.Environment.Name, n)
	wg := sync.WaitGroup{}
	mut := sync.RWMutex{}
	var anyErr error

	start := time.Now()
	for _, vm := range vms {
		wg.Add(1)
		go func(vm *models.VM) {
			defer wg.Done()
			err := b.VMMS.CreateVM(vm)
			if err != nil {
				pretty_log.TaskResult("[%s] Failed to create VM %s: %s", b.Environment.Name, vm.Name, err.Error())
				mut.Lock()
				anyErr = err
				mut.Unlock()
				return
			}
		}(vm)
	}
	wg.Wait()

	if anyErr != nil {
		return []models.TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   anyErr,
			},
		}
	}

	pretty_log.TaskGroup("[%s] Waiting for %d tiny VMs to be running", b.Environment.Name, n)
	for _, vm := range vms {
		wg.Add(1)
		go func(vm *models.VM) {
			defer wg.Done()
			err := b.VMMS.WaitForRunningVM(vm.Name)
			if err != nil {
				pretty_log.TaskResult("[%s] Failed to wait for VM %s to be running: %s", b.Environment.Name, vm.Name, err.Error())
				mut.Lock()
				anyErr = err
				mut.Unlock()
				return
			}
		}(vm)
	}
	wg.Wait()
	running := time.Now()

	if anyErr != nil {
		return []models.TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   anyErr,
			},
		}
	}

	pretty_log.TaskGroup("[%s] Deleting %d tiny VMs", b.Environment.Name, n)
	for _, vm := range vms {
		wg.Add(1)

		go func(vm *models.VM) {
			defer wg.Done()
			err := b.VMMS.DeleteVM(vm.Name)
			if err != nil {
				pretty_log.TaskResult("[%s] Failed to delete VM %s: %s", b.Environment.Name, vm.Name, err.Error())
				mut.Lock()
				anyErr = err
				mut.Unlock()
				return
			}
		}(vm)
	}
	wg.Wait()
	end := time.Now()

	if anyErr != nil {
		return []models.TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   anyErr,
			},
		}
	}

	pretty_log.TaskGroup("[%s] Getting metrics for %d tiny VMs", b.Environment.Name, n)
	metrics, err := b.StopMetricScrapers()
	if err != nil {
		return []models.TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   err,
			},
		}
	}

	return []models.TestResult{
		{
			Name:     "create-many-tiny-vms",
			Group:    "create-many-tiny-vms",
			Metadata: map[string]string{"n": strconv.Itoa(n)},
			Timers:   map[string]time.Time{TimerStarted: start, TimerFinished: end, TimerVmRunning: running},
			Metrics:  metrics,
		},
	}
}

func (b *Benchmark) CreateSnapshot() []models.TestResult {
	return nil
}

func (b *Benchmark) LiveMigrate() []models.TestResult {
	return nil
}

func (b *Benchmark) ScaleUpCluster() []models.TestResult {
	return nil
}

func (b *Benchmark) ScaleDownCluster() []models.TestResult {
	return nil
}

func (b *Benchmark) ScaleDownClusterWithVMs() []models.TestResult {
	return nil
}
