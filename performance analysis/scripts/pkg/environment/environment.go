package environment

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/melbahja/goph"
	"log"
	"performance/pkg/azure"
	"performance/pkg/config"
	"performance/pkg/pretty_log"
	"strconv"
	"strings"
	"time"
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

type AzureEnvironment struct {
	ResourceGroup string
	ControlNode   ControlNode
	WorkerNodes   []WorkerNode
}

// Setup initializes a base environment in Azure
func Setup(usabilityAnalysis, opennebula, kubevirt bool) error {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   config.Config.Azure.AuthLocation,
		SubscriptionID: config.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return err
	}

	if usabilityAnalysis {
		_, err = SetupAzureEnvironment(context.TODO(), client, "usability", config.Config.Clusters.UsabilityAnalysis.Workers)
		if err != nil {
			log.Fatalln(err.Error())
		}
	}

	if opennebula {
		pretty_log.TaskGroup("Creating OpenNebula environment")
		opennebulaEnv, err := SetupAzureEnvironment(context.TODO(), client, "opennebula", config.Config.Clusters.OpenNebula.Workers)
		if err != nil {
			return err
		}

		pretty_log.TaskGroup("Setting up OpenNebula control node")

		// Commands to set up OpenNebula
		controlCommandGroups := [][]string{
			{"sudo apt-get update"},
			{"sudo apt-get -y install gnupg wget apt-transport-https"},
			{"curl -fsSL https://downloads.opennebula.io/repo/repo2.key | sudo gpg --batch --yes --dearmor -o /etc/apt/trusted.gpg.d/opennebula.gpg"},
			{"sudo echo \"deb https://downloads.opennebula.org/repo/6.8/Ubuntu/22.04 stable opennebula\" | sudo tee /etc/apt/sources.list.d/opennebula.list"},
			{"sudo apt-get update"},
			{"sudo apt-get install -y opennebula opennebula-sunstone opennebula-gate opennebula-flow"},
			{"sudo ufw disable"},
			{"sudo systemctl start opennebula opennebula-sunstone"},
			{"sudo systemctl enable opennebula opennebula-sunstone"},
		}

		for idx, cmdGroup := range controlCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

			outputList, err := SshCommand(opennebulaEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return err
			}

			pretty_log.CompleteTask()

			if outputList != nil {
				for _, output := range outputList {
					pretty_log.TaskResult(output)
				}
			}
		}

		pretty_log.TaskGroup("Setting up OpenNebula worker nodes")

		// Commands to set up OpenNebula on worker nodes
		workerCommandGroups := [][]string{
			{"curl -fsSL https://downloads.opennebula.io/repo/repo2.key | sudo gpg --batch --yes --dearmor -o /etc/apt/trusted.gpg.d/opennebula.gpg"},
			{"sudo echo \"deb https://downloads.opennebula.org/repo/6.8/Ubuntu/22.04 stable opennebula\" | sudo tee /etc/apt/sources.list.d/opennebula.list"},
			{"sudo apt-get update"},
			{"sudo apt-get install -y opennebula-node"},
			{"sudo systemctl restart libvirtd.service"},
		}

		for idx, cmdGroup := range workerCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(workerCommandGroups), strings.Join(cmdGroup, " && "))

			for _, worker := range opennebulaEnv.WorkerNodes {
				outputList, err := SshCommand(worker.PublicIP, cmdGroup)
				if err != nil {
					pretty_log.FailTask()
					return err
				}

				pretty_log.CompleteTask()

				if outputList != nil {
					for _, output := range outputList {
						pretty_log.TaskResult(output)
					}
				}
			}
		}

		// Setup nodes in management server
		pretty_log.TaskGroup("Connect nodes to management server")

		// Commands to allows connections to workers
		controlCommands := [][]string{}
		for _, worker := range opennebulaEnv.WorkerNodes {
			controlCommands = append(controlCommands, []string{"ssh-keyscan " + worker.InternalIP + " | sudo tee -a /var/lib/one/.ssh/known_hosts"})
			controlCommands = append(controlCommands, []string{"ssh " + worker.InternalIP + " \"echo $(sudo cat /var/lib/one/.ssh/id_rsa.pub) | sudo tee -a /var/lib/one/.ssh/authorized_keys > /dev/null\""})
			controlCommands = append(controlCommands, []string{"sudo onehost create " + opennebulaEnv.ControlNode.InternalIP + " -i kvm -v kvm"})
		}

		for idx, cmdGroup := range controlCommands {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommands), strings.Join(cmdGroup, " && "))

			outputList, err := SshCommand(opennebulaEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return err
			}

			pretty_log.CompleteTask()

			if outputList != nil {
				for _, output := range outputList {
					pretty_log.TaskResult(output)
				}
			}
		}
	}

	if kubevirt {
		pretty_log.TaskGroup("Creating KubeVirt environment")
		kubevirtEnv, err := SetupAzureEnvironment(context.TODO(), client, "kubevirt", config.Config.Clusters.KubeVirt.Workers)
		if err != nil {
			return err
		}

		pretty_log.TaskGroup("Setting up KubeVirt control node")

		// Commands to set up KubeVirt with K3s on control node
		controlCommandGroups := [][]string{
			{"sudo apt-get update"},
			{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.7+k3s1 INSTALL_K3S_EXEC=\"server --tls-san " + kubevirtEnv.ControlNode.InternalIP + " --advertise-address " + kubevirtEnv.ControlNode.InternalIP + " --write-kubeconfig-mode=644 --disable=traefik --disable=servicelb\" sh -"},
		}

		for idx, cmdGroup := range controlCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

			outputList, err := SshCommand(kubevirtEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return err
			}

			pretty_log.CompleteTask()

			if outputList != nil {
				for _, output := range outputList {
					pretty_log.TaskResult(output)
				}
			}
		}

		// Fetch token from control node
		pretty_log.BeginTask("Fetching token from control node")
		outputList, err := SshCommand(kubevirtEnv.ControlNode.PublicIP, []string{"sudo cat /var/lib/rancher/k3s/server/node-token"})
		if err != nil {
			pretty_log.FailTask()
			return err
		}
		token := strings.TrimSuffix(outputList[0], "\n")
		pretty_log.CompleteTask()

		pretty_log.TaskGroup("Setting up KubeVirt worker nodes")

		// Commands to set up KubeVirt with K3s on worker nodes
		workerCommandGroups := [][]string{
			{"sudo apt-get update"},
			{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.7+k3s1 K3S_URL=https://" + kubevirtEnv.ControlNode.InternalIP + ":6443 K3S_TOKEN=" + token + " sh -"},
		}

		for idx, cmdGroup := range workerCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(workerCommandGroups), strings.Join(cmdGroup, " && "))

			for _, worker := range kubevirtEnv.WorkerNodes {
				outputList, err = SshCommand(worker.PublicIP, cmdGroup)
				if err != nil {
					pretty_log.FailTask()
					return err
				}

				pretty_log.CompleteTask()

				if outputList != nil {
					for _, output := range outputList {
						pretty_log.TaskResult(output)
					}
				}
			}
		}

		// Install KubeVirt on control node
		pretty_log.BeginTask("Installing KubeVirt on control node")
		controlCommandGroups = [][]string{
			{"kubectl create -f https://github.com/kubevirt/kubevirt/releases/download/" + config.Config.KubeVirt.Version + "/kubevirt-operator.yaml"},
			{"kubectl create -f https://github.com/kubevirt/kubevirt/releases/download/" + config.Config.KubeVirt.Version + "/kubevirt-cr.yaml"},
		}

		for idx, cmdGroup := range controlCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

			outputList, err = SshCommand(kubevirtEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return err
			}

			pretty_log.CompleteTask()

			if outputList != nil {
				for _, output := range outputList {
					pretty_log.TaskResult(output)
				}
			}
		}

		// Wait for KubeVirt to be deployed
		pretty_log.BeginTask("Waiting for KubeVirt to be deployed (max 30 seconds)")
		for i := 0; i < 30; i++ {
			// kubectl get kubevirt.kubevirt.io/kubevirt -n kubevirt -o=jsonpath="{.status.phase}" == Deployed
			res, _ := SshCommand(kubevirtEnv.ControlNode.PublicIP, []string{"kubectl get kubevirt.kubevirt.io/kubevirt -n kubevirt -o=jsonpath=\"{.status.phase}\""})
			if res != nil && res[0] == "Deployed" {
				break
			}

			time.Sleep(1 * time.Second)
		}
		pretty_log.CompleteTask()
	}

	return nil
}

// Shutdown cleans up the base environment in Azure and deletes all the resources
func Shutdown(usabilityAnalysis, opennebula, kubevirt bool) error {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   config.Config.Azure.AuthLocation,
		SubscriptionID: config.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return err
	}

	if usabilityAnalysis {
		pretty_log.TaskGroup("Shutting down Usability Analysis environment")
		err = deleteEnvironment(context.TODO(), client, "usability")
		if err != nil {
			pretty_log.FailTask()
			return err
		}
		pretty_log.CompleteTask()
	}

	if opennebula {
		pretty_log.TaskGroup("Shutting down OpenNebula environment")
		err = deleteEnvironment(context.TODO(), client, "opennebula")
		if err != nil {
			pretty_log.FailTask()
			return err
		}
		pretty_log.CompleteTask()
	}

	if kubevirt {
		pretty_log.TaskGroup("Shutting down KubeVirt environment")
		err = deleteEnvironment(context.TODO(), client, "kubevirt")
		if err != nil {
			pretty_log.FailTask()
			return err
		}
		pretty_log.CompleteTask()
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

// SshCommand executes an SshCommand command on the given VM
func SshCommand(ip string, commands []string) ([]string, error) {
	client, err := goph.NewUnknown(config.Config.Azure.Username, ip, goph.Password(config.Config.Azure.Password))
	if err != nil {
		return nil, err
	}

	var outAll []string

	for _, command := range commands {
		out, err := client.Run(command)
		if err != nil {
			return nil, err
		}

		outAll = append(outAll, string(out))
	}

	return outAll, nil
}

func SetupAzureEnvironment(ctx context.Context, client *azure.Client, namePrefix string, workers int) (*AzureEnvironment, error) {
	pretty_log.BeginTask("Creating resource group")
	resourceGroup, err := client.CreateResourceGroup(ctx, config.Config.Azure.ResourceGroupBaseName+"-"+namePrefix)
	if err != nil {
		return nil, err
	}

	prefixed := func(name string) string {
		return namePrefix + "-" + name
	}

	pretty_log.CompleteTask()

	rg := *resourceGroup.Name

	pretty_log.BeginTask("Creating virtual network")
	_, err = client.CreateVirtualNetwork(ctx, prefixed("vnet"), rg, "10.0.0.0/8")
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	pretty_log.BeginTask("Creating subnet")
	subnet, err := client.CreateSubnet(ctx, prefixed("subnet"), rg, prefixed("vnet"), "10.1.0.0/16")
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	_, err = client.CreateNetworkSecurityGroup(ctx, prefixed("nsg"), rg)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}

	pretty_log.BeginTask("Creating public IP")
	controlPublicIP, err := client.CreatePublicIP(ctx, prefixed("ip"), rg)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()
	pretty_log.TaskResult("Public IP: %s", *controlPublicIP.Properties.IPAddress)

	pretty_log.BeginTask("Creating control node NIC")
	controlNIC, err := client.CreateNIC(ctx, prefixed("nic-1"), rg, *subnet.ID, controlPublicIP.ID)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()
	pretty_log.TaskResult("Internal IP: %s", *controlNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress)

	pretty_log.BeginTask("Creating control node VM")
	_, err = client.CreateVM(ctx, prefixed("control-1"), rg, *controlNIC.ID, prefixed("control-1"), config.Config.Azure.Username, config.Config.Azure.Password, config.Config.Azure.PublicKeys)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	// Wait for VM to boot by sleeping
	pretty_log.BeginTask("Waiting for VM to boot (max 30 seconds)")
	for i := 0; i < 30; i++ {
		res, _ := SshCommand(*controlPublicIP.Properties.IPAddress, []string{"echo \"\""})
		if res != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}
	pretty_log.CompleteTask()

	// Generate SSH key pair for control node non-interactively, 2048 bits, no passphrase, RSA. Don't overwrite
	pretty_log.BeginTask("Generating SSH key pair")
	_, _ = SshCommand(*controlPublicIP.Properties.IPAddress, []string{"ssh-keygen -t rsa -b 2048 -N \"\" -f /home/" + config.Config.Azure.Username + "/.ssh/id_rsa <<< n 1> /dev/null 2> /dev/null"})
	// TODO: Fix me, always returns an error
	//if err != nil {
	//	pretty_log.FailTask()
	//	return nil, err
	//}
	publicKey, err := SshCommand(*controlPublicIP.Properties.IPAddress, []string{"cat /home/" + config.Config.Azure.Username + "/.ssh/id_rsa.pub"})
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}

	pretty_log.CompleteTask()

	workerNodes := make([]WorkerNode, workers)

	for i := 0; i < workers; i++ {
		pretty_log.BeginTask("Creating worker node %d public IP", i+1)
		workerPublicIP, err := client.CreatePublicIP(ctx, prefixed("worker-ip-"+strconv.Itoa(i+1)), rg)
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()
		pretty_log.TaskResult("Public IP: %s", *workerPublicIP.Properties.IPAddress)

		pretty_log.BeginTask("Creating worker node %d NIC", i+1)
		workerNIC, err := client.CreateNIC(ctx, "worker-nic-"+strconv.Itoa(i+1), rg, *subnet.ID, workerPublicIP.ID)
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()
		pretty_log.TaskResult("Internal IP: %s", *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress)

		pretty_log.BeginTask("Creating worker node %d VM", i+1)

		vm, err := client.CreateVM(ctx, prefixed("worker-"+strconv.Itoa(i+1)), rg, *workerNIC.ID, prefixed("worker-"+strconv.Itoa(i+1)), config.Config.Azure.Username, config.Config.Azure.Password,
			append(config.Config.Azure.PublicKeys, publicKey[0]))
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()

		workerNodes[i] = WorkerNode{
			VM:         *vm,
			InternalIP: *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *workerPublicIP.Properties.IPAddress,
		}
	}

	return &AzureEnvironment{
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
	pretty_log.CompleteTask()

	return nil
}
