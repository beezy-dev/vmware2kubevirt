package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"vmx2vmi/pkg/kubevirt"
	"vmx2vmi/pkg/vmdk"
	"vmx2vmi/pkg/vmx"

	"sigs.k8s.io/yaml"
)

func main() {
	vmxPath := flag.String("vmx", "", "Path to the VMX file (for VM conversion)")
	pvcName := flag.String("pvc", "", "Name of the PVC for the primary VMDK (for VM conversion)")
	outputVMName := flag.String("name", "", "Name for the KubeVirt VirtualMachine resource (defaults to VMX displayName)")
	namespace := flag.String("namespace", "default", "Namespace for the KubeVirt VirtualMachine")
	runVM := flag.Bool("run", false, "Set the VM to run immediately (spec.running=true)")
	vmdkInfoPath := flag.String("vmdk-info", "", "Path to a VMDK file to extract and display its descriptor")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "To display VMDK descriptor info (this action is exclusive):\n")
		fmt.Fprintf(os.Stderr, "  %s -vmdk-info <path-to-vmdk>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "To convert VMX to KubeVirt VirtualMachine YAML:\n")
		fmt.Fprintf(os.Stderr, "  %s -vmx <path-to-vmx> -pvc <pvc-name> [other-options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options for VM conversion and general use:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Handle VMDK info extraction if the -vmdk-info flag is provided. This action takes precedence.
	if *vmdkInfoPath != "" {
		// If -vmdk-info is specified, it's the primary action.
		// Warn if other potentially conflicting/irrelevant flags for other actions are present.
		if *vmxPath != "" || *pvcName != "" || *outputVMName != "" || *namespace != "default" || *runVM {
			log.Println("Warning: Other flags (-vmx, -pvc, -name, -namespace, -run) are ignored when -vmdk-info is specified.")
		}

		descriptor, isVMDK, err := vmdk.ExtractVMDKDescriptor(*vmdkInfoPath)
		if err != nil {
			if isVMDK {
				log.Fatalf("Error extracting descriptor from VMDK file '%s': %v\n", *vmdkInfoPath, err)
			} else {
				log.Fatalf("File '%s' is not a recognized VMDK or error occurred: %v\n", *vmdkInfoPath, err)
			}
		}
		fmt.Printf("--- VMDK Descriptor for: %s ---\n%s\n--- End Descriptor ---\n", *vmdkInfoPath, descriptor)
		return
	}

	// Handle VMX to KubeVirt VM conversion.
	// Both -vmx and -pvc must be provided for this action.
	if *vmxPath != "" && *pvcName != "" {
		vmxConfig, err := vmx.ParseVMX(*vmxPath)
		if err != nil {
			log.Fatalf("Error parsing VMX file: %v", err)
		}

		kvVM, err := kubevirt.CreateKubeVirtVM(vmxConfig, *pvcName, *outputVMName, *namespace, *runVM)
		if err != nil {
			log.Fatalf("Error creating KubeVirt VM object: %v", err)
		}

		yamlData, err := yaml.Marshal(kvVM)
		if err != nil {
			log.Fatalf("Error marshalling KubeVirt VM to YAML: %v", err)
		}

		// Determine output path
		vmxDir := filepath.Dir(*vmxPath)
		outputYAMLFileName := kvVM.Name + ".yaml"
		outputYAMLPath := filepath.Join(vmxDir, outputYAMLFileName)

		log.Printf("Writing KubeVirt VirtualMachine YAML to: %s\n", outputYAMLPath)
		err = os.WriteFile(outputYAMLPath, yamlData, 0644)
		if err != nil {
			log.Fatalf("Error writing KubeVirt VM YAML to file %s: %v", outputYAMLPath, err)
		}
		return
	}

	// If neither primary action was fully specified, provide specific error messages.
	if *vmxPath != "" && *pvcName == "" {
		log.Println("Error: -pvc flag is required with -vmx for VM conversion.")
		flag.Usage()
		os.Exit(1)
	}
	if *vmxPath == "" && *pvcName != "" {
		log.Println("Error: -vmx flag is required with -pvc for VM conversion.")
		flag.Usage()
		os.Exit(1)
	}
	// Handle cases where optional flags are provided without the necessary primary flags for conversion.
	if (*outputVMName != "" || *namespace != "default" || *runVM) && (*vmxPath == "" || *pvcName == "") && *vmdkInfoPath == "" {
		log.Println("Error: Optional flags like -name, -namespace, -run require both -vmx and -pvc for VM conversion.")
		flag.Usage()
		os.Exit(1)
	}

	// Default case: No action specified or insufficient flags for any action.
	log.Println("Error: Please specify an action by providing appropriate flags. Use -h or --help for usage.")
	flag.Usage()
	os.Exit(1)
}
