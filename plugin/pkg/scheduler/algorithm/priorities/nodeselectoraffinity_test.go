/*
 Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"reflect"
	_ "sort"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
)

func TestNodeAffinityPriority(t *testing.T) {

	label1 := map[string]string{"foo": "bar"}
	label2 := map[string]string{"key": "value"}
	label3 := map[string]string{"az": "az1"}
	label4 := map[string]string{"abc": "az11", "def": "az22"}
	label5 := map[string]string{"foo": "bar", "key": "value", "az": "az1"}

	affinity1 := api.Affinity{
		HardNodeAffinity: nil,
		SoftNodeAffinity: []api.SoftNodeAffinityTerm{{Weight: 2, MatchExpressions: []api.NodeSelectorRequirement{{Key: "foo", Operator: api.NodeSelectorOpIn, Values: []string{"bar", "value2"}}}}}}

	affinity2 := api.Affinity{
		HardNodeAffinity: nil,
		SoftNodeAffinity: []api.SoftNodeAffinityTerm{{Weight: 2, MatchExpressions: []api.NodeSelectorRequirement{{Key: "foo", Operator: api.NodeSelectorOpIn, Values: []string{"bar"}}}}, {Weight: 4, MatchExpressions: []api.NodeSelectorRequirement{{Key: "key", Operator: api.NodeSelectorOpIn, Values: []string{"value"}}}}, {Weight: 5, MatchExpressions: []api.NodeSelectorRequirement{{Key: "key", Operator: api.NodeSelectorOpIn, Values: []string{"value"}}, {Key: "foo", Operator: api.NodeSelectorOpIn, Values: []string{"bar"}}, {Key: "az", Operator: api.NodeSelectorOpIn, Values: []string{"az1"}}}}}}

	tests := []struct {
		pod          *api.Pod
		nodes        []api.Node
		expectedList schedulerapi.HostPriorityList
		test         string
	}{
		{
			pod: &api.Pod{Spec: api.PodSpec{Affinity: nil}},
			nodes: []api.Node{
				{ObjectMeta: api.ObjectMeta{Name: "machine1", Labels: label1}},
				{ObjectMeta: api.ObjectMeta{Name: "machine2", Labels: label2}},
				{ObjectMeta: api.ObjectMeta{Name: "machine3", Labels: label3}},
			},
			expectedList: []schedulerapi.HostPriority{{"machine1", 0}, {"machine2", 0}, {"machine3", 0}},
			test:         "all machines are same priority as softnodeaffinity is nil",
		},
		{
			pod: &api.Pod{Spec: api.PodSpec{Affinity: &affinity1}},
			nodes: []api.Node{
				{ObjectMeta: api.ObjectMeta{Name: "machine1", Labels: label4}},
				{ObjectMeta: api.ObjectMeta{Name: "machine2", Labels: label2}},
				{ObjectMeta: api.ObjectMeta{Name: "machine3", Labels: label3}},
			},
			expectedList: []schedulerapi.HostPriority{{"machine1", 0}, {"machine2", 0}, {"machine3", 0}},
			test:         "no machine macthes with softaffinity of pod so all machines priority is zero",
		},
		{
			pod: &api.Pod{Spec: api.PodSpec{Affinity: &affinity1}},
			nodes: []api.Node{
				{ObjectMeta: api.ObjectMeta{Name: "machine1", Labels: label1}},
				{ObjectMeta: api.ObjectMeta{Name: "machine2", Labels: label2}},
				{ObjectMeta: api.ObjectMeta{Name: "machine3", Labels: label3}},
			},
			expectedList: []schedulerapi.HostPriority{{"machine1", 10}, {"machine2", 0}, {"machine3", 0}},
			test:         "only machine1 is macthing with softaffinity of pod",
		},
		{
			pod: &api.Pod{Spec: api.PodSpec{Affinity: &affinity2}},
			nodes: []api.Node{
				{ObjectMeta: api.ObjectMeta{Name: "machine1", Labels: label1}},
				{ObjectMeta: api.ObjectMeta{Name: "machine5", Labels: label5}},
				{ObjectMeta: api.ObjectMeta{Name: "machine2", Labels: label2}},
			},
			expectedList: []schedulerapi.HostPriority{{"machine1", 1}, {"machine5", 10}, {"machine2", 3}},
			test:         "all machines matches the softaffinity of pod but with different priorities ",
		},
	}

	for _, test := range tests {
		nodeAffinity := NodeAffinity{nodeLister: algorithm.FakeNodeLister(api.NodeList{Items: test.nodes})}
		list, err := nodeAffinity.CalculateNodeAffinityPriority(test.pod, nil, nil, algorithm.FakeNodeLister(api.NodeList{Items: test.nodes}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(test.expectedList, list) {
			t.Errorf("%s: \nexpected %#v, \ngot      %#v", test.test, test.expectedList, list)
		}
	}
}
