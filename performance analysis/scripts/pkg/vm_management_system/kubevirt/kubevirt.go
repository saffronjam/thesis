package kubevirt

import (
	"performance/models"
	"performance/pkg/app/pretty_log"
	"performance/pkg/app/utils"
	"performance/pkg/vm_management_system"
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

	pretty_log.TaskGroup("Set up storage and volume snapshot classes")
	nfsServerIP := o.Environment.ControlNode.InternalIP
	nfsPaths := map[string]string{
		"disks":     "/mnt/nfs/disks",
		"snapshots": "/mnt/nfs/snapshots",
	}

	pretty_log.BeginTask("Create storage classes")
	storageClasses := [][]string{
		{"kubectl apply -f - <<EOF\napiVersion: helm.cattle.io/v1\nkind: HelmChart\nmetadata:\n  name: snapshot-controller\n  namespace: kube-system\nspec:\n  repo: https://rke2-charts.rancher.io\n  chart: rke2-snapshot-controller\n---    \napiVersion: helm.cattle.io/v1\nkind: HelmChart\nmetadata:\n  name: snapshot-controller-crd\n  namespace: kube-system\nspec:\n  repo: https://rke2-charts.rancher.io\n  chart: rke2-snapshot-controller-crd\n--- \napiVersion: helm.cattle.io/v1\nkind: HelmChart\nmetadata:\n  name: snapshot-validation-webhook\n  namespace: kube-system\nspec:\n  repo: https://rke2-charts.rancher.io\n  chart: rke2-snapshot-validation-webhook\nEOF"},
		{"kubectl get storageclass vm-disks &> /dev/null || kubectl apply -f - <<EOF\napiVersion: storage.k8s.io/v1\nkind: StorageClass\nmetadata:\n  name: vm-disks\nprovisioner: nfs.csi.k8s.io\nparameters:\n  server: " + nfsServerIP + "\n  share: " + nfsPaths["disks"] + "\nEOF"},
		{"kubectl get volumesnapshotclass vm-snapshots &> /dev/null || kubectl apply -f - <<EOF\napiVersion: snapshot.storage.k8s.io/v1\nkind: VolumeSnapshotClass\nmetadata:\n  name: vm-snapshots\ndriver: nfs.csi.k8s.io\ndeletionPolicy: Delete\nparameters:\n  server: " + nfsServerIP + "\n  share: " + nfsPaths["snapshots"] + "\nEOF"},
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
	return &models.VM{}
}

func (o *KubeVirt) ListVMs() []models.VM {
	return []models.VM{}
}

func (o *KubeVirt) CreateVM(name string) *models.VM {
	return &models.VM{}
}

func (o *KubeVirt) DeleteVM(name string) {

}

func (o *KubeVirt) DeleteAllVMs() {

}
