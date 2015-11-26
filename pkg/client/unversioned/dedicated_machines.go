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

package unversioned

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

// DaemonsSetsNamespacer has methods to work with DedicatedMachine resources in a namespace
type DedicatedMachinesNamespacer interface {
	DedicatedMachines(namespace string) DedicatedMachineInterface
}

type DedicatedMachineInterface interface {
	List(label labels.Selector, field fields.Selector) (*extensions.DedicatedMachineList, error)
	Get(name string) (*extensions.DedicatedMachine, error)
	Create(ctrl *extensions.DedicatedMachine) (*extensions.DedicatedMachine, error)
	Update(ctrl *extensions.DedicatedMachine) (*extensions.DedicatedMachine, error)
	UpdateStatus(ctrl *extensions.DedicatedMachine) (*extensions.DedicatedMachine, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, opts api.ListOptions) (watch.Interface, error)
}

// dedicatedMachines implements DaemonsSetsNamespacer interface
type dedicatedMachines struct {
	r  *ExtensionsClient
	ns string
}

func newDedicatedMachines(c *ExtensionsClient, namespace string) *dedicatedMachines {
	return &dedicatedMachines{c, namespace}
}

// Ensure statically that dedicatedMachines implements DedicatedMachinesInterface.
var _ DedicatedMachineInterface = &dedicatedMachines{}

func (c *dedicatedMachines) List(label labels.Selector, field fields.Selector) (result *extensions.DedicatedMachineList, err error) {
	result = &extensions.DedicatedMachineList{}
	err = c.r.Get().Namespace(c.ns).Resource("dedicatedmachines").LabelsSelectorParam(label).FieldsSelectorParam(field).Do().Into(result)
	return
}

// Get returns information about a particular daemon set.
func (c *dedicatedMachines) Get(name string) (result *extensions.DedicatedMachine, err error) {
	result = &extensions.DedicatedMachine{}
	err = c.r.Get().Namespace(c.ns).Resource("dedicatedmachines").Name(name).Do().Into(result)
	return
}

// Create creates a new daemon set.
func (c *dedicatedMachines) Create(daemon *extensions.DedicatedMachine) (result *extensions.DedicatedMachine, err error) {
	result = &extensions.DedicatedMachine{}
	err = c.r.Post().Namespace(c.ns).Resource("dedicatedmachines").Body(daemon).Do().Into(result)
	return
}

// Update updates an existing daemon set.
func (c *dedicatedMachines) Update(daemon *extensions.DedicatedMachine) (result *extensions.DedicatedMachine, err error) {
	result = &extensions.DedicatedMachine{}
	err = c.r.Put().Namespace(c.ns).Resource("dedicatedmachines").Name(daemon.Name).Body(daemon).Do().Into(result)
	return
}

// UpdateStatus updates an existing daemon set status
func (c *dedicatedMachines) UpdateStatus(daemon *extensions.DedicatedMachine) (result *extensions.DedicatedMachine, err error) {
	result = &extensions.DedicatedMachine{}
	err = c.r.Put().Namespace(c.ns).Resource("dedicatedmachines").Name(daemon.Name).SubResource("status").Body(daemon).Do().Into(result)
	return
}

// Delete deletes an existing daemon set.
func (c *dedicatedMachines) Delete(name string) error {
	return c.r.Delete().Namespace(c.ns).Resource("dedicatedmachines").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested daemon sets.
func (c *dedicatedMachines) Watch(label labels.Selector, field fields.Selector, opts api.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("dedicatedmachines").
		Param("resourceVersion", opts.ResourceVersion).
		TimeoutSeconds(TimeoutFromListOptions(opts)).
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Watch()
}
