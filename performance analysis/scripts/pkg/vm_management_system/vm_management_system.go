package vm_management_system

import (
	"performance/models"
)

type VmManagementSystem interface {
	// Setup initializes the VM management system
	// This could include downloading required images and setting up templates
	// It is done in a separate step, since it is a one-time operation
	//
	// This function should be idempotent
	Setup() error

	// GetVM returns the VM with the given name
	GetVM(name string) *models.VM
	// ListVMs returns a list of all VMs in the environment
	ListVMs() []models.VM
	// CreateVM creates a VM with the given specs
	// It does not need to wait for the VM to be running
	CreateVM(vm *models.VM, hostIdx ...int) error
	// DeleteVM deletes the VM with the given name
	// It should be synchronous and wait for the VM to be deleted
	DeleteVM(name string) error
	// MigrateVM migrates a VM to the given host
	MigrateVM(name string, hostIdx int) error

	// WaitForRunningVM waits for the VM to be running
	WaitForRunningVM(name string) error
	// WaitForAccessibleVM waits for the VM to be accessible via SSH
	//WaitForAccessibleVM(name string) error

	// DeleteAllVMs deletes all VMs in the environment
	// It should be treated as a cleanup operation, and not be included in any benchmarking
	DeleteAllVMs() error
}
