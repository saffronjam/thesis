package benchmark

import (
	"performance/models"
	"performance/pkg/app/pretty_log"
	"strconv"
	"sync"
	"time"
)

const (
	TimerStarted   = "started"
	TimerFinished  = "finished"
	TimerVmRunning = "vm-running"
)

type TestDefinition struct {
	Name string
	Func func() []TestResult
}

type TestResult struct {
	Name     string
	Group    string
	Metadata map[string]string

	Timers      map[string]time.Time
	CpuUsage    []float64
	MemoryUsage []float64
	DiskUsage   []float64

	Err error
}

func (b *Benchmark) AllTests() []TestDefinition {
	return []TestDefinition{
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

func RunTests(vmm string, tests []TestDefinition) map[string][]TestResult {
	results := make(map[string][]TestResult)

	for idx, test := range tests {
		pretty_log.TaskGroup("[%s] Running test %d/%d: %s", vmm, idx+1, len(tests), test.Name)

		res := test.Func()
		if _, ok := results[test.Name]; !ok {
			results[test.Name] = make([]TestResult, 0)
		}

		groupResults := results[test.Name]
		groupResults = append(groupResults, res...)
		results[test.Name] = groupResults
	}

	return results
}

func (b *Benchmark) CreateEachType() []TestResult {
	vms := map[string]*models.VM{
		"tiny":   TinyVM(),
		"small":  SmallVM(),
		"medium": MediumVM(),
		"large":  LargeVM(),
	}

	var res []TestResult
	for name, vm := range vms {
		testRes := TestResult{
			Name:  "create-" + name,
			Group: "create-each-type",
		}

		pretty_log.TaskGroup("[%s] Creating %s VM", b.Environment.Name, name)
		start := time.Now()
		err := b.VMMS.CreateVM(vm)
		if err != nil {
			pretty_log.TaskResult("[%s] Failed to create %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}

		pretty_log.TaskGroup("[%s] Waiting for %s VM to be running", b.Environment.Name, name)
		err = b.VMMS.WaitForRunningVM(vm.Name)
		if err != nil {
			pretty_log.TaskResult("[%s] Failed to wait for %s VM to be running: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}
		running := time.Now()

		pretty_log.TaskGroup("[%s] Deleting %s VM", b.Environment.Name, name)
		err = b.VMMS.DeleteVM(vm.Name)
		if err != nil {
			pretty_log.TaskResult("[%s] Failed to delete %s VM: %s", b.Environment.Name, name, err.Error())
			testRes.Err = err
			res = append(res, testRes)
			continue
		}
		end := time.Now()

		testRes.Timers = map[string]time.Time{
			TimerStarted:   start,
			TimerFinished:  end,
			TimerVmRunning: running,
		}
		testRes.CpuUsage = []float64{0.0}
		testRes.MemoryUsage = []float64{0.0}
		testRes.DiskUsage = []float64{0.0}

		res = append(res, testRes)
	}

	return res
}

func (b *Benchmark) CreateManyTinyVMs() []TestResult {
	n := 20

	vms := make([]*models.VM, n)
	for i := 0; i < len(vms); i++ {
		vms[i] = TinyVM()
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
		return []TestResult{
			{
				Name:  "create-many-tiny-vms",
				Group: "create-many-tiny-vms",
				Err:   anyErr,
			},
		}
	}

	return []TestResult{
		{
			Name:        "create-many-tiny-vms",
			Group:       "create-many-tiny-vms",
			Metadata:    map[string]string{"n": strconv.Itoa(n)},
			Timers:      map[string]time.Time{TimerStarted: start, TimerFinished: end, TimerVmRunning: running},
			CpuUsage:    []float64{0.0},
			MemoryUsage: []float64{0.0},
			DiskUsage:   []float64{0.0},
		},
	}
}

func (b *Benchmark) CreateSnapshot() []TestResult {
	return nil
}

func (b *Benchmark) LiveMigrate() []TestResult {
	return nil
}

func (b *Benchmark) ScaleUpCluster() []TestResult {
	return nil
}

func (b *Benchmark) ScaleDownCluster() []TestResult {
	return nil
}

func (b *Benchmark) ScaleDownClusterWithVMs() []TestResult {
	return nil
}
