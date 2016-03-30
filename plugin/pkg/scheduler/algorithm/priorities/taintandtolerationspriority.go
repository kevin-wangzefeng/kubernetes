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
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
)

type listTolerationPreferNoSchedule map[string]*TolerationOpAndValue
type listTaintPreferNoSchedule map[string]string

type TolerationOpAndValue struct {
	value string
	api.TolerationOperator
}

type NodeTaints struct {
	nodeLister algorithm.NodeLister
}

func NewTaintTolerationPriority(nodeLister1 algorithm.NodeLister) algorithm.PriorityFunction {
	nodeTaints := &NodeTaints{
		nodeLister: nodeLister1,
	}
	return nodeTaints.ComputeTaintTolerationPriority
}

// This function gets a list all PreferNoschedule Taint Effects for a node and returns a map[key]value
func getAllTaintEffectPreferNoSchedule(taints []api.Taint) (taintList listTaintPreferNoSchedule) {

	//store the key, value for all the taints having the effect PreferNoSchedule

	taintList = make(listTaintPreferNoSchedule)
	for _, taint := range taints {
		if taint.Effect == api.TaintEffectPreferNoSchedule {
			// Add this in the map
			taintList[taint.Key] = taint.Value
		}
	}
	return
}

func compareTaintListWithTolerationList(taintList listTaintPreferNoSchedule,
	tolerationList listTolerationPreferNoSchedule) (matchCount int) {

	matchCount = 0

	for taintKey, taintValue := range taintList {
		if tolerationOpAndValue, ok := tolerationList[taintKey]; ok {
			//match with the operator specified
			switch tolerationOpAndValue.TolerationOperator {
			case api.TolerationOpEqual:
				{
					if tolerationOpAndValue.value == taintValue {
						matchCount++
					}
				}
			case api.TolerationOpExists:
				{
					if strings.Contains(tolerationOpAndValue.value, taintValue) == true {
						matchCount++
					}
				}
			default:
				{
					glog.V(2).Infof("TolerationOperator not found ", tolerationOpAndValue.TolerationOperator)
				}
			}

		}
	}

	return

}

// Get a list of Key and TolerationOpAndValue for all the Toleration with the PreferNoSchedule Effect.
func getAllTolerationEffectPreferNoSchedule(tolerations []api.Toleration) (tolerationList listTolerationPreferNoSchedule) {

	//store the key and {value,TolerationOperator} for all the toleration having the effect PreferNoSchedule
	tolerationList = make(listTolerationPreferNoSchedule)
	for _, toleration := range tolerations {
		if toleration.Effect == api.TaintEffectPreferNoSchedule {
			tolerationList[toleration.Key] = &TolerationOpAndValue{value: toleration.Value,
				TolerationOperator: toleration.Operator}
		}
	}

	return
}

func (s *NodeTaints) ComputeTaintTolerationPriority(pod *api.Pod, machinesToPods map[string][]*api.Pod, podLister algorithm.PodLister, nodeLister algorithm.NodeLister) (schedulerapi.HostPriorityList, error) {

	nodes, err := nodeLister.List()
	if err != nil {
		return nil, err
	}

	result := []schedulerapi.HostPriority{}

	for _, node := range nodes.Items {

		result = append(result, calculateNodesPriority(pod, node))

	}
	return result, nil
}

// PreferNoSchedule taint is only considered here to arrive at the priority.
// This function assigns the priorities between 0-10
// 0 being the lowest and 10 being the highest priority

//

// Case 1: Nodes with no Taints get the highest priority (i.e 10)
// Case 2: if all of the Node's TaintEffectPreferNoSchedule  match Pods tolerations, the priority is highest.
// Case 3: if only a few taints match the pods toleration effect TaintEffectPreferNoSchedule, give the score accordingly.
// Case 4: if none of the Nodes taints match the pods tolerations Effect TaintEffectPreferNoSchedule, give zero priority.

// Priority function:
// p(N) = (10*(total matching taint effect))/ (total taint effect in the Node)

func calculateNodesPriority(pod *api.Pod, node api.Node) schedulerapi.HostPriority {

	var nodeScore int

	// Get a list of all the pod tolerations matching the effect TaintEffectPreferNoSchedule
	tolerationList := getAllTolerationEffectPreferNoSchedule(pod.Spec.Tolerations)

	// Get a list of all the node taints matching the effect TaintEffectPreferNoSchedule
	taintList := getAllTaintEffectPreferNoSchedule(node.Spec.Taints)

	if len(taintList) > 0 {

		// Compare the two list (i.e tolerationList and taintList)
		// find the total number of matching taint effect in node and pod
		matchingValues := compareTaintListWithTolerationList(taintList, tolerationList)

		nodeScore = int((10 * matchingValues) / (len(taintList)))
	} else {
		// this case happens when there are no taints or when there are no taints with
		// effect TaintEffectPreferNoSchedule
		// in both case the priority is 10

		nodeScore = 10
	}

	return schedulerapi.HostPriority{
		Host:  node.Name,
		Score: nodeScore,
	}

}
