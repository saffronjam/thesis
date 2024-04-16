### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How intuitive was the process of creating and deleting a VM?
Very easy! I like that it's as simple as creating a yaml file and then applying it. Much simpler than any other way I've tried for managing VMs.
2. How straightforward was it to access the VM using SSH and VNC?
Very simple! Simply doing `virtctl ssh cirros@my-vm` and then typing in the password and then I was in!
3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?
Getting VNC to work on Mac was a bit tricky, but I downloaded Tiger VNC and saved the following script (while updating the Tiger VNC version) as /usr/local/bin/remote-viewer: https://github.com/kubevirt/kubevirt/issues/812#issuecomment-391832335

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How intuitive was the process of migrating a VM from one node to another?
Very simple! I once again really like that you just edit yaml files and apply labels to conveniently move VMs between worker nodes. Much simpler than our current way of doing it in OpenNebula...
2. How straightforward was it to create and restore a snapshot of a VM?
Did not try
3. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?
I had to delete the entry from `~/.ssh/kubevirt_known_hosts` after each time I restarted the VM because `virtctl ssh` complained about key mismatch.

### Questions
After you have finished the tasks, reflect on the experience. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How easy was it to debug and find the issue?
Easy! Using `virtctl vnc` I could change the password and then try logging in.
2. How easy was it to fix the issue?
Very easy, just generated a new password.
3. Does the debug process differ from debugging a traditional VM management system?
I've used IPMI which is similar to the vnc session in this case.
4. Describe, if any, the issues you encountered during the task. Were the issues related to KubeVirt, documentation, or other?
The most difficult thing was to get the VNC viewer working on Mac, but after that it was smooth sailing.

## Summary
After you have completed the tasks, reflect on the overall experience of using KubeVirt. You may use the following questions as a guide, but feel free to elaborate with your own thoughts.

1. How would you compare the experience of using KubeVirt with other VM management systems you have used?
It seems to be much easier to deploy and manage machines! I want to start using this in production for our build pipelines and internal clusters.
2. How intuitive is the workflow of KubeVirt? Can you use it without reading the documentation?
Well I had the example files you provided which was simple. But the CLI tools also gave good help.
3. How would you describe the overall experience of using KubeVirt?
Great! I want more!