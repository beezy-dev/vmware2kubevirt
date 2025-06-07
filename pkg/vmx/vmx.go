package vmx

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// VMXConfig holds extracted VMX data
type VMXConfig struct {
	DisplayName string
	NumVCPUs    uint32
	MemoryMiB   int64 // VMX memsize is typically in MB
}

func ParseVMX(vmxPath string) (*VMXConfig, error) {
	content, err := os.ReadFile(vmxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read VMX file %s: %w", vmxPath, err)
	}

	config := &VMXConfig{
		NumVCPUs:  1,    // Default VCPUs
		MemoryMiB: 1024, // Default Memory (1GiB)
	}
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")

		switch strings.ToLower(key) {
		case "displayname":
			config.DisplayName = value
		case "numvcpus":
			if cpus, errConv := strconv.ParseUint(value, 10, 32); errConv == nil {
				config.NumVCPUs = uint32(cpus)
			} else {
				log.Printf("Warning: could not parse numvcpus value '%s': %v", value, errConv)
			}
		case "memsize":
			if mem, errConv := strconv.ParseInt(value, 10, 64); errConv == nil {
				config.MemoryMiB = mem
			} else {
				log.Printf("Warning: could not parse memsize value '%s': %v", value, errConv)
			}
		}
	}

	if config.DisplayName == "" {
		baseName := filepath.Base(vmxPath)
		config.DisplayName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		log.Printf("Warning: 'displayName' not found in VMX, using filename '%s' as fallback.", config.DisplayName)
	}

	return config, nil
}
