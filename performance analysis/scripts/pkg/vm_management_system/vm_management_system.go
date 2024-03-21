package vm_management_system

import "performance/pkg/models"

type VmManagementSystem interface {
	CreateVM(name string) (models.VM, error)
	DeleteVM(name string) error
}
