package kubevirt

import (
	"fmt"
	"performance/models"
	"performance/pkg/app"
	"performance/pkg/app/pretty_log"
	"performance/pkg/vm_management_system"
	"performance/utils"
	"strings"
	"time"
)

func New(environment *models.AzureEnvironment) *KubeVirt {
	return &KubeVirt{
		Environment: environment,
	}
}

type KubeVirt struct {
	Environment *models.AzureEnvironment

	vm_management_system.VmManagementSystem
}

func (o *KubeVirt) Setup() error {
	pretty_log.TaskResult("[KubeVirt] Nothing to do")
	return nil
}

func (o *KubeVirt) GetVM(name string) *models.VM {
	// Parse out CPU cores, RAM, and disk size to a VM struct { name: string, specs: { cpu: int, ram: int, diskSize: int } }. Use jq to parse the output of the command.
	getVmCommand := "kubectl get vm " + name + " -o json | jq '{name: .metadata.name, specs: {cpu: .spec.template.spec.domain.cpu.cores, ram: .spec.template.spec.domain.resources.requests.memory, diskSize: .spec.dataVolumeTemplates[0].spec.pvc.resources.requests.storage}}'"
	res, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{getVmCommand})
	if err != nil {
		return nil
	}

	vm, err := utils.ParseSshOutput[models.VM](res)
	if err != nil {
		return nil
	}

	return vm
}

func (o *KubeVirt) ListVMs() []models.VM {
	// Parse out a list of VMs to []VM structs { name: string, specs: { cpu: int, ram: int, diskSize: int } }. Use jq to parse the output of the command.
	listVmsCommand := "kubectl get vms -o json | jq '[.items[] | {name: .metadata.name, specs: {cpu: .spec.template.spec.domain.cpu.cores, ram: .spec.template.spec.domain.resources.requests.memory, diskSize: .spec.dataVolumeTemplates[0].spec.pvc.resources.requests.storage}}]'"
	res, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{listVmsCommand})
	if err != nil {
		return nil
	}

	vms, err := utils.ParseSshOutput[[]models.VM](res)
	if err != nil {
		return nil
	}

	return *vms
}

func (o *KubeVirt) CreateVM(vm *models.VM, hostIdx ...int) error {
	manifest := fmt.Sprintf(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %s
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          rng: {}
          disks:
          - disk:
              bus: virtio
            name: containerdisk
          - disk:
              bus: virtio
            name: emptydisk
          interfaces:
          - name: default
            masquerade: {}
        resources:
          requests:
            cpu: 100m
            memory: %dMi
          limits:
            cpu: %d
            memory: %dMi
      networks:
      - pod: {}
        name: default
      volumes:
      - name: emptydisk
        emptyDisk:
          capacity: %dGi
      - name: containerdisk
        containerDisk:
          image: %s
`, vm.Name, vm.Specs.RAM, vm.Specs.CPU, vm.Specs.RAM, vm.Specs.DiskSize, app.Config.KubeVirt.Image.URL)

	createVmCommand := "kubectl apply -f - <<EOF\n" + manifest + "\nEOF"

	// Sometimes the command fails with "connection reset by peer" or "EOF" since we create too many too quickly.
	// If that is the case, we simply try again.
	var err error
	for {
		_, err = utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{createVmCommand})
		if err != nil {
			if strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "EOF") {
				continue
			}

			return err
		}

		return nil
	}
}

func (o *KubeVirt) DeleteVM(name string) error {
	deleteVmCommand := "kubectl delete vm " + name

	// Sometimes the command fails with "connection reset by peer" or "EOF" since we create too many too quickly.
	// If that is the case, we simply try again.
	var err error
	for {
		_, err = utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteVmCommand})
		if err != nil {
			if strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "EOF") {
				continue
			}

			return err
		}

		return nil
	}
}

func (o *KubeVirt) DeleteAllVMs() error {
	deleteAllVmsCommand := "kubectl delete vms --all"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteAllVmsCommand})
	return err
}

func (o *KubeVirt) MigrateVM(name string, hostIdx int) error {
	return nil
}

func (o *KubeVirt) WaitForRunningVM(name string) error {
	runningCommand := "kubectl get vm " + name + " -o jsonpath='{.status.printableStatus}'"
	attemptsLeft := 1000
	for {
		time.Sleep(100 * time.Millisecond)
		attemptsLeft--
		res, _ := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{runningCommand})
		if len(res) == 0 {
			continue
		}

		if res[0] == "Running" {
			return nil
		}

		if attemptsLeft <= 0 {
			return fmt.Errorf("timeout waiting for VM %s to be running", name)
		}
	}
}

func (o *KubeVirt) WaitForAccessibleVM(name string) error {
	runningCommand := "ssh -o PasswordAuthentication=no -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o 'ProxyCommand=virtctl port-forward --stdio=true vmi/" + name + " 22' cirros@vmi/" + name + " 'sudo echo hello' 2>&1 | grep 'Permission denied (publickey,password)' && exit 0 || exit 1"
	attemptsLeft := 1000
	for {
		time.Sleep(100 * time.Millisecond)
		attemptsLeft--
		res, _ := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{runningCommand})
		if len(res) == 0 {
			continue
		}

		if strings.Contains(res[0], "Permission denied (publickey,password)") {
			return nil
		}

		if attemptsLeft <= 0 {
			return fmt.Errorf("timeout waiting for VM %s to be running", name)
		}
	}
}

func (o *KubeVirt) WaitForDeletedVM(name string) error {
	existsCommand := "kubectl get vm " + name
	attemptsLeft := 1000
	for {
		time.Sleep(100 * time.Millisecond)
		attemptsLeft--
		res, _ := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{existsCommand})
		if len(res) == 0 {
			continue
		}

		if res[0] == "" && strings.Contains(res[0], "NotFound") {
			return nil
		}

		if attemptsLeft <= 0 {
			return fmt.Errorf("timeout waiting for VM %s to be running", name)
		}
	}
}
