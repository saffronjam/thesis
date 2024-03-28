package benchmark

import (
	"performance/models"
	"time"
)

type TestDefinition struct {
	Name string
	Func func() error
}

type TestResult struct {
	Name     string
	Result   error
	Duration time.Duration
}

func (b *Benchmark) AllTests() []TestDefinition {
	return []TestDefinition{
		{
			Name: "CreateEachType",
			Func: b.CreateEachType,
		},
		{
			Name: "Create100TinyVMs",
			Func: b.Create100TinyVMs,
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

func RunTests(tests []TestDefinition) []TestResult {
	results := make([]TestResult, len(tests))

	for idx, test := range tests {
		now := time.Now()
		err := test.Func()
		duration := time.Since(now)

		results[idx] = TestResult{
			Name:     test.Name,
			Result:   err,
			Duration: duration,
		}
	}

	return results
}

func (b *Benchmark) CreateEachType() error {
	vms := []*models.VM{
		TinyVM(),
		SmallVM(),
		MediumVM(),
		LargeVM(),
	}

	for _, vm := range vms {
		b.VMMS.CreateVM(vm)
	}

	// Wait for VMs to be accessible

	for _, vm := range vms {
		b.VMMS.DeleteVM(vm.Name)
	}

	return nil
}

func (b *Benchmark) Create100TinyVMs() error {
	vms := make([]*models.VM, 100)
	for i := 0; i < len(vms); i++ {
		vms[i] = TinyVM()
	}

	for _, vm := range vms {
		_ = b.VMMS.CreateVM(vm)
		b.VMMS.DeleteVM(vm.Name)
	}

	for _, vm := range vms {
		b.VMMS.DeleteVM(vm.Name)
	}

	return nil
}

func (b *Benchmark) CreateSnapshot() error {
	return nil
}

func (b *Benchmark) LiveMigrate() error {
	return nil
}

func (b *Benchmark) ScaleUpCluster() error {
	return nil
}

func (b *Benchmark) ScaleDownCluster() error {
	return nil
}

func (b *Benchmark) ScaleDownClusterWithVMs() error {
	return nil
}
