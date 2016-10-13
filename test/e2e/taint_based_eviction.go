/*
Copyright 2016 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Eviction based on taints [Serial] [Slow] [Destructive]", func() {
	var c *client.Client
	var nodeList *api.NodeList
	var ns string
	f := framework.NewDefaultFramework("pod-eviction")
	ignoreLabels := framework.ImagePullerLabels

	BeforeEach(func() {
		c = f.Client
		ns = f.Namespace.Name
		nodeList = &api.NodeList{}

		framework.WaitForAllNodesHealthy(c, time.Minute)
		masterNodes, nodeList = framework.GetMasterAndWorkerNodesOrDie(c)

		err := framework.CheckTestingNSDeletedExcept(c, ns)
		framework.ExpectNoError(err)
	})

	It("validates that pod evictec by nodecontroller when NoExectue taint added to node", func() {
		nodeName, podName := runAndKeepPodWithLabelAndGetNodeName(f)

		By("Trying to apply a NoExectue taint on the found node.")
		testTaint := api.Taint{
			Key:    fmt.Sprintf("kubernetes.io/e2e-taint-key-%s", string(uuid.NewUUID())),
			Value:  "testing-taint-value",
			Effect: api.TaintEffectNoExecute,
		}
		framework.AddOrUpdateTaintOnNode(c, nodeName, testTaint)
		framework.ExpectNodeHasTaint(c, nodeName, testTaint)
		defer framework.RemoveTaintOffNode(c, nodeName, testTaint)

		// Wait a bit to allow node controller monitor taints can evict pods
		// TODO: this is brittle; there's no guarantee the node controller will have run in 10 seconds.
		framework.Logf("Sleeping 15 seconds to wait for pod to be evicted")
		time.Sleep(15 * time.Second)

		_, err := f.PodClient().Get(podName)
		if !apierrs.IsNotFound(err) {
			ExpectNoError(err)
		}
	})
})
