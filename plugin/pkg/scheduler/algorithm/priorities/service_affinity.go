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
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
)

type ServiceAffinity struct {
	serviceLister algorithm.ServiceLister
}

func NewServiceAffinityPriority(serviceLister algorithm.ServiceLister) algorithm.PriorityFunction {
	serviceAffinity := &ServiceAffinity{
		serviceLister: serviceLister,
	}
	return serviceAffinity.CalculateAffinityPriority
}

// CalculateAffinityPriority affiliates a pod to the existing services' pods
// that matches the labels indicated in affinitySelector.
// The more existing pods to affiliate deployed on, the higher priority the node gets.
func (s *ServiceAffinity) CalculateAffinityPriority(pod *api.Pod, podLister algorithm.PodLister, minionLister algorithm.MinionLister) (algorithm.HostPriorityList, error) {
	var maxCount int
	counts := map[string]int{}
	affinitySelector := labels.Set(pod.Spec.AffinitySelector)

	// Logically, we should match pod's affinitySelector with services' selectors.
	// Then, to find services' pods, we need to match pods' labels with services'
	// selectors. Finally, calculate the priorties with affinitySelector and the
	// found pods' labels.
	// As a result of optimization, we match the pod's affinitySelector with
	// existing pods' labels here.
	allPods, err := podLister.List(labels.Everything())
	if err != nil {
		glog.V(10).Infof("PodLister Error")
		return nil, err
	}

	if len(allPods) > 0 {
		for _, onePod := range allPods {
			// Only match pods in the same namespace
			if onePod.Namespace != pod.Namespace {
				continue
			}

			// match affinitySelector with every pod's labels.
			// Every matched label will add to the minion's priority
			for key, val := range onePod.ObjectMeta.Labels {
				if affinitySelector.Has(key) && affinitySelector.Get(key) == val {
					counts[onePod.Spec.NodeName]++
					if counts[onePod.Spec.NodeName] > maxCount {
						maxCount = counts[onePod.Spec.NodeName]
					}
				}
			}
		}
	} else {
		glog.V(10).Infof("No Pods")
	}

	minions, err := minionLister.List()
	if err != nil {
		return nil, err
	}
	result := []algorithm.HostPriority{}
	// score int - scale of 0-10
	// 0 being the lowest priority and 10 being the highest
	for _, minion := range minions.Items {
		// initializing to the default/min minion score of 0
		fScore := float32(0)
		if maxCount > 0 {
			fScore = 10 * (float32(counts[minion.Name]) / float32(maxCount))
		}
		result = append(result, algorithm.HostPriority{Host: minion.Name, Score: int(fScore)})
		glog.V(10).Infof(
			"%v -> %v: ServiceAffinityPriority, Score: (%d)", pod.Name, minion.Name, int(fScore),
		)
	}
	return result, nil
}
