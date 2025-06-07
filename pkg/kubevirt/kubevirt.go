package kubevirt

import (
	"fmt"
	"strings"

	"vmx2vmi/pkg/vmx" // Assuming vmx package is in this path

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// Ptr returns a pointer to the given value.
// Useful for struct fields that are pointers to primitive types.
func Ptr[T any](v T) *T {
	return &v
}

func CreateKubeVirtVM(vmxConfig *vmx.VMXConfig, pvcName string, vmNameOverride string, namespace string, startVM bool) (*kubevirtv1.VirtualMachine, error) {
	vmName := vmNameOverride
	if vmName == "" {
		vmName = vmxConfig.DisplayName
	}

	// Basic sanitization for Kubernetes resource name
	vmName = strings.ToLower(vmName)
	vmName = strings.ReplaceAll(vmName, " ", "-")
	vmName = strings.ReplaceAll(vmName, "_", "-")
	// A more robust sanitization regex might be: reg := regexp.MustCompile("[^a-z0-9-]+")
	// vmName = reg.ReplaceAllString(vmName, "")
	if len(vmName) > 63 { // K8s names often have length limits
		vmName = vmName[:63]
	}
	if vmName == "" { // if displayname was e.g. "  "
		return nil, fmt.Errorf("derived VM name is empty. Please provide a valid name via -name flag or ensure VMX displayName is suitable")
	}

	memoryQuantityStr := fmt.Sprintf("%dMi", vmxConfig.MemoryMiB)
	memoryQuantity, err := resource.ParseQuantity(memoryQuantityStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory quantity '%s': %w", memoryQuantityStr, err)
	}

	vm := &kubevirtv1.VirtualMachine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kubevirtv1.SchemeGroupVersion.String(),
			Kind:       "VirtualMachine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: namespace,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: Ptr(startVM),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubevirtv1.AppLabel: vmName, // Common KubeVirt label
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						CPU: &kubevirtv1.CPU{
							Cores: vmxConfig.NumVCPUs,
						},
						Memory: &kubevirtv1.Memory{
							Guest: &memoryQuantity,
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name:      "disk0", // Name for the disk device
									BootOrder: Ptr(uint(1)),
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio", // Defaulting to virtio. Could be sata, scsi.
										},
									},
								},
							},
							Interfaces: []kubevirtv1.Interface{
								{
									Name: "default",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Masquerade: &kubevirtv1.InterfaceMasquerade{}, // Simple default networking
									},
								},
							},
							Rng: &kubevirtv1.Rng{}, // Recommended for guest OS entropy
						},
					},
					Networks: []kubevirtv1.Network{
						{
							Name: "default", // Must match an interface name
							NetworkSource: kubevirtv1.NetworkSource{
								Pod: &kubevirtv1.PodNetwork{}, // Use pod network
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "disk0", // Must match a disk name in devices.disks
							VolumeSource: kubevirtv1.VolumeSource{
								PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvcName, // The PVC containing the VMDK data
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return vm, nil
}
