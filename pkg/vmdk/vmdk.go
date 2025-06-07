package vmdk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	// vmdkMagicKDMV is the magic number "KDMV" (0x564d444b) found at the beginning of some VMDK files.
	vmdkMagicKDMV = 0x564d444b
	// sectorSize is the standard sector size for VMDK calculations.
	sectorSize = 512
	// descriptorOffsetInHeaderPos is the byte offset of the 'descriptorOffset' field in the KDMV header.
	descriptorOffsetInHeaderPos = 28
	// descriptorSizeInHeaderPos is the byte offset of the 'descriptorSize' field in the KDMV header.
	descriptorSizeInHeaderPos = 36
	// minHeaderSizeForDescFields is the minimum number of bytes needed from the header to read up to descriptorSize.
	minHeaderSizeForDescFields = descriptorSizeInHeaderPos + 8 // descriptorSize field is 8 bytes
	// initialReadSize is a reasonable amount to read initially to check signatures and header fields.
	initialReadSize = 256
	// maxDescriptorSizeBytes is a sanity limit for the descriptor size to prevent excessive memory allocation.
	maxDescriptorSizeBytes = 16 * 1024 * 1024 // 16MB
)

var (
	// vmdkDescriptorFileSignature is the byte sequence indicating a descriptor-only VMDK file.
	vmdkDescriptorFileSignature = []byte("# Disk DescriptorFile")
)

// ExtractVMDKDescriptor attempts to read the descriptor text from a VMDK file.
// It returns the descriptor content, a boolean indicating if the file was identified
// as a supported VMDK type, and an error if one occurred.
//
// Supported VMDK types:
// 1. Descriptor-only files (starting with "# Disk DescriptorFile").
// 2. Monolithic KDMV-type files (e.g., sparse extents) with an embedded descriptor.
func ExtractVMDKDescriptor(filePath string) (descriptor string, isVMDK bool, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", false, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	initialBuffer := make([]byte, initialReadSize)
	n, readErr := file.ReadAt(initialBuffer, 0)
	// We can proceed if we read some bytes, even if EOF was hit,
	// as long as n is sufficient for signature checks.
	if readErr != nil && readErr != io.EOF {
		return "", false, fmt.Errorf("failed to read initial bytes from %s: %w", filePath, readErr)
	}
	if n == 0 && readErr == io.EOF { // Empty file
		return "", false, fmt.Errorf("file %s is empty", filePath)
	}
	actualInitialBytes := initialBuffer[:n]

	// 1. Check for descriptor-only file signature
	if bytes.HasPrefix(actualInitialBytes, vmdkDescriptorFileSignature) {
		// This is a descriptor-only file. Read the whole content.
		// Seek back to start and read everything.
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", true, fmt.Errorf("failed to seek to start of descriptor-only file %s: %w", filePath, err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			return "", true, fmt.Errorf("failed to read content of descriptor-only file %s: %w", filePath, err)
		}
		return string(content), true, nil
	}

	// 2. Check for KDMV magic number (monolithic file with embedded descriptor)
	if len(actualInitialBytes) < 4 { // Not enough bytes for magic number
		return "", false, fmt.Errorf("file %s is too small to be a KDMV VMDK (size: %d bytes)", filePath, len(actualInitialBytes))
	}

	magic := binary.LittleEndian.Uint32(actualInitialBytes[0:4])
	if magic == vmdkMagicKDMV {
		if len(actualInitialBytes) < minHeaderSizeForDescFields {
			return "", true, fmt.Errorf("KDMV file %s is too small (%d bytes) to read descriptor offset/size fields", filePath, len(actualInitialBytes))
		}

		descriptorOffsetSectors := binary.LittleEndian.Uint64(actualInitialBytes[descriptorOffsetInHeaderPos : descriptorOffsetInHeaderPos+8])
		descriptorSizeSectors := binary.LittleEndian.Uint64(actualInitialBytes[descriptorSizeInHeaderPos : descriptorSizeInHeaderPos+8])

		if descriptorSizeSectors == 0 {
			return "", true, fmt.Errorf("VMDK KDMV header in %s indicates zero sectors for descriptor size", filePath)
		}

		descriptorOffsetBytes := descriptorOffsetSectors * sectorSize
		descriptorSizeInBytes := descriptorSizeSectors * sectorSize

		if descriptorSizeInBytes > maxDescriptorSizeBytes {
			return "", true, fmt.Errorf("VMDK descriptor size %d bytes in %s exceeds maximum allowed (%d bytes)",
				descriptorSizeInBytes, filePath, maxDescriptorSizeBytes)
		}

		descriptorContentBytes := make([]byte, descriptorSizeInBytes)
		bytesRead, err := file.ReadAt(descriptorContentBytes, int64(descriptorOffsetBytes))
		if err != nil && err != io.EOF {
			return "", true, fmt.Errorf("failed to read descriptor from KDMV file %s (offset %d, size %d): %w",
				filePath, descriptorOffsetBytes, descriptorSizeInBytes, err)
		}
		if uint64(bytesRead) < descriptorSizeInBytes {
			return "", true, fmt.Errorf("read fewer bytes (%d) than expected for descriptor in KDMV file %s (expected %d at offset %d): %w",
				bytesRead, filePath, descriptorSizeInBytes, descriptorOffsetBytes, io.ErrUnexpectedEOF)
		}

		return string(descriptorContentBytes), true, nil
	}

	return "", false, fmt.Errorf("file %s is not a recognized VMDK format (neither descriptor-only nor KDMV)", filePath)
}
