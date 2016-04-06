/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package util

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
)

// For each of these resources, a pod that doesn't request the resource explicitly
// will be treated as having requested the amount indicated below, for the purpose
// of computing priority only. This ensures that when scheduling zero-request pods, such
// pods will not all be scheduled to the machine with the smallest in-use request,
// and that when scheduling regular pods, such pods will not see zero-request pods as
// consuming no resources whatsoever. We chose these values to be similar to the
// resources that we give to cluster addon pods (#10653). But they are pretty arbitrary.
// As described in #11713, we use request instead of limit to deal with resource requirements.
const DefaultMilliCpuRequest int64 = 100             // 0.1 core
const DefaultMemoryRequest int64 = 200 * 1024 * 1024 // 200 MB

// GetNonzeroRequests returns the default resource request if none is found or what is provided on the request
// TODO: Consider setting default as a fixed fraction of machine capacity (take "capacity api.ResourceList"
// as an additional argument here) rather than using constants
func GetNonzeroRequests(requests *api.ResourceList) (int64, int64) {
	var outMilliCPU, outMemory int64
	// Override if un-set, but not if explicitly set to zero
	if _, found := (*requests)[api.ResourceCPU]; !found {
		outMilliCPU = DefaultMilliCpuRequest
	} else {
		outMilliCPU = requests.Cpu().MilliValue()
	}
	// Override if un-set, but not if explicitly set to zero
	if _, found := (*requests)[api.ResourceMemory]; !found {
		outMemory = DefaultMemoryRequest
	} else {
		outMemory = requests.Memory().Value()
	}
	return outMilliCPU, outMemory
}

// FilterPodsByNameSpaces filters the pods based the given list of namespaces
func FilterPodsByNameSpaces(names sets.String, pods []*api.Pod) []*api.Pod {
	if len(pods) == 0 || len(names) == 0 {
		return pods
	}
	result := []*api.Pod{}
	for _, pod := range pods {
		if names.Has(pod.Namespace) {
			result = append(result, pod)
		}
	}
	return result
}

// IfNodeHasTopologyKey checks if given node labels has non-nil value with given topologykey as label key.
// If the topologyKey is nil/empty, regard as the node labels has the topologyKey.
func IfNodeHasTopologyKey(node *api.Node, topologyKey string) bool {
	if len(topologyKey) == 0 {
		return true
	} else if node.Labels != nil && len(node.Labels[topologyKey]) > 0 {
		return true
	}
	return false
}

// NodeHasTopologyKey checks if given node labels has non-nil value with given topologykey as label key.
// If the topologyKey is nil/empty, regard as the node labels has the topologyKey.
func NodesHaveSameTopologyKey(nodeA *api.Node, nodeB *api.Node, topologyKey string) bool {
	// TODO kevin-wangzefeng: update comments
	if len(topologyKey) == 0 {
		return true
	}
	return nodeA.Labels != nil && nodeB.Labels != nil && len(nodeA.Labels[topologyKey]) > 0 && nodeA.Labels[topologyKey] == nodeB.Labels[topologyKey]
}

// GetNamespacesFromPodAffinityTerm returns a set of names
// according to the namespaces indicated in podAffinityTerm.
// if the NameSpaces is nil considers the given pod's namespace
// if the Namespaces is empty list then considers all the namespaces
func GetNamespacesFromPodAffinityTerm(pod *api.Pod, podAffinityTerm api.PodAffinityTerm) sets.String {
	names := sets.String{}
	if podAffinityTerm.Namespaces == nil {
		names.Insert(pod.Namespace)
	} else if len(podAffinityTerm.Namespaces) != 0 {
		for _, nameSpace := range podAffinityTerm.Namespaces {
			names.Insert(nameSpace.Name)
		}
	}
	return names
}
