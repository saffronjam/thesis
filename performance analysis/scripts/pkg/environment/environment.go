package environment

import (
	"context"
	"performance/models"
	"performance/pkg/app"
	"performance/pkg/app/pretty_log"
	"performance/pkg/app/utils"
	"performance/pkg/azure"
	"strconv"
	"strings"
	"time"
)

// Setup initializes a base environment in Azure
func Setup(createOpennebula, createKubevirt bool) ([]models.BenchmarkEnvironment, error) {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   app.Config.Azure.AuthLocation,
		SubscriptionID: app.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return nil, err
	}

	setupResult := make([]models.BenchmarkEnvironment, 0)

	if createOpennebula {
		pretty_log.TaskGroup("Creating OpenNebula environment")
		opennebulaEnv, err := SetupAzureEnvironment(context.TODO(), client, "opennebula", app.Config.OpenNebula.Workers)
		if err != nil {
			return nil, err
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

			outputList, err := utils.SshCommand(opennebulaEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return nil, err
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

		if app.Config.OpenNebula.Workers > 0 {
			for idx, cmdGroup := range workerCommandGroups {
				for jdx, worker := range opennebulaEnv.WorkerNodes {
					pretty_log.BeginTask("- Command (%d/%d) [Worker: %d]: %s", idx+1, len(workerCommandGroups), jdx+1, strings.Join(cmdGroup, " && "))
					outputList, err := utils.SshCommand(worker.PublicIP, cmdGroup)
					if err != nil {
						pretty_log.FailTask()
						return nil, err
					}

					pretty_log.CompleteTask()

					if outputList != nil {
						for _, output := range outputList {
							pretty_log.TaskResult(output)
						}
					}
				}
			}
		} else {
			pretty_log.TaskResult("No worker nodes to setup")
		}

		// Setup nodes in management server
		pretty_log.TaskGroup("Connect nodes to management server")

		// Commands to allows connections to workers
		if app.Config.OpenNebula.Workers > 0 {
			controlCommands := [][]string{}
			for _, worker := range opennebulaEnv.WorkerNodes {
				controlCommands = append(controlCommands, []string{"ssh-keyscan " + worker.InternalIP + " | sudo tee -a /var/lib/one/.ssh/known_hosts"})
				controlCommands = append(controlCommands, []string{"sudo cat /var/lib/one/.ssh/id_rsa.pub | ssh " + worker.InternalIP + " -o StrictHostKeyChecking=no \"sudo tee -a /var/lib/one/.ssh/authorized_keys > /dev/null\""})
				controlCommands = append(controlCommands, []string{"sudo onehost create " + worker.InternalIP + " -i kvm -v kvm 2>&1 | tee /dev/stderr | grep -q 'NAME is already taken' && exit 0 || exit 1"})
			}

			for idx, cmdGroup := range controlCommands {
				pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommands), strings.Join(cmdGroup, " && "))

				outputList, err := utils.SshCommand(opennebulaEnv.ControlNode.PublicIP, cmdGroup)
				if err != nil {
					pretty_log.FailTask()
					return nil, err
				}

				pretty_log.CompleteTask()

				if outputList != nil {
					for _, output := range outputList {
						pretty_log.TaskResult(output)
					}
				}
			}
		} else {
			pretty_log.TaskResult("No worker nodes to connect")
		}

		setupResult = append(setupResult, models.BenchmarkEnvironment{
			Name:        "OpenNebula",
			Environment: opennebulaEnv,
		})
	} else {
		pretty_log.TaskResult("Fetching OpenNebula environment")
		opennebulaEnv, err := FetchAzureEnvironment(context.TODO(), client, "opennebula")
		if err != nil {
			return nil, err
		}

		setupResult = append(setupResult, models.BenchmarkEnvironment{
			Name:        "OpenNebula",
			Environment: opennebulaEnv,
		})
	}

	if createKubevirt {
		pretty_log.TaskGroup("Creating KubeVirt environment")
		kubevirtEnv, err := SetupAzureEnvironment(context.TODO(), client, "kubevirt", app.Config.KubeVirt.Workers)
		if err != nil {
			return nil, err
		}

		pretty_log.TaskGroup("Setting up KubeVirt control node")

		// Commands to set up KubeVirt with K3s on control node
		controlCommandGroups := [][]string{
			{"sudo apt-get update"},
			{"sudo apt-get install jq nfs-common -y"},
			{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.7+k3s1 INSTALL_K3S_EXEC=\"server --tls-san " + kubevirtEnv.ControlNode.InternalIP + " --advertise-address " + kubevirtEnv.ControlNode.InternalIP + " --write-kubeconfig-mode=644 --disable=traefik --disable=servicelb\" sh -"},
		}

		for idx, cmdGroup := range controlCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

			outputList, err := utils.SshCommand(kubevirtEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return nil, err
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
		outputList, err := utils.SshCommand(kubevirtEnv.ControlNode.PublicIP, []string{"sudo cat /var/lib/rancher/k3s/server/node-token"})
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		token := strings.TrimSuffix(outputList[0], "\n")
		pretty_log.CompleteTask()

		pretty_log.TaskGroup("Setting up KubeVirt worker nodes")

		// Commands to set up KubeVirt with K3s on worker nodes
		workerCommandGroups := [][]string{
			{"sudo apt-get update"},
			{"sudo apt-get install jq nfs-common -y"},
			{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.7+k3s1 K3S_URL=https://" + kubevirtEnv.ControlNode.InternalIP + ":6443 K3S_TOKEN=" + token + " sh -"},
		}

		if app.Config.KubeVirt.Workers > 0 {
			for idx, cmdGroup := range workerCommandGroups {
				pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(workerCommandGroups), strings.Join(cmdGroup, " && "))

				for _, worker := range kubevirtEnv.WorkerNodes {
					outputList, err = utils.SshCommand(worker.PublicIP, cmdGroup)
					if err != nil {
						pretty_log.FailTask()
						return nil, err
					}

					pretty_log.CompleteTask()

					if outputList != nil {
						for _, output := range outputList {
							pretty_log.TaskResult(output)
						}
					}
				}
			}
		} else {
			pretty_log.TaskResult("No worker nodes to setup")
		}

		// Install KubeVirt on control node
		pretty_log.BeginTask("Installing KubeVirt on control node")
		controlCommandGroups = [][]string{
			{"kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/" + app.Config.KubeVirt.Version + "/kubevirt-operator.yaml > /dev/null"},
			{"kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/" + app.Config.KubeVirt.Version + "/kubevirt-cr.yaml > /dev/null"},
			{"kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/" + app.Config.KubeVirt.CDI.Version + "/cdi-operator.yaml > /dev/null"},
			{"kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/" + app.Config.KubeVirt.CDI.Version + "/cdi-cr.yaml > /dev/null"},
		}

		for idx, cmdGroup := range controlCommandGroups {
			pretty_log.BeginTask("- Command (%d/%d): %s", idx+1, len(controlCommandGroups), strings.Join(cmdGroup, " && "))

			outputList, err = utils.SshCommand(kubevirtEnv.ControlNode.PublicIP, cmdGroup)
			if err != nil {
				pretty_log.FailTask()
				return nil, err
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
			res, _ := utils.SshCommand(kubevirtEnv.ControlNode.PublicIP, []string{"kubectl get kubevirt.kubevirt.io/kubevirt -n kubevirt -o=jsonpath=\"{.status.phase}\""})
			if res != nil && res[0] == "Deployed" {
				break
			}

			time.Sleep(1 * time.Second)
		}
		pretty_log.CompleteTask()

		setupResult = append(setupResult, models.BenchmarkEnvironment{
			Name:        "KubeVirt",
			Environment: kubevirtEnv,
		})
	} else {
		pretty_log.TaskResult("Fetching KubeVirt environment")
		kubevirtEnv, err := FetchAzureEnvironment(context.TODO(), client, "kubevirt")
		if err != nil {
			return nil, err
		}

		setupResult = append(setupResult, models.BenchmarkEnvironment{
			Name:        "KubeVirt",
			Environment: kubevirtEnv,
		})
	}

	return setupResult, nil
}

// Shutdown cleans up the base environment in Azure and deletes all the resources
func Shutdown(deleteOpennebula, deleteKubevirt bool) error {
	client, err := azure.New(&azure.Opts{
		AuthLocation:   app.Config.Azure.AuthLocation,
		SubscriptionID: app.Config.Azure.SubscriptionID,
	})

	if err != nil {
		return err
	}

	if deleteOpennebula {
		pretty_log.TaskGroup("Shutting down OpenNebula environment")
		err = deleteEnvironment(context.TODO(), client, "opennebula")
		if err != nil {
			pretty_log.FailTask()
			return err
		}
		pretty_log.CompleteTask()
	}

	if deleteKubevirt {
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

func FetchAzureEnvironment(ctx context.Context, client *azure.Client, namePrefix string) (*models.AzureEnvironment, error) {
	rg := app.Config.Azure.ResourceGroupBaseName + "-" + namePrefix

	prefixed := func(name string) string {
		return namePrefix + "-" + name
	}

	pretty_log.BeginTask("Fetching resource group")
	controlNIC, err := client.GetNIC(ctx, namePrefix+"-nic-1", rg)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	pretty_log.BeginTask("Fetching public IP")
	controlPublicIP, err := client.GetPublicIP(ctx, prefixed("ip"), rg)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	pretty_log.BeginTask("Fetching control node VM")
	controlVM, err := client.GetVM(ctx, prefixed("control-1"), rg)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	workerNodes := make([]models.WorkerNode, 0)

	for i := 0; i < app.Config.OpenNebula.Workers; i++ {
		pretty_log.BeginTask("Fetching worker node %d public IP", i+1)
		workerPublicIP, err := client.GetPublicIP(ctx, prefixed("worker-ip-"+strconv.Itoa(i+1)), rg)
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()

		pretty_log.BeginTask("Fetching worker node %d NIC", i+1)
		workerNIC, err := client.GetNIC(ctx, "worker-nic-"+strconv.Itoa(i+1), rg)
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()

		pretty_log.BeginTask("Fetching worker node %d VM", i+1)
		workerVM, err := client.GetVM(ctx, prefixed("worker-"+strconv.Itoa(i+1)), rg)
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()

		workerNodes = append(workerNodes, models.WorkerNode{
			VM:         *workerVM,
			InternalIP: *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *workerPublicIP.Properties.IPAddress,
		})
	}

	return &models.AzureEnvironment{
		ResourceGroup: rg,
		ControlNode: models.ControlNode{
			VM:         *controlVM,
			InternalIP: *controlNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *controlPublicIP.Properties.IPAddress,
		},
		WorkerNodes: workerNodes,
	}, nil
}

func SetupAzureEnvironment(ctx context.Context, client *azure.Client, namePrefix string, workers int) (*models.AzureEnvironment, error) {
	pretty_log.BeginTask("Creating resource group")
	resourceGroup, err := client.CreateResourceGroup(ctx, app.Config.Azure.ResourceGroupBaseName+"-"+namePrefix)
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
	_, err = client.CreateVM(ctx, prefixed("control-1"), rg, *controlNIC.ID, prefixed("control-1"), app.Config.Azure.Username, app.Config.Azure.Password, app.Config.Azure.PublicKeys)
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	// Wait for VM to boot by sleeping
	pretty_log.BeginTask("Waiting for VM to boot (max 30 seconds)")
	for i := 0; i < 30; i++ {
		res, _ := utils.SshCommand(*controlPublicIP.Properties.IPAddress, []string{"echo \"\""})
		if res != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}
	pretty_log.CompleteTask()

	// Generate SSH key pair for control node non-interactively, 2048 bits, no passphrase, RSA. Don't overwrite
	pretty_log.BeginTask("Generating SSH key pair")
	_, _ = utils.SshCommand(*controlPublicIP.Properties.IPAddress, []string{"ssh-keygen -t rsa -b 2048 -N \"\" -f /home/" + app.Config.Azure.Username + "/.ssh/id_rsa <<< n 1> /dev/null 2> /dev/null"})
	// TODO: Fix me, always returns an error
	//if err != nil {
	//	pretty_log.FailTask()
	//	return nil, err
	//}
	publicKey, err := utils.SshCommand(*controlPublicIP.Properties.IPAddress, []string{"cat /home/" + app.Config.Azure.Username + "/.ssh/id_rsa.pub"})
	if err != nil {
		pretty_log.FailTask()
		return nil, err
	}
	pretty_log.CompleteTask()

	workerNodes := make([]models.WorkerNode, workers)

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

		vm, err := client.CreateVM(ctx, prefixed("worker-"+strconv.Itoa(i+1)), rg, *workerNIC.ID, prefixed("worker-"+strconv.Itoa(i+1)), app.Config.Azure.Username, app.Config.Azure.Password,
			append(app.Config.Azure.PublicKeys, publicKey[0]))
		if err != nil {
			pretty_log.FailTask()
			return nil, err
		}
		pretty_log.CompleteTask()

		workerNodes[i] = models.WorkerNode{
			VM:         *vm,
			InternalIP: *workerNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *workerPublicIP.Properties.IPAddress,
		}
	}

	return &models.AzureEnvironment{
		ResourceGroup: rg,
		ControlNode: models.ControlNode{
			InternalIP: *controlNIC.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
			PublicIP:   *controlPublicIP.Properties.IPAddress,
		},
		WorkerNodes: workerNodes,
	}, nil
}

func deleteEnvironment(ctx context.Context, client *azure.Client, namePrefix string) error {
	rg := app.Config.Azure.ResourceGroupBaseName + "-" + namePrefix

	pretty_log.BeginTask("Deleting resource group (including all resources)")
	err := client.DeleteResourceGroup(ctx, rg)
	if err != nil {
		return err
	}
	pretty_log.CompleteTask()

	return nil
}
