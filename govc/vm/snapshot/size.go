/*
Copyright (c) 2016 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snapshot

import "github.com/vmware/govmomi/vim25/types"

func extractDiskLayoutFiles(diskLayoutList []types.VirtualMachineFileLayoutExDiskLayout) []int {
	var result []int

	for _, layoutExDisk := range diskLayoutList {
		for _, link := range layoutExDisk.Chain {
			for i := range link.FileKey { // diskDescriptor, diskExtent pairs (among others)
				result = append(result, int(link.FileKey[i]))
			}
		}
	}

	return result
}

func removeKey(l *[]int, key int) {
	for i, k := range *l {
		if k == key {
			*l = append((*l)[:i], (*l)[i+1:]...)
			break
		}
	}
}

func SnapshotSize(info types.ManagedObjectReference, parent *types.ManagedObjectReference, vmlayout *types.VirtualMachineFileLayoutEx, isCurrent bool) int {

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
		if layout.Key.Value == info.Value {

			// gather file keys if snapshot is current
			fileKeyList = append(fileKeyList, int(layout.DataKey)) // The .vmsn file
			fileKeyList = append(fileKeyList, diskLayout...)       // The .vmdk files

			// if there is a parent, and the MOID for the parent matches the
			// MOID of the layout we are looking at
		} else if parent != nil && layout.Key.Value == parent.Value {
			parentFiles = append(parentFiles, diskLayout...)
		}
	}

	// remove file keys associated with parent snapshot
	for _, parentFile := range parentFiles {
		removeKey(&fileKeyList, parentFile)
	}

	// remove all snapshot files from the list associated with the VM itself
	for _, file := range allSnapshotFiles {
		removeKey(&diskFiles, file)
	}

	// build a map of vm.LayoutEx.File.Key to vm.LayoutEx.File to make
	// retrieving the size for a specific file easier later
	fileKeyMap := make(map[int]types.VirtualMachineFileLayoutExFileInfo)
	for _, file := range vmlayout.File {
		fileKeyMap[int(file.Key)] = file
	}

	size := 0

	for _, fileKey := range fileKeyList {
		file := fileKeyMap[fileKey]
		if parent != nil ||
			(file.Type != string(types.VirtualMachineFileLayoutExFileTypeDiskDescriptor) &&
				file.Type != string(types.VirtualMachineFileLayoutExFileTypeDiskExtent)) {
			size += int(file.Size)
		}
	}

	// if the snapshot is active, add up all associated disk files not
	// directly associated with the snapshot layout
	if isCurrent {
		for _, diskFile := range diskFiles {
			file := fileKeyMap[diskFile]
			size += int(file.Size)
		}
	}

	return size
}
