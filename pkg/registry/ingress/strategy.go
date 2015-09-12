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

package ingress

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/experimental"
	"k8s.io/kubernetes/pkg/apis/experimental/validation"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
)

// ingressStrategy implements verification logic for Ingresses.
type ingressStrategy struct {
	runtime.ObjectTyper
	api.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication Ingress objects.
var Strategy = ingressStrategy{api.Scheme, api.SimpleNameGenerator}

func (ingressStrategy) NamespaceScoped() bool {
	return true
}

func (ingressStrategy) PrepareForCreate(obj runtime.Object) {
	ingress := obj.(*experimental.Ingress)
	ingress.Generation = 1
	ingress.Status = experimental.IngressStatus{}
}

func (ingressStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newIngress := obj.(*experimental.Ingress)
	oldIngress := old.(*experimental.Ingress)
	//Copy since Ingress has a status sub-resource
	newIngress.Status = oldIngress.Status

	// Any changes to the spec increment the generation number, any changes to the
	// status should reflect the generation number of the corresponding object. We push
	// the burden of managing the status onto the clients because we can't (in general)
	// know here what version of spec the writer of the status has seen. It may seem like
	// we can at first -- since obj contains spec -- but in the future we will probably make
	// status its own object, and even if we don't, writes may be the result of a
	// read-update-write loop, so the contents of spec may not actually be the spec that
	// the controller has *seen*.
	//
	// TODO: Any changes to a part of the object that represents desired state (labels,
	// annotations etc) should also increment the generation.
	if !reflect.DeepEqual(oldIngress.Spec, newIngress.Spec) {
		newIngress.Generation = oldIngress.Generation + 1
	}
}

func (ingressStrategy) Validate(ctx api.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	ingress := obj.(*experimental.Ingress)
	return validation.ValidateIngress(ingress)
}

func (ingressStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (ingressStrategy) ValidateUpdate(ctx api.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	newIngress := obj.(*experimental.Ingress)
	oldIngress := old.(*experimental.Ingress)
	validationErrorList := validation.ValidateIngress(newIngress)
	updateErrorList := validation.ValidateIngressUpdate(oldIngress, newIngress)
	return append(validationErrorList, updateErrorList...)
}

func (ingressStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// IngressToSelectableFields returns a label set that represents the object.
func IngressToSelectableFields(ingress *experimental.Ingress) fields.Set {
	return fields.Set{
		"metadata.name": ingress.Name,
	}
}

// MatchIngress is the filter used by the generic etcd backend to ingress
// watch events from etcd to clients of the apiserver only interested in specific
// labels/fields.
func MatchIngress(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			ingress, ok := obj.(*experimental.Ingress)
			if !ok {
				return nil, nil, fmt.Errorf("Given object is not a replication controller.")
			}
			return labels.Set(ingress.ObjectMeta.Labels), IngressToSelectableFields(ingress), nil
		},
	}
}
