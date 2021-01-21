package snapshot

import "github.com/vmware/govmomi/vim25/types"

// returns vm.LayoutEx.Disk[].Chain[].FileKey[]
// flattens file pairs into a single list
func extractDiskLayoutFiles(diskLayoutList []types.VirtualMachineFileLayoutExDiskLayout) []int {
	var result []int

	// NOTE: layoutExDisk.Key is not referenced here.
	for _, layoutExDisk := range diskLayoutList {
		for _, link := range layoutExDisk.Chain {
			for i := range link.FileKey { // diskDescriptor, diskExtent pairs (among others)
				result = append(result, int(link.FileKey[i]))
			}
		}
	}

	return result
}

// removeKey (appears) to remove a given file key from the list
func removeKey(l *[]int, key int) {
	for i, k := range *l {
		if k == key {
			*l = append((*l)[:i], (*l)[i+1:]...)
			break
		}
	}
}

/*
	Questions:

	- What is `parent`?
	  - If `parent` is the parent snapshot, how do we know?
	- How do we know if isCurrent should be false?
	  - check against vm.Snapshot.CurrentSnapshot?
*/
func SnapshotSize(

	// info == vm.Snapshot
	info *types.VirtualMachineSnapshotInfo,

	// this function cannot be expected to process a snapshot tree. Instead,
	// something else looks at the tree and calls this for each snapshot. This
	// value would be the MOID for that snapshot.
	parent *types.ManagedObjectReference,

	// files that comprise this virtual machine
	vmlayout *types.VirtualMachineFileLayoutEx,

	// This can be determined by comparing info.CurrentSnapshot.Value?
	isCurrent bool,
) int {

	// flat list of diskDescriptor, diskExtent values
	var fileKeyList []int

	// files associated with parent snapshot only
	var parentFiles []int

	// all snapshot files, regardless of whether snapshot has a parent
	var allSnapshotFiles []int

	diskFiles := extractDiskLayoutFiles(vmlayout.Disk)

	// vmlayout.Snapshot == Layout of each snapshot of the virtual machine.
	for _, layout := range vmlayout.Snapshot {
		diskLayout := extractDiskLayoutFiles(layout.Disk)

		allSnapshotFiles = append(allSnapshotFiles, diskLayout...)

		// if the layout MOID matches active snapshot MOID
		// Q: Why match on this? Is this because the snapshot is growing?
		if layout.Key.Value == info.CurrentSnapshot.Value {

			// gather file keys if snapshot is current?
			fileKeyList = append(fileKeyList, int(layout.DataKey)) // The .vmsn file
			fileKeyList = append(fileKeyList, diskLayout...)       // The .vmdk files

			// if there is a parent, and the MOID for the parent matches the
			// MOID of the layout we are looking at
		} else if parent != nil && layout.Key.Value == parent.Value {
			parentFiles = append(parentFiles, diskLayout...)
		}
	}

	// remove file keys associated with parent snapshot?
	// Q: Why are the keys for these files removed?
	for _, parentFile := range parentFiles {
		removeKey(&fileKeyList, parentFile)
	}

	// remove all snapshot files from the list associated with the VM itself?
	// leaving just the non-snapshot files? our baseline for comparison?
	for _, file := range allSnapshotFiles {
		removeKey(&diskFiles, file)
	}

	// build a map of vm.LayoutEx.File.Key to vm.LayoutEx.File, presumably to
	// make retrieving the size for a specific file easier later
	fileKeyMap := make(map[int]types.VirtualMachineFileLayoutExFileInfo)
	for _, file := range vmlayout.File {
		fileKeyMap[int(file.Key)] = file
	}

	size := 0

	// why are we excluding diskExtent files if there is a parent for the
	// snapshot?
	// Q: does `if parent != nil` filter to files that have a parent? If
	// so, is the idea that snapshot files *have* a parent, but other
	// files do not?
	for _, fileKey := range fileKeyList {
		file := fileKeyMap[fileKey]
		if parent != nil ||
			(file.Type != string(types.VirtualMachineFileLayoutExFileTypeDiskDescriptor) &&
				file.Type != string(types.VirtualMachineFileLayoutExFileTypeDiskExtent)) {
			size += int(file.Size)
		}
	}

	// if the snapshot *is* active, we add up all associated files?
	if isCurrent {
		for _, diskFile := range diskFiles {
			file := fileKeyMap[diskFile]
			size += int(file.Size)
		}
	}

	return size
}
