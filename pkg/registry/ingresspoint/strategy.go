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

package ingresspoint

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

// ingressPointStrategy implements verification logic for Replication IngressPoints.
type ingressPointStrategy struct {
	runtime.ObjectTyper
	api.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication IngressPoint objects.
var Strategy = ingressPointStrategy{api.Scheme, api.SimpleNameGenerator}

func (ingressPointStrategy) NamespaceScoped() bool {
	return true
}

func (ingressPointStrategy) PrepareForCreate(obj runtime.Object) {
	ingressPoint := obj.(*experimental.IngressPoint)
	ingressPoint.Generation = 1
	ingressPoint.Status = experimental.IngressPointStatus{}
}

func (ingressPointStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newIngressPoint := obj.(*experimental.IngressPoint)
	oldIngressPoint := old.(*experimental.IngressPoint)
	//Copy since IngressPoint has a status sub-resource
	newIngressPoint.Status = oldIngressPoint.Status

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
	if !reflect.DeepEqual(oldIngressPoint.Spec, newIngressPoint.Spec) {
		newIngressPoint.Generation = oldIngressPoint.Generation + 1
	}
}

func (ingressPointStrategy) Validate(ctx api.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	ingressPoint := obj.(*experimental.IngressPoint)
	return validation.ValidateIngressPoint(ingressPoint)
}

func (ingressPointStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (ingressPointStrategy) ValidateUpdate(ctx api.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	newIngressPoint := obj.(*experimental.IngressPoint)
	oldIngressPoint := old.(*experimental.IngressPoint)
	validationErrorList := validation.ValidateIngressPoint(newIngressPoint)
	updateErrorList := validation.ValidateIngressPointUpdate(oldIngressPoint, newIngressPoint)
	return append(validationErrorList, updateErrorList...)
}

func (ingressPointStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// IngressPointToSelectableFields returns a label set that represents the object.
func IngressPointToSelectableFields(ingressPoint *experimental.IngressPoint) fields.Set {
	return fields.Set{
		"metadata.name": ingressPoint.Name,
	}
}

// MatchIngressPoint is the filter used by the generic etcd backend to ingressPoint
// watch events from etcd to clients of the apiserver only interested in specific
// labels/fields.
func MatchIngressPoint(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			ingressPoint, ok := obj.(*experimental.IngressPoint)
			if !ok {
				return nil, nil, fmt.Errorf("Given object is not a replication controller.")
			}
			return labels.Set(ingressPoint.ObjectMeta.Labels), IngressPointToSelectableFields(ingressPoint), nil
		},
	}
}
