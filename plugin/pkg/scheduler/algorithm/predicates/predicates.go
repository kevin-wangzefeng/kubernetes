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

package predicates

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

type NodeInfo interface {
	GetNodeInfo(nodeID string) (*api.Node, error)
}

type PersistentVolumeInfo interface {
	GetPersistentVolumeInfo(pvID string) (*api.PersistentVolume, error)
}

type PersistentVolumeClaimInfo interface {
	GetPersistentVolumeClaimInfo(namespace string, pvcID string) (*api.PersistentVolumeClaim, error)
}

type StaticNodeInfo struct {
	*api.NodeList
}

func (nodes StaticNodeInfo) GetNodeInfo(nodeID string) (*api.Node, error) {
	for ix := range nodes.Items {
		if nodes.Items[ix].Name == nodeID {
			return &nodes.Items[ix], nil
		}
	}
	return nil, fmt.Errorf("failed to find node: %s, %#v", nodeID, nodes)
}

type ClientNodeInfo struct {
	*client.Client
}

func (nodes ClientNodeInfo) GetNodeInfo(nodeID string) (*api.Node, error) {
	return nodes.Nodes().Get(nodeID)
}

type CachedNodeInfo struct {
	*cache.StoreToNodeLister
}

// GetNodeInfo returns cached data for the node 'id'.
func (c *CachedNodeInfo) GetNodeInfo(id string) (*api.Node, error) {
	node, exists, err := c.Get(&api.Node{ObjectMeta: api.ObjectMeta{Name: id}})

	if err != nil {
		return nil, fmt.Errorf("error retrieving node '%v' from cache: %v", id, err)
	}

	if !exists {
		return nil, fmt.Errorf("node '%v' is not in cache", id)
	}

	return node.(*api.Node), nil
}

func isVolumeConflict(volume api.Volume, pod *api.Pod) bool {
	if volume.GCEPersistentDisk != nil {
		disk := volume.GCEPersistentDisk

		existingPod := &(pod.Spec)
		for ix := range existingPod.Volumes {
			if existingPod.Volumes[ix].GCEPersistentDisk != nil &&
				existingPod.Volumes[ix].GCEPersistentDisk.PDName == disk.PDName &&
				!(existingPod.Volumes[ix].GCEPersistentDisk.ReadOnly && disk.ReadOnly) {
				return true
			}
		}
	}
	if volume.AWSElasticBlockStore != nil {
		volumeID := volume.AWSElasticBlockStore.VolumeID

		existingPod := &(pod.Spec)
		for ix := range existingPod.Volumes {
			if existingPod.Volumes[ix].AWSElasticBlockStore != nil &&
				existingPod.Volumes[ix].AWSElasticBlockStore.VolumeID == volumeID {
				return true
			}
		}
	}
	if volume.RBD != nil {
		mon := volume.RBD.CephMonitors
		pool := volume.RBD.RBDPool
		image := volume.RBD.RBDImage

		existingPod := &(pod.Spec)
		for ix := range existingPod.Volumes {
			if existingPod.Volumes[ix].RBD != nil {
				mon_m := existingPod.Volumes[ix].RBD.CephMonitors
				pool_m := existingPod.Volumes[ix].RBD.RBDPool
				image_m := existingPod.Volumes[ix].RBD.RBDImage
				if haveSame(mon, mon_m) && pool_m == pool && image_m == image {
					return true
				}
			}
		}
	}
	return false
}

// NoDiskConflict evaluates if a pod can fit due to the volumes it requests, and those that
// are already mounted. If there is already a volume mounted on that node, another pod that uses the same volume
// can't be scheduled there.
// This is GCE, Amazon EBS, and Ceph RBD specific for now:
// - GCE PD allows multiple mounts as long as they're all read-only
// - AWS EBS forbids any two pods mounting the same volume ID
// - Ceph RBD forbids if any two pods share at least same monitor, and match pool and image.
// TODO: migrate this into some per-volume specific code?
func NoDiskConflict(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
	podSpec := &(pod.Spec)
	for ix := range podSpec.Volumes {
		for podIx := range existingPods {
			if isVolumeConflict(podSpec.Volumes[ix], existingPods[podIx]) {
				return false, nil
			}
		}
	}
	return true, nil
}

type VolumeZoneChecker struct {
	nodeInfo NodeInfo
	pvInfo   PersistentVolumeInfo
	pvcInfo  PersistentVolumeClaimInfo
}

// VolumeZonePredicate evaluates if a pod can fit due to the volumes it requests, given
// that some volumes may have zone scheduling constraints.  The requirement is that any
// volume zone-labels must match the equivalent zone-labels on the node.  It is OK for
// the node to have more zone-label constraints (for example, a hypothetical replicated
// volume might allow region-wide access)
//
// Currently this is only supported with PersistentVolumeClaims, and looks to the labels
// only on the bound PersistentVolume.
//
// Working with volumes declared inline in the pod specification (i.e. not
// using a PersistentVolume) is likely to be harder, as it would require
// determining the zone of a volume during scheduling, and that is likely to
// require calling out to the cloud provider.  It seems that we are moving away
// from inline volume declarations anyway.
func NewVolumeZonePredicate(nodeInfo NodeInfo, pvInfo PersistentVolumeInfo, pvcInfo PersistentVolumeClaimInfo) algorithm.FitPredicate {
	c := &VolumeZoneChecker{
		nodeInfo: nodeInfo,
		pvInfo:   pvInfo,
		pvcInfo:  pvcInfo,
	}
	return c.predicate
}

func (c *VolumeZoneChecker) predicate(pod *api.Pod, existingPods []*api.Pod, nodeID string) (bool, error) {
	node, err := c.nodeInfo.GetNodeInfo(nodeID)
	if err != nil {
		return false, err
	}
	if node == nil {
		return false, fmt.Errorf("node not found: %q", nodeID)
	}

	nodeConstraints := make(map[string]string)
	for k, v := range node.ObjectMeta.Labels {
		if k != unversioned.LabelZoneFailureDomain && k != unversioned.LabelZoneRegion {
			continue
		}
		nodeConstraints[k] = v
	}

	if len(nodeConstraints) == 0 {
		// The node has no zone constraints, so we're OK to schedule.
		// In practice, when using zones, all nodes must be labeled with zone labels.
		// We want to fast-path this case though.
		return true, nil
	}

	namespace := pod.Namespace

	manifest := &(pod.Spec)
	for i := range manifest.Volumes {
		volume := &manifest.Volumes[i]
		if volume.PersistentVolumeClaim != nil {
			pvcName := volume.PersistentVolumeClaim.ClaimName
			if pvcName == "" {
				return false, fmt.Errorf("PersistentVolumeClaim had no name: %q", pvcName)
			}
			pvc, err := c.pvcInfo.GetPersistentVolumeClaimInfo(namespace, pvcName)
			if err != nil {
				return false, err
			}

			if pvc == nil {
				return false, fmt.Errorf("PersistentVolumeClaim was not found: %q", pvcName)
			}

			pvName := pvc.Spec.VolumeName
			if pvName == "" {
				return false, fmt.Errorf("PersistentVolumeClaim is not bound: %q", pvcName)
			}

			pv, err := c.pvInfo.GetPersistentVolumeInfo(pvName)
			if err != nil {
				return false, err
			}

			if pv == nil {
				return false, fmt.Errorf("PersistentVolume not found: %q", pvName)
			}

			for k, v := range pv.ObjectMeta.Labels {
				if k != unversioned.LabelZoneFailureDomain && k != unversioned.LabelZoneRegion {
					continue
				}
				nodeV, _ := nodeConstraints[k]
				if v != nodeV {
					glog.V(2).Infof("Won't schedule pod %q onto node %q due to volume %q (mismatch on %q)", pod.Name, nodeID, pvName, k)
					return false, nil
				}
			}
		}
	}

	return true, nil
}

type ResourceFit struct {
	info NodeInfo
}

type TolerationMatch struct {
	info NodeInfo
}

type resourceRequest struct {
	milliCPU int64
	memory   int64
}

var FailedResourceType string

func getResourceRequest(pod *api.Pod) resourceRequest {
	result := resourceRequest{}
	for _, container := range pod.Spec.Containers {
		requests := container.Resources.Requests
		result.memory += requests.Memory().Value()
		result.milliCPU += requests.Cpu().MilliValue()
	}
	return result
}

func CheckPodsExceedingFreeResources(pods []*api.Pod, capacity api.ResourceList) (fitting []*api.Pod, notFittingCPU, notFittingMemory []*api.Pod) {
	totalMilliCPU := capacity.Cpu().MilliValue()
	totalMemory := capacity.Memory().Value()
	milliCPURequested := int64(0)
	memoryRequested := int64(0)
	for _, pod := range pods {
		podRequest := getResourceRequest(pod)
		fitsCPU := totalMilliCPU == 0 || (totalMilliCPU-milliCPURequested) >= podRequest.milliCPU
		fitsMemory := totalMemory == 0 || (totalMemory-memoryRequested) >= podRequest.memory
		if !fitsCPU {
			// the pod doesn't fit due to CPU request
			notFittingCPU = append(notFittingCPU, pod)
			continue
		}
		if !fitsMemory {
			// the pod doesn't fit due to Memory request
			notFittingMemory = append(notFittingMemory, pod)
			continue
		}
		// the pod fits
		milliCPURequested += podRequest.milliCPU
		memoryRequested += podRequest.memory
		fitting = append(fitting, pod)
	}
	return
}

func podName(pod *api.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

// PodFitsResources calculates fit based on requested, rather than used resources
func (r *ResourceFit) PodFitsResources(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
	info, err := r.info.GetNodeInfo(node)
	if err != nil {
		return false, err
	}

	if int64(len(existingPods))+1 > info.Status.Capacity.Pods().Value() {
		glog.V(10).Infof("Cannot schedule Pod %+v, because Node %+v is full, running %v out of %v Pods.", podName(pod), node, len(existingPods), info.Status.Capacity.Pods().Value())
		FailedResourceType = "PodExceedsMaxPodNumber"
		return false, nil
	}

	podRequest := getResourceRequest(pod)
	if podRequest.milliCPU == 0 && podRequest.memory == 0 {
		return true, nil
	}

	pods := append(existingPods, pod)
	_, exceedingCPU, exceedingMemory := CheckPodsExceedingFreeResources(pods, info.Status.Capacity)
	if len(exceedingCPU) > 0 {
		glog.V(10).Infof("Cannot schedule Pod %+v, because Node %v does not have sufficient CPU", podName(pod), node)
		FailedResourceType = "PodExceedsFreeCPU"
		return false, nil
	}
	if len(exceedingMemory) > 0 {
		glog.V(10).Infof("Cannot schedule Pod %+v, because Node %v does not have sufficient Memory", podName(pod), node)
		FailedResourceType = "PodExceedsFreeMemory"
		return false, nil
	}
	glog.V(10).Infof("Schedule Pod %+v on Node %+v is allowed, Node is running only %v out of %v Pods.", podName(pod), node, len(pods)-1, info.Status.Capacity.Pods().Value())
	return true, nil
}

func NewResourceFitPredicate(info NodeInfo) algorithm.FitPredicate {
	fit := &ResourceFit{
		info: info,
	}
	return fit.PodFitsResources
}

func NewSelectorMatchPredicate(info NodeInfo) algorithm.FitPredicate {
	selector := &NodeSelector{
		info: info,
	}
	return selector.PodSelectorMatches
}

func NewTolerationMatchPredicate(info NodeInfo) algorithm.FitPredicate {
	tolerationMatch := &TolerationMatch{
		info: info,
	}
	return tolerationMatch.PodToleratesTaints
}

func PodMatchesNodeLabels(pod *api.Pod, node *api.Node) bool {
	if len(pod.Spec.NodeSelector) == 0 {
		return true
	}
	selector := labels.SelectorFromSet(pod.Spec.NodeSelector)
	return selector.Matches(labels.Set(node.Labels))
}

type NodeSelector struct {
	info NodeInfo
}

func (n *NodeSelector) PodSelectorMatches(pod *api.Pod, existingPods []*api.Pod, nodeID string) (bool, error) {
	node, err := n.info.GetNodeInfo(nodeID)
	if err != nil {
		return false, err
	}
	return PodMatchesNodeLabels(pod, node), nil
}

// PodToleratesTaints matches pod's tolerations against node's taints if the effect is one of the following
// 1. NoSchedule
// 2. NoScheduleNoAdmit
// 3. NoScheduleNoAdmitNoExecute
// If the effect is PreferNoSchedule , then it's skipped.
// If all of a node's taints are tolerated by atleast one of the pod's tolerations ,this node is a candidate for scheduling
// the pod.
func (t *TolerationMatch) PodToleratesTaints(pod *api.Pod, existingPods []*api.Pod, nodeID string) (bool, error) {
	node, err := t.info.GetNodeInfo(nodeID)
	if err != nil {
		return false, err
	}
	return PodTolerationsMatchNodeTaints(pod, node), nil
}

func PodTolerationsMatchNodeTaints(pod *api.Pod, node *api.Node) bool {
	// If the pod doesn't specify any tolerations, need to the check the node for the following
	// two cases.

	// If the node had taints then this pod cannot fit in this node
	// If the node has no taints then it can fit any pod.
	if len(pod.Spec.Tolerations) == 0 {
		nodeTaintsLen := len(node.Status.Taints)
		if nodeTaintsLen == 0 {
			//Node also has no taints so this can fit any pod
			return true
		} else if nodeTaintsLen > 0 {
			//Node has some taints but the pod has no matching toleration
			//so this pod cant fit this node
			return false
		}

		return true
	}
	// If the node doesn't have any taints applied , we don't need to process the pod's tolerations
	// as this pod can be scheduled on to the node, so return true
	if node.Status.Taints == nil || len(node.Status.Taints) == 0 {
		return true
	}
taintLoop:
	for _, currTaint := range node.Status.Taints {
		if currTaint.Effect == api.TaintEffectNoSchedule ||
			currTaint.Effect == api.TaintEffectNoScheduleNoAdmit ||
			currTaint.Effect == api.TaintEffectNoScheduleNoAdmitNoExecute {
			for _, toleration := range pod.Spec.Tolerations {
				// If the toleration operator is "Equal" , then the triple <key , value , effect>                       		     // needs to be matched. If no operator is specified , it defaults to "Equal" ,
				// so perform the same check in this case as well
				if toleration.Operator == api.TolerationOpEqual || toleration.Operator == "" {
					if toleration.Key == currTaint.Key &&
						toleration.Value == currTaint.Value &&
						toleration.Effect == currTaint.Effect {
						continue taintLoop
					}
				} else if toleration.Operator == api.TolerationOpExists {
					// If the toleration operator is "Exists" , then we need to match
					// only the key and the effect. The value can be ignored.
					if toleration.Key == currTaint.Key && toleration.Effect == currTaint.Effect {
						continue taintLoop
					}
				}
			}
			// We reached here which indicates that this taint couldn't match
			// the toleration on the pod. This means that this pod cannot tolerate ALL of the taints.
			// So we need to return a false value here , indicating that this node isn't a fit for this pod.
			return false
		} else if currTaint.Effect == api.TaintEffectPreferNoSchedule {
			continue taintLoop
		}
	}
	// All of the node's taints are now processed and we found that all of the taints are tolerated by the pod
	// So this node is a probable candidate for scheduling the node
	return true
}

func PodFitsHost(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
	if len(pod.Spec.NodeName) == 0 {
		return true, nil
	}
	return pod.Spec.NodeName == node, nil
}

type NodeLabelChecker struct {
	info     NodeInfo
	labels   []string
	presence bool
}

func NewNodeLabelPredicate(info NodeInfo, labels []string, presence bool) algorithm.FitPredicate {
	labelChecker := &NodeLabelChecker{
		info:     info,
		labels:   labels,
		presence: presence,
	}
	return labelChecker.CheckNodeLabelPresence
}

// CheckNodeLabelPresence checks whether all of the specified labels exists on a node or not, regardless of their value
// If "presence" is false, then returns false if any of the requested labels matches any of the node's labels,
// otherwise returns true.
// If "presence" is true, then returns false if any of the requested labels does not match any of the node's labels,
// otherwise returns true.
//
// Consider the cases where the nodes are placed in regions/zones/racks and these are identified by labels
// In some cases, it is required that only nodes that are part of ANY of the defined regions/zones/racks be selected
//
// Alternately, eliminating nodes that have a certain label, regardless of value, is also useful
// A node may have a label with "retiring" as key and the date as the value
// and it may be desirable to avoid scheduling new pods on this node
func (n *NodeLabelChecker) CheckNodeLabelPresence(pod *api.Pod, existingPods []*api.Pod, nodeID string) (bool, error) {
	var exists bool
	node, err := n.info.GetNodeInfo(nodeID)
	if err != nil {
		return false, err
	}
	nodeLabels := labels.Set(node.Labels)
	for _, label := range n.labels {
		exists = nodeLabels.Has(label)
		if (exists && !n.presence) || (!exists && n.presence) {
			return false, nil
		}
	}
	return true, nil
}

type ServiceAffinity struct {
	podLister     algorithm.PodLister
	serviceLister algorithm.ServiceLister
	nodeInfo      NodeInfo
	labels        []string
}

func NewServiceAffinityPredicate(podLister algorithm.PodLister, serviceLister algorithm.ServiceLister, nodeInfo NodeInfo, labels []string) algorithm.FitPredicate {
	affinity := &ServiceAffinity{
		podLister:     podLister,
		serviceLister: serviceLister,
		nodeInfo:      nodeInfo,
		labels:        labels,
	}
	return affinity.CheckServiceAffinity
}

// CheckServiceAffinity ensures that only the nodes that match the specified labels are considered for scheduling.
// The set of labels to be considered are provided to the struct (ServiceAffinity).
// The pod is checked for the labels and any missing labels are then checked in the node
// that hosts the service pods (peers) for the given pod.
//
// We add an implicit selector requiring some particular value V for label L to a pod, if:
// - L is listed in the ServiceAffinity object that is passed into the function
// - the pod does not have any NodeSelector for L
// - some other pod from the same service is already scheduled onto a node that has value V for label L
func (s *ServiceAffinity) CheckServiceAffinity(pod *api.Pod, existingPods []*api.Pod, nodeID string) (bool, error) {
	var affinitySelector labels.Selector

	// check if the pod being scheduled has the affinity labels specified in its NodeSelector
	affinityLabels := map[string]string{}
	nodeSelector := labels.Set(pod.Spec.NodeSelector)
	labelsExist := true
	for _, l := range s.labels {
		if nodeSelector.Has(l) {
			affinityLabels[l] = nodeSelector.Get(l)
		} else {
			// the current pod does not specify all the labels, look in the existing service pods
			labelsExist = false
		}
	}

	// skip looking at other pods in the service if the current pod defines all the required affinity labels
	if !labelsExist {
		services, err := s.serviceLister.GetPodServices(pod)
		if err == nil {
			// just use the first service and get the other pods within the service
			// TODO: a separate predicate can be created that tries to handle all services for the pod
			selector := labels.SelectorFromSet(services[0].Spec.Selector)
			servicePods, err := s.podLister.List(selector)
			if err != nil {
				return false, err
			}
			// consider only the pods that belong to the same namespace
			nsServicePods := []*api.Pod{}
			for _, nsPod := range servicePods {
				if nsPod.Namespace == pod.Namespace {
					nsServicePods = append(nsServicePods, nsPod)
				}
			}
			if len(nsServicePods) > 0 {
				// consider any service pod and fetch the node its hosted on
				otherNode, err := s.nodeInfo.GetNodeInfo(nsServicePods[0].Spec.NodeName)
				if err != nil {
					return false, err
				}
				for _, l := range s.labels {
					// If the pod being scheduled has the label value specified, do not override it
					if _, exists := affinityLabels[l]; exists {
						continue
					}
					if labels.Set(otherNode.Labels).Has(l) {
						affinityLabels[l] = labels.Set(otherNode.Labels).Get(l)
					}
				}
			}
		}
	}

	// if there are no existing pods in the service, consider all nodes
	if len(affinityLabels) == 0 {
		affinitySelector = labels.Everything()
	} else {
		affinitySelector = labels.Set(affinityLabels).AsSelector()
	}

	node, err := s.nodeInfo.GetNodeInfo(nodeID)
	if err != nil {
		return false, err
	}

	// check if the node matches the selector
	return affinitySelector.Matches(labels.Set(node.Labels)), nil
}

func PodFitsHostPorts(pod *api.Pod, existingPods []*api.Pod, node string) (bool, error) {
	existingPorts := getUsedPorts(existingPods...)
	wantPorts := getUsedPorts(pod)
	for wport := range wantPorts {
		if wport == 0 {
			continue
		}
		if existingPorts[wport] {
			return false, nil
		}
	}
	return true, nil
}

func getUsedPorts(pods ...*api.Pod) map[int]bool {
	ports := make(map[int]bool)
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, podPort := range container.Ports {
				ports[podPort.HostPort] = true
			}
		}
	}
	return ports
}

// MapPodsToMachines obtains a list of pods and pivots that list into a map where the keys are host names
// and the values are the list of pods running on that host.
func MapPodsToMachines(lister algorithm.PodLister) (map[string][]*api.Pod, error) {
	machineToPods := map[string][]*api.Pod{}
	// TODO: perform more targeted query...
	pods, err := lister.List(labels.Everything())
	if err != nil {
		return map[string][]*api.Pod{}, err
	}
	for _, scheduledPod := range pods {
		host := scheduledPod.Spec.NodeName
		machineToPods[host] = append(machineToPods[host], scheduledPod)
	}
	return machineToPods, nil
}

// search two arrays and return true if they have at least one common element; return false otherwise
func haveSame(a1, a2 []string) bool {
	for _, val1 := range a1 {
		for _, val2 := range a2 {
			if val1 == val2 {
				return true
			}
		}
	}
	return false
}
