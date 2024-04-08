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

	token string
}

func (o *KubeVirt) Install() error {
	pretty_log.TaskGroup("[KubeVirt] Install control node")

	// Commands to set up KubeVirt with K3s on control node
	controlCommandGroups := [][]string{
		{"sudo apt-get update"},
		{"sudo apt-get install nfs-common -y"},
		{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.7+k3s1 INSTALL_K3S_EXEC=\"server --tls-san " + o.Environment.ControlNode.InternalIP + " --advertise-address " + o.Environment.ControlNode.InternalIP + " --write-kubeconfig-mode=644 --disable=traefik --disable=servicelb\" sh -"},
		{"sleep 10"},
		{"mkdir -p /home/" + app.Config.Azure.Username + "/.kube && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config && sudo chown " + app.Config.Azure.Username + " /home/" + app.Config.Azure.Username + "/.kube/config && sudo chmod 600 /home/" + app.Config.Azure.Username + "/.kube/config"},
	}

	for idx, cmdGroup := range controlCommandGroups {
		id := pretty_log.BeginTask("[KubeVirt] - Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))
		_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, cmdGroup)
		if err != nil {
			pretty_log.FailTask(id)
			return err
		}

		pretty_log.CompleteTask(id)
	}

	id := pretty_log.BeginTask("[KubeVirt] Fetching token from control node")
	outputList, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{"sudo cat /var/lib/rancher/k3s/server/node-token"})
	if err != nil {
		pretty_log.FailTask(id)
		return err
	}
	o.token = strings.TrimSuffix(outputList[0], "\n")
	pretty_log.CompleteTask(id)

	pretty_log.TaskGroup("[KubeVirt] Install worker nodes")

	// Commands to set up KubeVirt with K3s on worker nodes
	workerCommandGroups := [][]string{
		{"sudo apt-get update"},
		{"sudo apt-get install nfs-common -y"},
		{"curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC=\"--node-label kubevirt=kubevirt \" INSTALL_K3S_SKIP_START=true INSTALL_K3S_VERSION=v1.28.7+k3s1 K3S_URL=https://" + o.Environment.ControlNode.InternalIP + ":6443 K3S_TOKEN=" + o.token + " sh -"},
	}

	if app.Config.KubeVirt.Workers > 0 {
		for idx, cmdGroup := range workerCommandGroups {
			id = pretty_log.BeginTask("[KubeVirt] - Command (%d/%d): %s", idx+1, len(workerCommandGroups), strings.Join(cmdGroup, " && "))
			for _, worker := range o.Environment.WorkerNodes {
				_, err = utils.SshCommand(worker.PublicIP, cmdGroup)
				if err != nil {
					pretty_log.FailTask(id)
					return err
				}

				pretty_log.CompleteTask(id)
			}
		}
	} else {
		pretty_log.TaskResult("[KubeVirt] No worker nodes to setup")
	}

	// Install KubeVirt on control node
	pretty_log.TaskGroup("[KubeVirt] Installing KubeVirt and virtctl on control node")
	controlCommandGroups = [][]string{
		{"kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/" + app.Config.KubeVirt.Version + "/kubevirt-operator.yaml > /dev/null"},
		{"kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/" + app.Config.KubeVirt.Version + "/kubevirt-cr.yaml > /dev/null"},
		{"kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/" + app.Config.KubeVirt.CDI.Version + "/cdi-operator.yaml > /dev/null"},
		{"kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/" + app.Config.KubeVirt.CDI.Version + "/cdi-cr.yaml > /dev/null"},
		{"wget https://github.com/kubevirt/kubevirt/releases/download/" + app.Config.KubeVirt.Virtctl.Version + "/virtctl-" + app.Config.KubeVirt.Virtctl.Version + "-linux-amd64 -O virtctl && sudo install virtctl /usr/local/bin/virtctl && rm virtctl"},
	}

	for idx, cmdGroup := range controlCommandGroups {
		id = pretty_log.BeginTask("[KubeVirt] - Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

		_, err = utils.SshCommand(o.Environment.ControlNode.PublicIP, cmdGroup)
		if err != nil {
			pretty_log.FailTask(id)
			return err
		}

		pretty_log.CompleteTask(id)
	}

	// Wait for KubeVirt to be deployed
	id = pretty_log.BeginTask("[KubeVirt] Waiting for KubeVirt to be deployed (max 300 seconds)")
	for i := 0; i < 300; i++ {
		// kubectl get kubevirt.kubevirt.io/kubevirt -n kubevirt -o=jsonpath="{.status.phase}" == Deployed
		res, _ := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{"kubectl get kubevirt.kubevirt.io/kubevirt -n kubevirt -o=jsonpath=\"{.status.phase}\""})
		if res != nil && res[0] == "Deployed" {
			break
		}

		time.Sleep(1 * time.Second)
	}
	pretty_log.CompleteTask(id)

	pretty_log.TaskGroup("[KubeVirt] Set up NFS, mounts and CSI Driver on control node")
	nfsServerIP := o.Environment.ControlNode.InternalIP
	nfsPaths := map[string]string{
		"disks":     "/mnt/nfs/disks",
		"snapshots": "/mnt/nfs/snapshots",
	}

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

	nfsCommands := [][]string{
		// NFS and mounts
		{"sudo apt-get update -y"},
		{"sudo apt-get install nfs-kernel-server -y"},
		{"sudo mkdir -p /mnt/nfs /mnt/nfs/disks /mnt/nfs/snapshots"},
		{"echo \"/mnt/nfs *(rw,sync,no_subtree_check,no_root_squash)\" | sudo tee /etc/exports > /dev/null"},
		{"sudo exportfs -a"},

		// CSI driver
		{"curl -skSL https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/v4.6.0/deploy/install-driver.sh | bash -s v4.6.0 --"},

		// Storage classes
		{"kubectl apply -f - <<EOF" + volumeSnapshotClassCRDs + "EOF"},
		{"kubectl get storageclass vm-disks &> /dev/null || kubectl apply -f - <<EOF " + storageClass + "EOF"},
	}

	for idx, cmdGroup := range nfsCommands {
		id = pretty_log.BeginTask("[KubeVirt] - Command (%d/%d): %s", idx+1, len(nfsCommands), strings.Join(cmdGroup, " && "))
		_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, cmdGroup)
		if err != nil {
			pretty_log.FailTask(id)
			return err
		}

		pretty_log.CompleteTask(id)
	}

	return nil
}

func (o *KubeVirt) Setup() error {
	pretty_log.TaskGroup("[KubeVirt] Nothing to setup")
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
      nodeSelector:
        kubevirt: kubevirt
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

func (o *KubeVirt) ConnectWorker(workerIdx int) error {
	// run sudo systemctl restart k3s-agent
	_, err := utils.SshCommand(o.Environment.WorkerNodes[workerIdx].PublicIP, []string{"sudo systemctl restart k3s-agent"})
	if err != nil {
		return err
	}

	return nil
}

func (o *KubeVirt) DisconnectWorker(workerIdx int) error {
	// Check if node is already disconnected
	workerName := *o.Environment.WorkerNodes[workerIdx].VM.Name
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{"kubectl get node " + workerName})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}

		return err
	}

	commands := []string{
		"kubectl drain " + *o.Environment.WorkerNodes[workerIdx].VM.Name + " --ignore-daemonsets --delete-local-data",
		"kubectl delete node " + *o.Environment.WorkerNodes[workerIdx].VM.Name,
	}

	for _, cmd := range commands {
		_, err = utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{cmd})
		if err != nil {
			return err
		}
	}

	//_, err = utils.SshCommand(o.Environment.WorkerNodes[workerIdx].PublicIP, []string{"sudo k3s-agent-uninstall.sh"})
	//if err != nil {
	//	if strings.Contains(err.Error(), "command not found") {
	//		return nil
	//	}
	//
	//	return err
	//}

	return nil
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

func (o *KubeVirt) DeleteAllVMs() error {
	deleteAllVmsCommand := "kubectl delete vms --all"
	_, err := utils.SshCommand(o.Environment.ControlNode.PublicIP, []string{deleteAllVmsCommand})
	return err
}
