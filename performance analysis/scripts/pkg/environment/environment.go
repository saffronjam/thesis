package environment

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/melbahja/goph"
	"performance/pkg/azure"
	"performance/pkg/config"
	"performance/pkg/pretty_log"
	"strconv"
)

type ControlNode struct {
	VM         armcompute.VirtualMachine
	InternalIP string
	PublicIP   string
}

type WorkerNode struct {
	VM         armcompute.VirtualMachine
	InternalIP string
	PublicIP   string
}

type Environment struct {
	ResourceGroup string
	ControlNode   ControlNode
	WorkerNodes   []WorkerNode
}

// Setup initializes a base environment in Azure
func Setup(opennebula, kubevirt bool, workers int) error {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   config.Config.Azure.AuthLocation,
		SubscriptionID: config.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return err
	}

	if opennebula {
		pretty_log.TaskGroup("Creating OpenNebula environment")
		_, err = setupEnvironment(context.TODO(), client, "opennebula", workers)
		if err != nil {
			return err
		}

		_ = []string{
			"wget -q -O- https://downloads.opennebula.org/repo/repo.key | apt-key add -",
			"echo \"deb https://downloads.opennebula.org/repo/5.6/Ubuntu/18.04 stable opennebula\" | tee /etc/apt/sources.list.d/opennebula.list",
			"apt-get update -y",
			"apt-get install opennebula opennebula-sunstone opennebula-gate opennebula-flow -y",
			"/usr/share/one/install_gems",
			"systemctl start opennebula",
			"systemctl enable opennebula",
			"systemctl start opennebula-sunstone",
			"systemctl enable opennebula-sunstone",
		}

	}

	if kubevirt {
		pretty_log.TaskGroup("Creating KubeVirt environment")
		_, err = setupEnvironment(context.TODO(), client, "kubevirt", workers)
		if err != nil {
			return err
		}
	}

	return nil
}

// Shutdown cleans up the base environment in Azure and deletes all the resources
func Shutdown(opennebula, kubevirt bool) error {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   config.Config.Azure.AuthLocation,
		SubscriptionID: config.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return err
	}

	if opennebula {
		pretty_log.TaskGroup("Shutting down OpenNebula environment")
		err = deleteEnvironment(context.TODO(), client, "opennebula")
		if err != nil {
			return err
		}
	}

	if kubevirt {
		pretty_log.TaskGroup("Shutting down KubeVirt environment")
		err = deleteEnvironment(context.TODO(), client, "kubevirt")
		if err != nil {
			return err
		}
	}

	return nil
}

// Increase increases the number of workers in the environment
func Increase(opennebula, kubevirt bool, increaseTo int) error {
	return nil
}

// Decrease decreases the number of workers in the environment
func Decrease(opennebula, kubevirt bool, decreaseTo int) error {
	return nil
}

// SSH executes an SSH command on the given VM
func SSH(vm *armcompute.VirtualMachine, command string) (string, error) {
	client, err := goph.New("root", "192.1.1.3", goph.Password("you_password_here"))
	if err != nil {
		return "", err
	}

	out, err := client.Run(command)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func setupEnvironment(ctx context.Context, client *azure.Client, namePrefix string, workers int) (*Environment, error) {
	pretty_log.BeginTask("Creating resource group")
	resourceGroup, err := client.CreateResourceGroup(ctx, config.Config.Azure.ResourceGroupBaseName+"-"+namePrefix)
	if err != nil {
		return nil, err
	}

	prefixed := func(name string) string {
		return namePrefix + "-" + name
	}

	pretty_log.EndTask()

	rg := *resourceGroup.Name

	pretty_log.BeginTask("Creating virtual network")
	_, err = client.CreateVirtualNetwork(ctx, prefixed("vnet"), rg, "10.0.0.0/8")
	if err != nil {
		return nil, err
	}
	pretty_log.EndTask()

	pretty_log.BeginTask("Creating subnet")
	subnet, err := client.CreateSubnet(ctx, prefixed("subnet"), rg, prefixed("vnet"), "10.1.0.0/16")
	if err != nil {
		return nil, err
	}
	pretty_log.EndTask()

	_, err = client.CreateNetworkSecurityGroup(ctx, prefixed("nsg"), rg)
	if err != nil {
		return nil, err
	}

	pretty_log.BeginTask("Creating public IP")
	controlPublicIP, err := client.CreatePublicIP(ctx, prefixed("ip"), rg)
	if err != nil {
		return nil, err
	}
	pretty_log.EndTask()
	pretty_log.TaskResult("Public IP: %s", *controlPublicIP.Properties.IPAddress)

	pretty_log.BeginTask("Creating control node NIC")
	controlNIC, err := client.CreateNIC(ctx, prefixed("nic-1"), rg, *subnet.ID, controlPublicIP.ID)
	if err != nil {
		return nil, err
	}
	pretty_log.EndTask()
	pretty_log.TaskResult("Internal IP: %s", *controlNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress)

	pretty_log.BeginTask("Creating control node VM")
	_, err = client.CreateVM(ctx, prefixed("control-1"), rg, *controlNIC.ID, prefixed("control-1"), "thesis", "Thesis1!")
	if err != nil {
		return nil, err
	}
	pretty_log.EndTask()

	workerNodes := make([]WorkerNode, workers)

	for i := 0; i < workers; i++ {
		pretty_log.BeginTask("Creating worker node %d public IP", i+1)
		workerPublicIP, err := client.CreatePublicIP(ctx, prefixed("worker-ip-"+strconv.Itoa(i+1)), rg)
		if err != nil {
			return nil, err
		}
		pretty_log.EndTask()
		pretty_log.TaskResult("Public IP: %s", *workerPublicIP.Properties.IPAddress)

		pretty_log.BeginTask("Creating worker node %d NIC", i+1)
		workerNIC, err := client.CreateNIC(ctx, "worker-nic-"+strconv.Itoa(i+1), rg, *subnet.ID, workerPublicIP.ID)
		if err != nil {
			return nil, err
		}
		pretty_log.EndTask()
		pretty_log.TaskResult("Internal IP: %s", *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress)

		pretty_log.BeginTask("Creating worker node %d VM", i+1)
		vm, err := client.CreateVM(ctx, prefixed("worker-"+strconv.Itoa(i+1)), rg, *workerNIC.ID, prefixed("worker-"+strconv.Itoa(i+1)), "thesis", "Thesis1!")
		if err != nil {
			return nil, err
		}
		pretty_log.EndTask()

		workerNodes[i] = WorkerNode{
			VM:         *vm,
			InternalIP: *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *workerPublicIP.Properties.IPAddress,
		}
	}

	return &Environment{
		ResourceGroup: rg,
		ControlNode: ControlNode{
			InternalIP: *controlNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *controlPublicIP.Properties.IPAddress,
		},
		WorkerNodes: workerNodes,
	}, nil
}

func deleteEnvironment(ctx context.Context, client *azure.Client, namePrefix string) error {
	rg := config.Config.Azure.ResourceGroupBaseName + "-" + namePrefix

	pretty_log.BeginTask("Deleting resource group (including all resources)")
	err := client.DeleteResourceGroup(ctx, rg)
	if err != nil {
		return err
	}
	pretty_log.EndTask()

	return nil
}
