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

type SelectorAffinity struct {
	serviceLister    algorithm.ServiceLister
	controllerLister algorithm.ControllerLister
}

func NewSelectorAffinityPriority(serviceLister algorithm.ServiceLister, controllerLister algorithm.ControllerLister) algorithm.PriorityFunction {
	selectorAffinity := &SelectorAffinity{
		serviceLister:    serviceLister,
		controllerLister: controllerLister,
	}
	return selectorAffinity.CalculateAffinityPriority
}

func (s *selectorAffinity) CalculateAffinityPriority(pod *api.Pod, podLister alogorithm.PodLister, minionLister algorithm.MinionLister) (algorithm.HostPriorityList, error) {
	affinitySelector := labels.Set(pod.Spec.AffinitySelector)
	
	// var affinityPods []*api.Pod
	var maxCount int
	counts := map[string]int{}
	allPods,err := podLister.List(labels.Everything())
	
	if err == nil {
		if len(allPods) > 0 {
			for _, onePod := range allPods {
				if onePod.Namespace != pod.Namespace {
					continue
				} 
				if affinitySelector.Matches(labels.Set(pod.Objectmeta.Labels)) {
					//affinityPods = append(affinityPods, onePod)  //也许没用
					counts[onePod.Spec.NodeName]++
					if counts[onePod.Spec.NodeName] > maxCount {
						maxCount = counts[onePod.Spec.NodeName]
					}
				}
			}
		} else {
			glog.V(10).Infof("No Pods")
		}
	} else {
		glog.V(10).Infof("PodLister Error")
	}

	minions, err := minionLister.List()
	if err != nil {
		return nil, err
	}
	result := []algorithm.HostPriority{}
	//score int - scale of 0-10
	// 0 being the lowest priority and 10 being the highest
	for _, minion := range minions.Items {
		// initializing to the default/max minion score of 10
		fScore := float32(10)
		if maxCount > 0 {
			fScore = 10 * (float32(counts[minion.Name]) / float32(maxCount))
		}
		result = append(result, algorithm.HostPriority{Host: minion.Name, Score: int(fScore)})
		glog.V(10).Infof(
			"%v -> %v: SelectorSpreadPriority, Score: (%d)", pod.Name, minion.Name, int(fScore),
		)
	}
	return result, nil
}

