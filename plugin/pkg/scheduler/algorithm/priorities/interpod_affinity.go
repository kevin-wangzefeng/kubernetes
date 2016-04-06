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
	"fmt"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	priorityutil "k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/priorities/util"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// RequiredDuringScheduling affinity is not symmetric, and there is an implicit PreferredDuringScheduling affinity rule
// corresponding to every RequiredDuringScheduling affinity rule.
// hardPodAffinityImplicitWeight represents the weight of implicit PreferredDuringScheduling affinity rule.
const HardPodAffinityImplicitWeight int = 1

type InterPodAffinity struct {
	info 		predicates.NodeInfo
	nodeLister  algorithm.NodeLister
	podLister 	algorithm.PodLister
}

func NewInterPodAffinityPriority(info predicates.NodeInfo, nodeLister algorithm.NodeLister, podLister algorithm.PodLister) algorithm.PriorityFunction {
	interPodAffinity := &InterPodAffinity{
		info: 		info,
		nodeLister: nodeLister,
		podLister:  podLister,
	}
	return interPodAffinity.CalculateInterPodAffinityPriority
}

// countPodsThatMatchPodAffinityTerm counts the number of given pods that match the podAffinityTerm.
func countPodsThatMatchPodAffinityTerm(nodeInfo predicates.NodeInfo, pod *api.Pod, podsForMatching []*api.Pod, node *api.Node, podAffinityTerm api.PodAffinityTerm) (int, error) {
	labelSelector, err := unversioned.LabelSelectorAsSelector(podAffinityTerm.LabelSelector)
	if err != nil {
		return 0, err
	}

	matchedCount := 0
	names := priorityutil.GetNamespacesFromPodAffinityTerm(pod, podAffinityTerm)
	filteredPods := priorityutil.FilterPodsByNameSpaces(names, podsForMatching)
	for _, filteredPod := range filteredPods {
		if labelSelector.Matches(labels.Set(filteredPod.Labels)) {
			filteredPodNode, err := nodeInfo.GetNodeInfo(filteredPod.Spec.NodeName)
			if err != nil {
				return 0, err
			}
			if priorityutil.NodesHaveSameTopologyKey(filteredPodNode, node, podAffinityTerm.TopologyKey) {
				matchedCount++
			}
		}
	}
	return matchedCount, nil
}

type TopologyCounts map[string]int

func (tc TopologyCounts) GetIndex(topologyKey, topologyValue string) string {
	return fmt.Sprintf("%s, %s", topologyKey, topologyValue)
}

// if the topology key is empty, count weight for each node label value as index,
// otherwise only count weight for node.Labels[topologyKey] as index.
func (tc TopologyCounts) CountWeightByTopologykey(node *api.Node, weight int, topologyKey string) {
	if len(topologyKey) == 0 && node.Labels != nil {
		for k, v := range node.Labels {
			index := tc.GetIndex(k, v)
			tc[index] += weight
		}
	} else if len(topologyKey) > 0 && node.Labels != nil && len(node.Labels[topologyKey]) > 0 {
		index := tc.GetIndex(topologyKey, node.Labels[topologyKey])
		tc[index] += weight
	}
	return
}

// CountWeightByPodMatchAffinityTerm counts the weight to topologyCounts for all the given pods that match the podAffinityTerm.
func (tc TopologyCounts) CountWeightByPodMatchAffinityTerm(nodeInfo predicates.NodeInfo, pod *api.Pod, podsForMatching []*api.Pod, weight int, podAffinityTerm api.PodAffinityTerm, node *api.Node) error {
	if weight == 0 {
		return nil
	}
	// get the pods which are there in that particular node
	podsMatchedCount, err := countPodsThatMatchPodAffinityTerm(nodeInfo, pod, podsForMatching, node, podAffinityTerm)
	if err != nil {
		return err
	}
	if podsMatchedCount > 0 {
		tc.CountWeightByTopologykey(node, weight*podsMatchedCount, podAffinityTerm.TopologyKey)
	}
	return nil
}

// compute a sum by iterating through the elements of weightedPodAffinityTerm and adding
// "weight" to the sum if the corresponding PodAffinityTerm is satisfied for
// that node; the node(s) with the highest sum are the most preferred.
// Symmetry need to be considered for preferredDuringSchedulingIgnoredDuringExecution from podAffinity & podAntiAffinity,
// symmetry need to be considered for hard requirements from podAffinity
func (ipa *InterPodAffinity) CalculateInterPodAffinityPriority(pod *api.Pod, nodeNameToInfo map[string]*schedulercache.NodeInfo, nodeLister algorithm.NodeLister) (schedulerapi.HostPriorityList, error) {
	nodes, err := nodeLister.List()
	if err != nil {
		return nil, err
	}
	allPods, err := ipa.podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	affinity, err := api.GetAffinityFromPodAnnotations(pod.Annotations)
	if err != nil {
		return nil, err
	}
	topologyCounts := TopologyCounts{}
	for _, node := range nodes.Items {
		// count weights for the weighted pod affinity
		if affinity.PodAffinity != nil {
			for _, weightedTerm := range affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				err := topologyCounts.CountWeightByPodMatchAffinityTerm(ipa.info, pod, allPods, weightedTerm.Weight, weightedTerm.PodAffinityTerm, &node)
				if err != nil {
					return nil, err
				}
			}
		}

		// count weights for the weighted pod anti-affinity
		if affinity.PodAntiAffinity != nil {
			for _, weightedTerm := range affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				err := topologyCounts.CountWeightByPodMatchAffinityTerm(ipa.info, pod, allPods, (0 - weightedTerm.Weight), weightedTerm.PodAffinityTerm, &node)
				if err != nil {
					return nil, err
				}
			}
		}

		// reverse direction checking: count weights for the inter-pod affinity/anti-affinity rules
		// that are indicated by existing pods on the node.
		for _, ep := range allPods {
			epForMatching := []*api.Pod{ep}
			epAffinity, err := api.GetAffinityFromPodAnnotations(ep.Annotations)
			if err != nil {
				return nil, err
			}

			// count the implicit weight for the hard pod affinity indicated by the existing pod.
			if epAffinity.PodAffinity != nil {
				var podAffinityTerms []api.PodAffinityTerm
				if len(epAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0 {
					podAffinityTerms = epAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
				}
				// TODO: Uncomment this block when implement RequiredDuringSchedulingRequiredDuringExecution.
				//if len(affinity.PodAffinity.RequiredDuringSchedulingRequiredDuringExecution) != 0 {
				//	podAffinityTerms = append(podAffinityTerms, affinity.PodAffinity.RequiredDuringSchedulingRequiredDuringExecution...)
				//}
				for _, epAffinityTerm := range podAffinityTerms {
					err := topologyCounts.CountWeightByPodMatchAffinityTerm(ipa.info, pod, epForMatching, HardPodAffinityImplicitWeight, epAffinityTerm, &node)
					if err != nil {
						return nil, err
					}
				}
			}

			// count weight for the weighted pod affinity indicated by the existing pod.
			if epAffinity.PodAffinity != nil {
				for _, epWeightedTerm := range epAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					err := topologyCounts.CountWeightByPodMatchAffinityTerm(ipa.info, pod, epForMatching, epWeightedTerm.Weight, epWeightedTerm.PodAffinityTerm, &node)
					if err != nil {
						return nil, err
					}
				}
			}

			// count weight for the weighted pod anti-affinity indicated by the existing pod.
			if epAffinity.PodAntiAffinity != nil {
				for _, epWeightedTerm := range epAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					err := topologyCounts.CountWeightByPodMatchAffinityTerm(ipa.info, pod, epForMatching, (0 - epWeightedTerm.Weight), epWeightedTerm.PodAffinityTerm, &node)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	// convert the topology key based weights to the node name based weights
	var maxCount int
	var minCount int
	counts := map[string]int{}
	for countIndex, count := range topologyCounts {
		for _, node := range nodes.Items {
			for k, v := range node.Labels {
				if topologyCounts.GetIndex(k, v) == countIndex {
					counts[node.Name] = counts[node.Name] + count
					if counts[node.Name] > maxCount {
						maxCount = counts[node.Name]
					}
					if counts[node.Name] < minCount {
						minCount = counts[node.Name]
					}
				}
			}
		}
	}

	// calculate final priority score for each node
	result := []schedulerapi.HostPriority{}
	for _, node := range nodes.Items {
		fScore := float64(0)
		if (maxCount - minCount) > 0 {
			fScore = 10 * (float64(counts[node.Name]-minCount) / float64(maxCount-minCount))
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: int(fScore)})
		glog.V(10).Infof(
			"%v -> %v: InterPodAffinityPriority, Score: (%d)", pod.Name, node.Name, int(fScore),
		)
	}

	return result, nil
}
