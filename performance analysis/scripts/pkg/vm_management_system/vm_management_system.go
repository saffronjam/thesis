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

	GetVM(name string) *models.VM
	ListVMs() []models.VM
	CreateVM(name string) *models.VM
	DeleteVM(name string)

	// DeleteAllVMs deletes all VMs in the environment
	// It should be treated as a cleanup operation, and not be included in any benchmarking
	DeleteAllVMs()
}
