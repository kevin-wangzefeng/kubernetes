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

package priorities

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// NodeTaints hold the node lister
type TaintToleration struct {
	nodeLister algorithm.NodeLister
}

// NewTaintTolerationPriority
func NewTaintTolerationPriority(nodeLister algorithm.NodeLister) algorithm.PriorityFunction {
	taintToleration := &TaintToleration{
		nodeLister: nodeLister,
	}
	return taintToleration.ComputeTaintTolerationPriority
}

// CountIntolerableTaintsPreferNoSchedule gives the count of intolerable taints of a pod with effect PreferNoSchedule
func countIntolerableTaintsPreferNoSchedule(taints []api.Taint, tolerations []api.Toleration) (intolerableTaints int) {
	for _, taint := range taints {
		if taint.Effect != api.TaintEffectPreferNoSchedule {
			continue
		}
		intolerable := true
		for _, toleration := range tolerations {
			if toleration.Key != taint.Key {
				continue
			}
			switch toleration.Operator {
			case "", api.TolerationOpEqual:
				if toleration.Value == taint.Value {
					intolerable = false
					break
				}
			case api.TolerationOpExists:
				intolerable = false
				break
			}
		}
		if intolerable {
			intolerableTaints++
		}
	}
	return
}

// getAllTolerationEffectPreferNoSchedule gets the list of all Toleration with Effect PreferNoSchedule
func getAllTolerationEffectPreferNoSchedule(tolerations []api.Toleration) (tolerationList []api.Toleration) {
	for _, toleration := range tolerations {
		if toleration.Effect == api.TaintEffectPreferNoSchedule {
			tolerationList = append(tolerationList, toleration)
		}
	}
	return
}

// ComputeTaintTolerationPriority prepares the priority list for all the nodes
func (s *TaintToleration) ComputeTaintTolerationPriority(pod *api.Pod, nodeNameToInfo map[string]*schedulercache.NodeInfo, nodeLister algorithm.NodeLister) (schedulerapi.HostPriorityList, error) {
	// counts hold the count of intolerable taints of a pod for a given node
	var counts map[string]int

	// The maximum priority value to give to a node
	// Priority values range from 0 - maxPriority
	const maxPriority = 10
	result := []schedulerapi.HostPriority{}

	// the max value of counts
	var maxCount int
	nodes, err := nodeLister.List()
	if err != nil {
		return nil, err
	}
	counts = make(map[string]int)

	// Fetch a list of all toleration with effect PreferNoSchedule
	tolerationList := getAllTolerationEffectPreferNoSchedule(pod.Spec.Tolerations)

	// calculate the intolerable taints for all the nodes
	for _, node := range nodes.Items {
		count := countIntolerableTaintsPreferNoSchedule(node.Spec.Taints, tolerationList)
		counts[node.Name] = count
		if count > maxCount {
			maxCount = count
		}
	}
	for _, node := range nodes.Items {
		fScore := float64(maxPriority)
		if maxCount > 0 {
			fScore = (1.0 - float64(float64(counts[node.Name])/float64(maxCount))) * 10
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: int(fScore)})
	}
	return result, nil
}
