# Kubernetes as a VM management system - Usability Analysis

This is a usability analysis of KubeVirt that involves performing a set of tasks and then reflecting on the experience, where the goal is to evaluate how well KubeVirt works in practice. By participating in the usability analysis you provide valuable feedback and insight that can be used to improve KubeVirt. Your answers will be included as-is in the appendix of the project report. Upon request, the answers can be anonymized.

## KubeVirt

The following is a brief introduction to KubeVirt, its most relevant features and how it compares to previous VM management systems. If you are already familiar with KubeVirt and its machinery, you can skip this section.

KubeVirt is an extension to Kubernetes that allows for the creation of virtual machines (VMs) and is designed to provide a similar experience to managing any other resource in Kubernetes, such as Pods or Deployments. This means that VMs are defined in YAML files and integrate with the command line tool `kubectl`. KubeVirt defines its resources as a *Custom Resource Definition* (CRD) in Kubernetes, where the most prominent resources are *VirtualMachine* and *VirtualMachineInstance*. The relationship between them is comparable to that of a Deployment and a Pod, where the Deployment defines a desired state and a Pod represents the running state. See References below for more information.

Certain VM operations are not possible using kubectl, such as accessing the VM using SSH or VNC. For this reason, KubeVirt provides a complementary CLI tool called `virtctl`. 

Being built on top of Kubernetes allows the VMs to integrate with other resources, including the Service and PersistentVolume resources. For instance, a VM can be exposed to the network using a LoadBalancer Service or manage storage dynamically using a persistent storage provisioner. However, since a VM requires a disk image (such as a QCOW2 image), it means that a PersistentVolume must point to a folder with a disk image in it. For this reason, manually creating a PersistentVolume for a VM can be tedious. Instead, KubeVirt primarily provides two alternatives for managing disks that are presented below. See the References section for more information.

- **containerDisk**: A disk that is created at every boot. It is not persistent and is similar to a container.
- **DataVolume**: A disk that is persistent and initialized when the VM is created. It uses PersistentVolumeClaim and a PersistentVolume under the hood where it injects a disk image.

## Prerequisites
* Install the following tools:
  * **kubectl**\
    The CLI tool for Kubernetes. You can download it [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
  * **virtctl**\
    The CLI tool for KubeVirt. You can download it [here](https://kubevirt.io/user-guide/operations/virtctl_client_tool).
  * **VNC viewer**\
    A tool for viewing the VNC connection, such as [RealVNC](https://www.realvnc.com/en/).
* **kubeconfig file**\
  The configuration file for accessing the Kubernetes cluster. You should have received this alongside this document. Make sure that the kubeconfig file is in the default location (`~/.kube/config`) or set the `KUBECONFIG` environment variable to the path of the kubeconfig file.
* **Your own namespace**\
  You should have received a unique namespace alongside this document. Set the namespace using 
  ```bash
  kubectl config set-context --current --namespace=<your namespace>
  ```
* **Monitor resources**\
    In a separate terminal window, run `watch -n 1 kubectl get vmis` to monitor the state of the VMIs, or any other resource you want to monitor.


## Basic usage (Task 1/3)

The first task is about creating a VM using KubeVirt. You will need to create a VM and then access it using SSH or VNC. You can use a containerDisk for simplicity.

### Manifests

Below is a minimal manifest for creating the VM using a containerDisk. The credentials for the Cirros image are user **cirros** and password **gocubsgo**.


**my-vm.yml**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
spec:
  running: true
  template:
    spec:
      domain:
        cpu:
          cores: 1
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
          interfaces:
          - name: default
            masquerade: {}
        resources:
          requests:
            memory: 128M
      networks:
      - name: default
        pod: {}
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo
```


### Steps
1. Create a VM using the manifest above.
```bash
kubectl apply -f my-vm.yml
```

2. Access the VM using SSH or VNC (credentials vary depending on the image since no CloudInit is used). You can use the *--local-ssh* flag if there is a problem with the SSH keys.  
```bash
virtctl ssh cirros@my-vm
virtctl vnc my-vm
```

3. Stop, start, and restart the VM.
```bash
virtctl stop my-vm
virtctl start my-vm
virtctl restart my-vm
```

4. Delete the VM.
```bash
kubectl delete vm my-vm
```

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How intuitive was the process of creating and deleting a VM?
2. How straightforward was it to access the VM using SSH and VNC?
3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

## Maintenance (Task 2/3)
The second task is related to maintenance, where you will create a snapshot of a VM and then restore it. You will also perform a live migration on a VM to move it from one node to another without shutting it down.

Both snapshots and live migrations are treated as a resource by KubeVirt, meaning that the operation will take place in the background after a VirtualMachineSnapshot resource or a VirtalMachineInstanceMigration resource is created. See the References section for more information.

A live migration resource in KubeVirt does not move resources itself and instead uses the Kubernetes scheduler to decide where to place the VM. This means that a LiveMigration resource only tells the scheduler that the VM can be moved but not where to move it. See the References section for more information. The Kubernetes scheduler is sophistiacted and support a wide range of scheduling strataegies, but for this task we can force a move between two nodes by using the `spec.template.spec.nodeSelector` in the VM and editing the labels of the worker nodes in the cluster. To not interfere with other participants, you should create a unique label for you.  

A snapshot in KubeVirt works similarly to a snapshot in other VM management systems, where it captures the state of the VM at a specific point in time. However, the snapshot uses Kubernetes VolumeSnapshot, which is a Kubernetes API for taking snapshots of PersistentVolumes. This means that a VM with a containerDisk *cannot* be snapshotted. Instead, you should use a VM with a DataVolume. See the References section for more information.

### Live Migration Manifests
The following is a minimal manifest for creating a VM that runs on a node with label `name: worker-1`. The credentials for the Cirros image are user **cirros** and password **gocubsgo**.

**my-vm.yml**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
spec:
  running: true
  template:
    spec:
      nodeSelector:
        # Edit this to use a label of your choice
        my-label: some-value
      domain:
        cpu:
          cores: 1
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
          interfaces:
          - name: default
            masquerade: {}
        resources:
          requests:
            memory: 128M
      networks:
      - name: default
        pod: {}
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo
```

The following is a minimal manifest for creating a live migration resource.

**my-vm-migration.yml**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  name: my-vm-migration
spec:
  vmiName: my-vm
```

### Live Migration Steps
1. Ensure worker node 1 has your label (Make sure to replace `my-label` and `some-value` with your own values):
```bash
kubectl label node aks-user-21932338-vmss000004 my-label=some-value
```

2. Create a VM and wait for it to start. The VM should start on **aks-user-21932338-vmss000004**:
```bash
kubectl apply -f my-vm.yml
```

3. (Optional) Check if the VM is running on worker node 1:
```bash
kubectl get vmis
```

4. Create a LiveMigration resource or use virtctl:
```bash
kubectl apply -f my-vm-migration.yml
```
```bash
virtctl migrate my-vm
```

5. Add the same label to worker node 2 and remove it from worker node 1 (Make sure to replace `my-label` and `some-value` with your own values):
```bash
kubectl label node aks-user-21932338-vmss000005 my-label=some-value
kubectl label node aks-user-21932338-vmss000004 my-label-
```

6. (Optional) Check if the VM is now running on worker node 2:
```bash
kubectl get vmis
```

7. Unlabel worker node 2
```bash
kubectl label node aks-user-21932338-vmss000005 my-label-
```

8. Delete the VM
```bash
kubectl delete vm my-vm
```

### Snapshot Manifests
The following is a minimal manifest for creating a VM with a DataVolume. The credentials for the Alpine image are user **root** without password.

**my-vm.yml**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
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
            memory: 1Gi
      networks:
      - pod: {}
        name: default
      volumes:
      - dataVolume:
          name: datavolume
        name: datavolume-disk
  dataVolumeTemplates:
  - metadata:
      name: datavolume
    spec:
      pvc:
        storageClassName: azurefile-csi-kubevirt
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
      source:
        registry:
          url: docker://quay.io/kubevirt/cirros-container-disk-demo
```

The following is a minimal manifest for creating a VirtualMachineSnapshot resource.

**my-vm-snapshot.yml**
```yaml
apiVersion: snapshot.kubevirt.io/v1alpha1
kind: VirtualMachineSnapshot
metadata:
  name: my-vm-snapshot
spec:
  source:
    apiGroup: kubevirt.io
    kind: VirtualMachine
    name: my-vm
```

The following is a minimal manifest for restoring a VM from a snapshot.

**my-vm-restore.yml**
```yaml
apiVersion: snapshot.kubevirt.io/v1alpha1
kind: VirtualMachineRestore
metadata:
  name: my-vm-restore
spec:
  target:
    apiGroup: kubevirt.io
    kind: VirtualMachine
    name: my-vm
  virtualMachineSnapshotName: my-vm-snapshot
```


### Snapshot Steps
1. Create a VM
```bash
kubectl apply -f my-vm.yml
```
    
2. Create a snapshot of the VM
```bash
kubectl apply -f my-vm-snapshot.yml
```

3. Wait for the snapshot to be ready:
```bash
kubectl get vmsnapshots
```

4. Stop the VM
```bash
virtctl stop my-vm
```

5. Restore the VM from the snapshot
```bash
kubectl apply -f my-vm-restore.yml
```

6. Wait for the VM to be restored
```bash
kubectl get vmrestore
```

7. Delete the resources
```bash
kubectl delete vmsnapshot my-vm-snapshot
kubectl delete vm my-vm
```

If you tried to actually see if the snapshot worked you will see that the VM is not restored. This is because only snapshot creation, and not snapshot restorations, are supported in Azure File CSI (the test environment used for this analysis). However, KubeVirt still snapshots VM specifications.

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How intuitive was the process of migrating a VM from one node to another?
2. How straightforward was it to create and restore a snapshot of a VM?
3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

## Debugging (Task 3/3)
The third task is about debugging a VM that is not accessible through SSH. You will be presented with a VM manifest which, if deployed, will result in a VM that is running but cannot be accessed through SSH. Since the VM is running, you can access it through VNC using virtctl. You can use a containerDisk since the task is not related to persistent storage.

### Manifests
The following is a manifest for creating a Ubuntu VM that is not accessible through SSH. Credentials for the Ubuntu image should be user **cloud** and password **cloud**.

**my-vm.yml**
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
          - name: cloudinit
            disk:
              bus: virtio
          rng: {}
        resources:
          requests:
            cpu: 2
            memory: 2Gi
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/containerdisks/ubuntu:22.04
      - name: cloudinit
        cloudInitNoCloud:
          userData: |-
            #cloud-config
            users:
            - name: cloud
              # Generated with `mkpasswd -m sha-512 --rounds 4096`
              passwd: $6$rounds=4096$abc
              shell: /bin/bash
              lock-passwd: false
              chpasswd: { expire: False }
              sudo: ALL=(ALL) NOPASSWD:ALL
              ssh_authorized_keys:
              - ssh-ed25519 abc
            ssh_pwauth: True
```

### Steps
1. Create a VM
```bash
kubectl apply -f my-vm.yml
```
2. Access the VM through VNC
```bash
virtctl vnc my-vm
```
3. Find out what the issue is

4. Edit the VM manifest to fix the issue

5. (Optional) Restart the VM (some changes require a restart)
```bash
virtctl restart my-vm
```

6. Ensure the VM is accessible through SSH. You can use the *--local-ssh* flag if there is a problem with the SSH keys.  
```bash
virtctl ssh cloud@my-vm 
```
7. Delete the VM
```bash
kubectl delete vm my-vm
```

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How easy was it to debug and find the issue?
2. How easy was it to fix the issue?
3. Does the debug process differ from debugging a traditional VM management system?
4. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

## Summary
After you have completed the tasks, reflect on the overall experience of using KubeVirt. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How would you compare the experience of using KubeVirt with other VM management systems you have used?
2. How intuitive is the workflow of KubeVirt? Can you use it without reading the documentation?
3. How would you describe the overall experience of using KubeVirt?

Thank you for participating in the usability analysis!

## References
1. **VirtualMachineInstance**: [https://kubevirt.io/user-guide/virtual_machines/virtual_machine_instances](https://kubevirt.io/user-guide/virtual_machines/virtual_machine_instances)
2. **Disks**: [https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes)
3. **Snapshots**: [https://kubevirt.io/user-guide/operations/snapshot_restore_api](https://kubevirt.io/user-guide/operations/snapshot_restore_api)
4. **Live Migration**: [https://kubevirt.io/user-guide/operations/live_migration](https://kubevirt.io/user-guide/operations/live_migration)