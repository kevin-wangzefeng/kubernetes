/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// Make a node with the given count of Taints ( with effect TaintEffectPreferNoSchedule)
// Only TaintEffectPreferNoSchedule is considered in priority function
// No need to consider other Effect

func makeNodeWithTaints(node string, taintCount int) api.Node {
	var taints []api.Taint
	for i := 0; i < taintCount; i++ {
		taints = append(taints, api.Taint{
			Key:    "Key" + strconv.FormatInt(int64(i), 10),
			Value:  "Value" + strconv.FormatInt(int64(i), 10),
			Effect: api.TaintEffectPreferNoSchedule,
		})
	}

	taintsData, _ := json.Marshal(taints)
	return api.Node{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				api.TaintsAnnotationKey: string(taintsData),
			},
		},
	}
}

// Make a  pod with the given count of Tolerations ( with effect TaintEffectPreferNoSchedule)
// Only TaintEffectPreferNoSchedule is considered in priority function
// No need to consider other Effect

func makePodWithToleration(op api.TolerationOperator, tolerationCount int) api.Pod {

	var tolerationArray []api.Toleration
	for i := 0; i < tolerationCount; i++ {
		tolerationArray = append(tolerationArray, api.Toleration{
			Key:      "Key" + strconv.FormatInt(int64(i), 10),
			Value:    "Value" + strconv.FormatInt(int64(i), 10),
			Operator: op,
			Effect:   api.TaintEffectPreferNoSchedule,
		})
	}

	tolerationsData, _ := json.Marshal(tolerationArray)
	return api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				api.TolerationsAnnotationKey: string(tolerationsData),
			},
		},
	}
}

// This function will create a set of nodes and pods and test the priority
// Nodes with zero,one,two,three,four and hundred taints are created
// Pods with zero,one,two,three,four and hundred tolerations are created

func TestTaintAndToleration(t *testing.T) {

	//Pod with no toleration
	podWithNoToleration := makePodWithToleration(api.TolerationOpExists, 0)

	// Pod with one toleration
	podWithOneToleration := makePodWithToleration(api.TolerationOpExists, 1)

	// Pod with two toleration
	podWithTwoToleration := makePodWithToleration(api.TolerationOpExists, 2)

	// Pod with three tolerations
	podWithThreeToleration := makePodWithToleration(api.TolerationOpExists, 3)

	// Pod with four tolerations
	podWithFourToleration := makePodWithToleration(api.TolerationOpExists, 4)

	// Pod with hundred toleration
	podWithHundredToleration := makePodWithToleration(api.TolerationOpEqual, 100)

	// Node with no taints
	nodeWithNoTaints := makeNodeWithTaints("Node0", 0)

	// Nodes with one taints
	nodeWithOneTaint := makeNodeWithTaints("Node1", 1)

	// Nodes with two taints
	nodeWithTwoTaints := makeNodeWithTaints("Node2", 2)

	// Nodes with three taints
	nodeWithThreeTaints := makeNodeWithTaints("Node3", 3)

	// Nodes with four taints
	nodeWithFourTaints := makeNodeWithTaints("Node4", 4)

	// Node with hundred taints
	nodeWithHundredTaints := makeNodeWithTaints("Node100", 100)

	tests := []struct {
		pod          *api.Pod
		pods         []*api.Pod
		nodes        []api.Node
		expectedList schedulerapi.HostPriorityList
		test         string
	}{
		{
			pod: &podWithNoToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
				nodeWithThreeTaints,
				nodeWithFourTaints,
				nodeWithHundredTaints},
			test: "test priority for a pod with No Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 9},
				{Host: "Node2", Score: 9},
				{Host: "Node3", Score: 9},
				{Host: "Node4", Score: 9},
				{Host: "Node100", Score: 0},
			},
		},
		{
			pod: &podWithOneToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
			},
			test: "test priority for a pod with One Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 10},
				{Host: "Node2", Score: 0},
			},
		},
		{
			pod: &podWithTwoToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
				nodeWithThreeTaints,
				nodeWithFourTaints,
				nodeWithHundredTaints,
			},
			test: "test priority for a pod with Two Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 10},
				{Host: "Node2", Score: 10},
				{Host: "Node3", Score: 9},
				{Host: "Node4", Score: 9},
				{Host: "Node100", Score: 0},
			},
		},
		{
			pod: &podWithThreeToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
				nodeWithThreeTaints,
				nodeWithFourTaints,
				nodeWithHundredTaints},

			test: "test priority for a pod with Three Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 10},
				{Host: "Node2", Score: 10},
				{Host: "Node3", Score: 10},
				{Host: "Node4", Score: 9},
				{Host: "Node100", Score: 0},
			},
		},
		{
			pod: &podWithFourToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
				nodeWithThreeTaints,
				nodeWithFourTaints,
				nodeWithHundredTaints},
			test: "test priority for a pod with Four Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 10},
				{Host: "Node2", Score: 10},
				{Host: "Node3", Score: 10},
				{Host: "Node4", Score: 10},
				{Host: "Node100", Score: 0},
			},
		},
		{
			pod: &podWithHundredToleration,
			nodes: []api.Node{
				nodeWithNoTaints,
				nodeWithOneTaint,
				nodeWithTwoTaints,
				nodeWithThreeTaints,
				nodeWithFourTaints,
				nodeWithHundredTaints},
			test: "test priority for a pod with Four Toleration and nodes with multiple taints",
			pods: []*api.Pod{
				{},
			},
			expectedList: []schedulerapi.HostPriority{
				{Host: "Node0", Score: 10},
				{Host: "Node1", Score: 10},
				{Host: "Node2", Score: 10},
				{Host: "Node3", Score: 10},
				{Host: "Node4", Score: 10},
				{Host: "Node100", Score: 10},
			},
		},
	}
	for _, test := range tests {
		nodeNameToInfo := schedulercache.CreateNodeNameToInfoMap(test.pods)
		taintToleration := TaintToleration{nodeLister: algorithm.FakeNodeLister(api.NodeList{Items: test.nodes})}
		list, err := taintToleration.ComputeTaintTolerationPriority(
			test.pod,
			nodeNameToInfo,
			algorithm.FakeNodeLister(api.NodeList{Items: test.nodes}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(test.expectedList, list) {
			t.Errorf("%s: expected %#v, got %#v", test.test, test.expectedList, list)
		}
	}

}
