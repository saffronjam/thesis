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

- kubectl: The CLI tool for Kubernetes. You can download it [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
- virtctl: The CLI tool for KubeVirt. You can download it [here](https://kubevirt.io/user-guide/operations/virtctl_client_tool).
- kubeconfig file: The configuration file for accessing the Kubernetes cluster. You should have received this alongside this document.

## Basic usage (Task 1/3)

The first task is about creating a VM using KubeVirt. You will need to create a VM and then access it using SSH or VNC. You can use a containerDisk for simplicity.

### Manifests

Below is a minimal manifest for creating the VM using a containerDisk. The credentials for the Cirros image are user **cirros** and password **gocubsgo**.

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
      resources:
        requests:
        memory: 128M
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo
```

### Steps
1. Create a VM using the manifest above.
```bash
kubectl apply -f my-vm.yaml
```

2. Access the VM using SSH or VNC (credentials vary depending on the image since no CloudInit is used).
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

## Maintenance (Task 2/3)
The second task is related to maintenance, where you will create a snapshot of a VM and then restore it. You will also perform a live migration on a VM to move it from one node to another without shutting it down. For this task, you can use a VM with a containerDisk since the task is not related to persistent storage.

Both snapshots and live migrations are treated as a resource by KubeVirt, meaning that the operation will take place in the background after a VirtualMachineSnapshot resource or a LiveMigration resource is created. See the References section for more information.

A live migration resource in KubeVirt does not move resources itself and instead uses the Kubernetes scheduler to decide where to place the VM. This means that a LiveMigration resource only tells the scheduler that the VM can be moved but not where to move it. The simplest way to tell the scheduler to move a VM to a specific host is to edit `spec.template.spec.nodeSelector` in the VM manifest. See the References section for more information.

### Manifests
The following is a minimal manifest for creating a VM that runs on a node with label `name: worker-1`. The credentials for the Cirros image are user **cirros** and password **gocubsgo**.
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
        name: worker1
      domain:
      cpu:
        cores: 1
      devices:
        disks:
        - name: containerdisk
          disk:
            bus: virtio
      resources:
        requests:
        memory: 128M
      volumes:
      - name: containerdisk
        containerDisk:
          image: quay.io/kubevirt/cirros-container-disk-demo
```

The following is a minimal manifest for creating a VirtualMachineSnapshot resource.
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

The following is a minimal manifest for creating a LiveMigration resource.
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  name: my-vm-migration
spec:
  vmiName: my-vm
  targetNode: worker2
```

The following is a minimal manifest for restoring a VM from a snapshot.
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
kubectl apply -f my-vm.yaml
```
    
2. Create a snapshot of the VM
```bash
kubectl apply -f my-vm-snapshot.yaml
```

3. Wait for the snapshot to be ready:
```bash
kubectl wait vmsnapshot my-vm-snapshot --for condition=Ready
```

4. Restore the VM from the snapshot
```bash
kubectl apply -f my-vm-restore.yaml
```

5. Use the following kubectl command to wait for the restore to be ready:
```bash
kubectl wait vmrestore my-vm-restore --for condition=Ready
```

### Live Migration Steps
1. Create a VM and wait for it to start
```bash
kubectl apply -f my-vm.yaml
```

2. Edit the label under `spec.template.spec.nodeSelector` in the VM manifest to `name: worker-2`:
```bash
kubectl edit vm my-vm
```

3. Create a LiveMigration resource or use virtctl:
```bash
kubectl apply -f my-vm-migration.yaml
```
```bash
virtctl migrate my-vm
```

4. Use the following kubectl command to wait for the migration to be ready:
```bash
kubectl wait vmimigration my-vm-migration --for condition=MigrationSucceeded
```

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to add comparisons with other VM management systems you have used.

1. How straightforward was it to create a VM and access it using SSH or VNC?
2. How intuitive was the process of stopping, starting, and restarting the VM?
3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

## Debugging (Task 3/3)
The third task is about debugging a VM that is not accessible through SSH. You will be presented with a VM manifest which, if deployed, will result in a VM that is running but cannot be accessed through SSH. Since the VM is running, you can access it through VNC using virtctl. You can use a containerDisk since the task is not related to persistent storage.

### Manifests
The following is a manifest for creating a Ubuntu VM that is not accessible through SSH. Credentials for the Ubuntu image should be user **cloud** and password **cloud**.

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
            passwd: $6$rounds=4096$abc
            shell: /bin/bash
            lock-passwd: false
            ssh_pwauth: True
            chpasswd: { expire: False }
            sudo: ALL=(ALL) NOPASSWD:ALL
            ssh_authorized_keys:
            - ssh-ed25519 abc
```

### Steps
1. Create a VM
```bash
kubectl apply -f my-vm.yaml
```
2. Access the VM through VNC
```bash
virtctl vnc my-vm
```
3. Debug the issue to fix the misconfiguration
4. Edit the VM manifest to fix the issue
```bash
kubectl edit vm my-vm
```
5. Ensure the VM is accessible through SSH
```bash
virtctl ssh cirros@my-vm
```
6. Delete the VM
```bash
kubectl delete vm my-vm
```

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to add comparisons with other VM management systems you have used.

1. How easy was it to debug and find the issue?
2. How easy was it to fix the issue?
3. Does the debug process differ from debugging a traditional VM management system?
4. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

## Summary
After you have completed the tasks, reflect on the overall experience of using KubeVirt. You may use the following questions as a guide, but feel free to discuss and compare them with other VM management systems you have used.

1. How would you compare the experience of using KubeVirt with other VM management systems you have used?
2. How intuitive is the workflow of KubeVirt? Can you use it without reading the documentation?
3. How would you describe the overall experience of using KubeVirt?

Thank you for participating in the usability analysis!