package opennebula

import (
	"encoding/json"
	"performance/models"
	"performance/pkg/app"
	"performance/pkg/app/pretty_log"
	"performance/pkg/app/utils"
	"performance/pkg/vm_management_system"
)

type OpenNebula struct {
	Environment *models.AzureEnvironment

	vm_management_system.VmManagementSystem
}

func New(environment *models.AzureEnvironment) *OpenNebula {
	return &OpenNebula{
		Environment: environment,
	}
}

func (o OpenNebula) Setup() error {
	pretty_log.TaskGroup("Setting up OpenNebula")
	// Download image
	pretty_log.BeginTask("Setting up image if not present")

	//installIfNotPresent := "if sudo oneimage list --list NAME | grep -w " + app.Config.OpenNebula.Image.Name + " | wc -l | grep -q ^0$; then\n  sudo oneimage create --name " + app.Config.OpenNebula.Image.Name + " --path \"" + app.Config.OpenNebula.Image.URL + "\" --datastore 1 --format qcow2\nelse\n  echo 'Image already exists.'\nfi\nexit 0"
	//installIfNotPresent := "sudo oneimage list --list NAME | grep -w " + app.Config.OpenNebula.Image.Name + " | wc -l | grep -q ^0$ && sudo oneimage create --name " + app.Config.OpenNebula.Image.Name + " --path \"" + app.Config.OpenNebula.Image.URL + "\" --datastore 1 --format qcow2 || true"
	//installIfNotPresent := "sudo oneimage create --name cirros --path http://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img --datastore 1 --format qcow2 &"

	/*
		EOF
		NAME="cirros"
		PATH="http://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img"
		DATASTORE=1
		EOF
	*/
	installIfNotPresent := "if sudo oneimage list --list NAME | grep -w " + app.Config.OpenNebula.Image.Name + " | wc -l | grep -q ^0$; then\n  sudo oneimage create -d 1 <<EOF\nNAME=\"" + app.Config.OpenNebula.Image.Name + "\"\nPATH=\"" + app.Config.OpenNebula.Image.URL + "\"\nEOF\nelse\n  echo 'Image already exists.'\nfi\nexit 0"

	res, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{installIfNotPresent})
	if err != nil {
		pretty_log.FailTask()
		return err
	}
	pretty_log.CompleteTask()
	pretty_log.TaskResultList(res)

	// Create template
	pretty_log.BeginTask("Setting up template if not present")

	installIfNotPresent = "if sudo onetemplate list --list NAME | grep -w " + app.Config.OpenNebula.Template.Name + " | wc -l | grep -q ^0$; then\n  sudo onetemplate create <<EOF\nNAME=\"" + app.Config.OpenNebula.Template.Name + "\"\nDISK=[\n  IMAGE=\"" + app.Config.OpenNebula.Image.Name + "\"\n]\nGRAPHICS=[\n  TYPE=\"VNC\",\n  LISTEN=\"0.0.0.0\"\n]\nEOF\nelse\n  echo 'Template already exists.'\nfi\nexit 0"
	res, err = utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{installIfNotPresent})
	if err != nil {
		pretty_log.FailTask()
		return err
	}
	pretty_log.CompleteTask()
	pretty_log.TaskResultList(res)

	return nil
}

func (o OpenNebula) GetVM(name string) *models.VM {
	getCmd := "onevm list --list NAME --json | jq '.VM_POOL.VM[] | select(.NAME==\"" + name + "\") | {name: .NAME, specs: {cpu: (.TEMPLATE.CPU | tonumber), ram: (.TEMPLATE.MEMORY | tonumber), diskSize: (.TEMPLATE.DISK[0].SIZE | tonumber)}}"
	outputList, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{getCmd})
	if err != nil {
		return nil
	}

	// Parse output as json into a VM
	var vm models.VM
	err = json.Unmarshal([]byte(outputList[0]), &vm)
	if err != nil {
		return nil
	}

	// Disk diskSize is in MB, convert to GB
	vm.Specs.DiskSize = vm.Specs.DiskSize / 1024

	return &vm
}

func (o OpenNebula) ListVMs() []models.VM {
	listCmd := "onevm list --list NAME --json | jq '[.VM_POOL.VM[] | {name: .NAME, specs: {cpu: (.TEMPLATE.CPU | tonumber), ram: (.TEMPLATE.MEMORY | tonumber), diskSize: (.TEMPLATE.DISK[0]?.SIZE | tonumber // 0)}}]'"
	outputList, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{listCmd})
	if err != nil {
		return nil
	}

	// Parse output as json into []VM
	var vms []models.VM
	err = json.Unmarshal([]byte(outputList[0]), &vms)
	if err != nil {
		return nil
	}

	// Disk diskSize is in MB, convert to GB
	for i := range vms {
		vms[i].Specs.DiskSize = vms[i].Specs.DiskSize / 1024
	}

	return vms
}

func (o OpenNebula) CreateVM(name string) *models.VM {
	createCmd := "onetemplate instantiate 2 --name my-vm3 <<EOF\nCPU=\"2\"\nMEMORY=\"2048\"\nDISK=[IMAGE=\"cirros\"]\nEOF"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{createCmd})
	if err != nil {
		return nil
	}

	return o.GetVM(name)
}

func (o OpenNebula) DeleteVM(name string) {
	deleteCmd := "onevm delete " + name
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteCmd})
	if err != nil {
		return
	}
}

func (o OpenNebula) DeleteAllVMs() {
	listCmd := "onevm list --list NAME --json | jq '.VM_POOL.VM[].NAME' | xargs -I {} onevm delete {}"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{listCmd})
	if err != nil {
		return
	}
}
