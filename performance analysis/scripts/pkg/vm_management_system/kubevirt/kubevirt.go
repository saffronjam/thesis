package kubevirt

import (
	"fmt"
	"performance/models"
	"performance/pkg/app"
	"performance/pkg/app/pretty_log"
	"performance/pkg/vm_management_system"
	"performance/utils"
	"strings"
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
	pretty_log.TaskGroup("Setting up KubeVirt")

	pretty_log.BeginTask("Set up NFS mounts on control node")
	nfsCommands := [][]string{
		{"sudo apt-get update -y"},
		{"sudo apt-get install nfs-kernel-server -y"},
		{"sudo mkdir -p /mnt/nfs /mnt/nfs/disks /mnt/nfs/snapshots"},
		{"echo \"/mnt/nfs *(rw,sync,no_subtree_check)\" | sudo tee /etc/exports > /dev/null"},
		{"sudo exportfs -a"},
	}

	for _, command := range nfsCommands {
		_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, command)
		if err != nil {
			pretty_log.FailTask()
			return err
		}
	}
	pretty_log.CompleteTask()

	pretty_log.BeginTask("Install NFS CSI driver")
	csiDriverCommands := [][]string{
		{"curl -skSL https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/v4.6.0/deploy/install-driver.sh | bash -s v4.6.0 --"},
	}

	for _, command := range csiDriverCommands {
		_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, command)
		if err != nil {
			pretty_log.FailTask()
			return err
		}
	}
	pretty_log.CompleteTask()

	nfsServerIP := o.Environment.ControlNode.InternalIP
	nfsPaths := map[string]string{
		"disks":     "/mnt/nfs/disks",
		"snapshots": "/mnt/nfs/snapshots",
	}

	pretty_log.BeginTask("Create storage classes")

	volumeSnapshotClassCRDs := `
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: snapshot-controller
  namespace: kube-system
spec:
  repo: https://rke2-charts.rancher.io
  chart: rke2-snapshot-controller
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: snapshot-controller-crd
  namespace: kube-system
spec:
  repo: https://rke2-charts.rancher.io
  chart: rke2-snapshot-controller-crd
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: snapshot-validation-webhook
  namespace: kube-system
spec:
  repo: https://rke2-charts.rancher.io
  chart: rke2-snapshot-validation-webhook
`

	storageClass := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vm-disks
provisioner: nfs.csi.k8s.io
parameters:
  server: %s
  share: %s
`, nfsServerIP, nfsPaths["disks"])

	volumeSnapshotClass := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: vm-snapshots
driver: nfs.csi.k8s.io
deletionPolicy: Delete
parameters:
  server: %s
  share: %s
`, nfsServerIP, nfsPaths["snapshots"])

	storageClasses := [][]string{
		{"kubectl apply -f - <<EOF" + volumeSnapshotClassCRDs + "EOF"},
		{"kubectl get storageclass vm-disks &> /dev/null || kubectl apply -f - <<EOF " + storageClass + "EOF"},
		{"kubectl get volumesnapshotclass vm-snapshots &> /dev/null || kubectl apply -f - <<EOF" + volumeSnapshotClass + "EOF"},
	}

	for _, command := range storageClasses {
		_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, command)
		if err != nil {
			pretty_log.FailTask()
			return err
		}
	}
	pretty_log.CompleteTask()

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

func (o *KubeVirt) CreateVM(vm *models.VM) *models.VM {
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
            name: datavolume-disk
          interfaces:
          - name: default
            masquerade: {}
        resources:
          requests:
            memory: %dMi
      networks:
      - pod: {}
        name: default
      volumes:
      - dataVolume:
          name: %s-dv
        name: datavolume-disk
  dataVolumeTemplates:
  - metadata:
      name: %s-dv
    spec:
      pvc:
        storageClassName: vm-disks
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: %dGi
      source:
        registry:
          url: %s
`, vm.Name, vm.Specs.RAM, vm.Name, vm.Name, vm.Specs.DiskSize, app.Config.KubeVirt.Image.URL)

	createVmCommand := "kubectl apply -f - <<EOF\n" + manifest + "\nEOF"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{createVmCommand})
	if err != nil {
		return nil
	}

	return o.GetVM(vm.Name)
}

func (o *KubeVirt) DeleteVM(name string) {
	deleteVmCommand := "kubectl delete vm " + name
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteVmCommand})
	if err != nil {
		return
	}
}

func (o *KubeVirt) DeleteAllVMs() {
	deleteAllVmsCommand := "kubectl delete vms --all"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteAllVmsCommand})
	if err != nil {
		return
	}
}

func (o *KubeVirt) WaitForRunningVM(name string) {
}

func (o *KubeVirt) WaitForDeletedVM(name string) {
	existsCommand := "kubectl get vm " + name
	for {
		res, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{existsCommand})
		if err != nil {
			return
		}

		if err != nil && res[0] == "" && strings.Contains(res[0], "NotFound") {
			return
		}
	}
}
