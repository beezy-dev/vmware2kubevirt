# Convert VMware Virtual Machine to Kubervirt Virtual Machine

> ![NOTE]
> This repository is only for learning purposes from a NetApp/ONTAP perspective only. 
 
The VMware datastore provides a one-to-many, or a single endpoint (SAN or NAS) hosting multiple virtual machines along their configuration files.   
In contrast, the construct of datastore in Kubevirt does not exists, instead each virtual machine will have one or multiple PVCs to host the virtual machine disk(s) using a raw disk image format while the configuration is stored in the etcd like any other kubernetes primitives. 

## Migration workflow 
Considering the above, converting ten virtual machines from a VMware datastore to 10 Kubevirt VirtualMachines will require the following steps:

1. pre-provision 10 ONTAP volumes through Trident as PVCs with the appropriate size (OpenShift Virtualization is adding an extra 5.5% for overhead)
1. perform a storage vmotion of each VMware virtual machines to its dedicated ONTAP volumes
1. offline the 10 virtual machines
1. snapshots each ONTAP volumes through Trident
1. import each snapshots as a new PVCs in their respective namespace if needed
1. convert the vmdks to with qemu-img 
1. use vmx2vmi to convert the VMware virtual machine configurations to their respective Kubevirt VirtualMachine manifest
1. leverage the cloud-init to replace the VMware tools with the Kubevirt guest tools
1. boot the virtual machines and verify their status 
1. perform a clean up of the VMware virtual machines PVCs after 7 days
1. perform a clean up of the original VMware datastore after 30 days


# Example

Run the CLI command without any option or with -h/--help

```
$ go run main.go
2025/06/07 15:13:52 Error: Please specify an action by providing appropriate flags. Use -h or --help for usage.
Usage of /home/romdalf/.cache/go-build/78/786181692e5695985180c09aa918560055de7ff14094f4a48cc58a4897bede7b-d/main:

To display VMDK descriptor info (this action is exclusive):
  /home/romdalf/.cache/go-build/78/786181692e5695985180c09aa918560055de7ff14094f4a48cc58a4897bede7b-d/main -vmdk-info <path-to-vmdk>

To convert VMX to KubeVirt VirtualMachine YAML:
  /home/romdalf/.cache/go-build/78/786181692e5695985180c09aa918560055de7ff14094f4a48cc58a4897bede7b-d/main -vmx <path-to-vmx> -pvc <pvc-name> [other-options]

Options for VM conversion and general use:
  -name string
        Name for the KubeVirt VirtualMachine resource (defaults to VMX displayName)
  -namespace string
        Namespace for the KubeVirt VirtualMachine (default "default")
  -pvc string
        Name of the PVC for the primary VMDK (for VM conversion)
  -run
        Set the VM to run immediately (spec.running=true)
  -vmdk-info string
        Path to a VMDK file to extract and display its descriptor
  -vmx string
        Path to the VMX file (for VM conversion)
```

## VMDK Descriptor

The disk component within VMWare is represented by one or multiple files depending of the creation format. 
If the format is:
- Single File (Monolithic): A single ```.vmdk``` file contains both the descriptor and the data.
- Multiple Files (Split): The descriptor is a separate ```.vmdk``` file, and the data is split into multiple ```*-flat.vmdk``` files (or ```*-s###.vmdk``` for sparse split files).   
This was historically used for larger disks to overcome filesystem limitations or for easier transfer.

### Single File (Monolithic)

Read the file descriptor from a monolithic VMDK file

```
$ go run main.go -vmdk-info vmware/monolithic/myvm_disk.vmdk 
```
Expected  output:

```
--- VMDK Descriptor for: vmware/monolithic/myvm_disk.vmdk ---
# Disk DescriptorFile
version=1
CID=4e0549ad
parentCID=ffffffff
createType="streamOptimized"

# Extent description
RW 20971520 SPARSE "myvm_disk.vmdk"

# The Disk Data Base
#DDB

ddb.virtualHWVersion = "4"
ddb.geometry.cylinders = "20805"
ddb.geometry.heads = "16"
ddb.geometry.sectors = "63"
ddb.adapterType = "ide"
ddb.toolsVersion = "2147483647"

--- End Descriptor ---
```

## VMX to VirtualMachine

Run the following command to create the KubeVirt VirtualMachine manifest from a VMware virtual machine vmx file: 

```
$ go run main.go -vmx vmware/monolithic/vmlin01.vmx -pvc vmlin01-boot -name vmlin01-convert-test -namespace vm2kv-poc
```

Expected output:
``` 
2025/06/07 15:14:01 Writing KubeVirt VirtualMachine YAML to: vmware/monolithic/vmlin01-convert-test.yaml
``` 

Considering our ```vmware``` folder containing examples, the content of ```vmlin01-convert-test.yaml``` would be:

```
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  creationTimestamp: null
  name: vmlin01-convert-test
  namespace: vm2kv-poc
spec:
  running: false
  template:
    metadata:
      creationTimestamp: null
      labels:
        kubevirt.io: vmlin01-convert-test
    spec:
      domain:
        cpu:
          cores: 4
        devices:
          disks:
          - bootOrder: 1
            disk:
              bus: virtio
            name: disk0
          interfaces:
          - masquerade: {}
            name: default
          rng: {}
        memory:
          guest: 8Gi
        resources: {}
      networks:
      - name: default
        pod: {}
      volumes:
      - name: disk0
        persistentVolumeClaim:
          claimName: vmlin01-boot
status: {}
```

