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
	var fileKeyList []int
	var parentFiles []int
	var allSnapshotFiles []int

	diskFiles := extractDiskLayoutFiles(vmlayout.Disk)

	for _, layout := range vmlayout.Snapshot {
		diskLayout := extractDiskLayoutFiles(layout.Disk)
		allSnapshotFiles = append(allSnapshotFiles, diskLayout...)

		if layout.Key.Value == info.Value {
			fileKeyList = append(fileKeyList, int(layout.DataKey)) // The .vmsn file
			fileKeyList = append(fileKeyList, diskLayout...)       // The .vmdk files
		} else if parent != nil && layout.Key.Value == parent.Value {
			parentFiles = append(parentFiles, diskLayout...)
		}
	}

	for _, parentFile := range parentFiles {
		removeKey(&fileKeyList, parentFile)
	}

	for _, file := range allSnapshotFiles {
		removeKey(&diskFiles, file)
	}

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

	if isCurrent {
		for _, diskFile := range diskFiles {
			file := fileKeyMap[diskFile]
			size += int(file.Size)
		}
	}

	return size
}
