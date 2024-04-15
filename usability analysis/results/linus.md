# Emils assignment

## Task 1

There were no trivial way to get the SSH fingerprint of the VM.

However since VNC was so easy i could easily get it via VNC — nope I couldn't because cirros doesn't have ssh-keygen per default

Could this be done programmatically somehow?

Could I provide some sort of init-script that would sign the ssh-server's key with a SSH CA?

    --local-ssh                    --local-ssh=true: Set this to true to use the SSH/SCP client available on your system by using this command as ProxyCommand; If set to false, this will establish a SSH/SCP connection with limited capabilities provided by this client

--local-ssh seems really cool, but I'd love to be able to add that to my ssh-config so I don't have to use the virtctl command every time.

I'd love the usage to be something like `ssh my-vm.virt.local` (.local might not be optimal but you get the gist)

Later I also found `./virtctl console`, which to me is much more useful than VNC. Just as VNC that works udring early boot as well, but allows easy scrolling

### Questions

#### 1. How intuitive was the process of creating and deleting a VM?

The act of doing `kubectl apply -f` and `kubectl delete` is quite neat, but the VM-definition is a not very.

It would be quite difficult for me to figure out I need to use `bus: virtio` without having someone else tell me I need to.

The `./virtctl create vm` interface on the other hand seems to be much easier to use. As referenced from the official documentation

At this point I decided I wanted to try to launch a debian VM.

Using `./virtctl create vm` however it was not clear which containerdisks are generally available or how I can create one (`./virtctl create vm --help` and the help for `--volume-containerdisk`). What I'd like is an explanation of what that flag actually means

What I'm searching for now is how to create a debian VM, looking at https://quay.io/organization/kubevirt where the exaple provided was I could not find anything called `debian`

Just trying `./virtctl-v1.2.0-linux-amd64 create vm | kubectl apply -f -` did not provide a working VM either, so I can't just modify that to work as I want

[The documentation](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk) referred to quay.io/containerdisks, which did have some more useful disks but not debian (only ubuntu, fedora and centos)

> containerDisks are not a good solution for any workload that requires persistent root disks across VM restarts.

So containerDisks are probably not what I want for my bog-standard VM.

At this point I thought I would just try to look at the standard kubevirt documentation in the hopes that they mention how you run a VM that isn't their demo one
What I found were two guides:
- https://kubevirt.io/labs/kubernetes/lab1.html
    - Did not mention anything on how to run my own VM
- https://kubevirt.io/labs/kubernetes/lab2.html
    - This might be what I want but it is not really clear what a CDI is nor what I should use it for, even after clicking through to https://github.com/kubevirt/containerized-data-importer
    - The first step of the guide is to install an operator, which I don't think I want to do just to get started
    - As an aside it is a bit worrying that they have written this:
        > If the importer pod completes in error, you may need to retry it or specify a different URL to the fedora cloud image. To retry, first delete the importer pod and the DataVolume, and then recreate the DataVolume.
        - That means I can't rely on just creating the resource and have it do what I want. I need to monitor it with my own retry-logic in order to be sure it works
    - This guide specifies how I can provide my own SSH-keys via cloud-init, which is nice
    - Trying to follow that guide gave me this error when I ran `kubectl describe datavolumes.cdi.kubevirt.io`
        > Normal   ErrClaimNotValid  48s                datavolume-import-controller  PVC spec is missing accessMode and no storageClass to choose profile
        - While this doesn't seem like an unsurmountable problem I decided to stop here.

2. How straightforward was it to access the VM using SSH and VNC?

Quite, I would have loved to easily be able to integrate the ssh proxycommand into my own ssh config

Also, I find `./virtctl console` nicer than vnc

3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

I could not figure out how to create a debian image. The documentation was very lacking as it seems geared towards how to operate a KubeVirt cluster, not how to use one.

From the documentation it also seems that KubeVirt is plauged by the same issues a lot of the kubernetes ecosystem is, a lot of strange abstractions that aren't well explained (CDI, datavolume, containerdisks etc etc)

## Task 2

I will try to modify the VM I already have running with the nodeSelector

Applying the migration did not seem to move my instance. I used the following nodeSelector:

    nodeSelector:
      name: worker-1

`kubectl get virtualmachineinstancemigration` shows phase=succeeded, even though it clearly did not move:

    $ kubectl get vmi
    NAME    AGE   PHASE     IP            NODENAME             READY
    my-vm   32m   Running   10.42.55.33   usability-worker-2   True

If I labeled the nodes beforehand the migration succeeded. However I had to delete the migration first.

I found this unintuitive as if it couldn't migrate (nothing matching the nodeSelector) I would have expected the migration to fail, not succeed

Once again this is also something that is a bit strange, that I have to remove the migration resource and recreate it in order for something to happen. That means I can't just create a resource and trust kubevirt will manage it but will need to have logic around the migration

### Questions

#### 1. How intuitive was the process of migrating a VM from one node to another?

Not very, it was unintuitive that the migration succeeded even though nothing was migrated, even though it should have been (I used a nodeSelector that didn't exist, so it should not have been running on any node)

I also found it unintuitive that I needed to create the migration resource after everything is correctly labeled. I would have expected that just updating the nodeSelector would move the vm.

#### 2. How straightforward was it to create and restore a snapshot of a VM?

N/A

#### 3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

The documentation mentions that I should look at the status of the VMI, which I did not think to do (regarding the intuitive question above)

Had I read the documentation I would at least have know that the migration decided to do nothing when I used a non-existent nodeSelector

Once again [the documentation](https://kubevirt.io/user-guide/operations/live_migration/) deals a lot with how to operate the migration infrastructure and not anything on how to do the migration itself. Granted it is listed under the `Operations` section of the documentation but I found nothing when searching under the `Virtual Machines` section where I would have expected it to be.

## Debugging (Task 3/3)

```
$ ./virtctl-v1.2.0-linux-amd64 ssh cloud@broken
ssh: handshake failed: knownhosts: key mismatch
```

`./virtctl ssh --local-ssh cloud@broken` failed:

    unix_listener: cannot bind to path /tmp/ssh_mux_vmi/broken.default_22_cloud.1EnfpxMEdMOKTOSJ: No such file or directory

I don't know what special things they do, but it seems like muxing is broken for some reason

Ah, the issue is that the host (%h) contains a `/` which breaks with my `ControlPath` settings

This solved the issue:

    ./virtctl-v1.2.0-linux-amd64 ssh --local-ssh --local-ssh-opts '-o ControlMaster no' cloud@broken

The `key mismatch` error is however more difficult to debug because there is no good debug options for `virtctl ssh`

### Questions

1. How easy was it to debug and find the issue?

Not very, I did not manage to solve all issues

The passwd issue was easy as not being able to log in was very obvious with `virtctl console`

The ssh issue I solved by accident, the `knownhosts: key mismatch` made me look into the .ssh/authorized_keys file and updated that with `virtctl console`. It didn't solve the key mismatch issue but it did allow me to continue with local-ssh

I did not manage to solve the `key mismatch` issue as there is very little in the way of debugging `virtctl ssh`

2. How easy was it to fix the issue?

Two of the issues was easy to fix, I appreciated that `virtctl restart` worked as I did not think cloud-init would update the configs

3. Does the debug process differ from debugging a traditional VM management system?

I'm not sure what a traditional vm management system refers to. It is quite similar to the opennebula command lines. I appreciated that `virtctl console` was so easy to use.

4. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?

I could not (easily) find documentation about how `virtctl ssh` worked so I could not figure out one of the issues.
In fact there seems to be no documentaiton for `virtctl`. Is it the same virtctl as the one from openshift?

The fact that `kubectl ssh` broke without even a verbose option is worrying

## Summary

1. How would you compare the experience of using KubeVirt with other VM management systems you have used?

I like the `virtctl` tool (when it worked) and like the idea of declaring my VMs

However the system is very complicated with a lot of parts that it isn't clera how they fit together.

The documentation is also very lacking

The fact that you manage the system with two different tools, both `virtctl` and `kubectl` is unnescesary and confusing.

I find it telling that  I didn't even manage to run a debian vm

2. How intuitive is the workflow of KubeVirt? Can you use it without reading the documentation?

Not at all. I would not have been able to use it without the documentation. I could barely use it with the documentation

3. How would you describe the overall experience of using KubeVirt?

The idea is good but the implementation is severly lacking. I could see the potential and it seemed very promising but from this experience KubeVirt did not deliver

One could probably build something sensible with it but I feel the barrier of entry is very high.