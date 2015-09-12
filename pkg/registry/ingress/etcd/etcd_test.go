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

package etcd

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/experimental"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/util"
)

func newStorage(t *testing.T) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "experimental")
	return NewREST(etcdStorage), fakeClient
}

func validNewIngress() *experimental.Ingress {
	return &experimental.Ingress{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: experimental.IngressSpec{
			Rules: []experimental.IngressRule{
				{
					Host: "bar",
					Paths: []experimental.IngressPath{
						{
							Path: "/images",
							Backend: experimental.IngressBackend{
								ServiceRef:  api.LocalObjectReference{Name: "foo"},
								ServicePort: util.IntOrString{IntVal: 8080, Kind: util.IntstrInt},
								Protocol:    api.ProtocolTCP,
							},
						},
					},
				},
			},
		},
	}
}

var validIngress = *validNewIngress()

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	Ingress := validNewIngress()
	Ingress.ObjectMeta = api.ObjectMeta{}
	test.TestCreate(
		// valid
		Ingress,
		// invalid (invalid name)
		&experimental.Ingress{
			Spec: experimental.IngressSpec{
				Rules: []experimental.IngressRule{
					{
						Host: "bar",
						Paths: []experimental.IngressPath{
							{
								Path: "/images",
								Backend: experimental.IngressBackend{
									ServiceRef:  api.LocalObjectReference{Name: "foo"},
									ServicePort: util.IntOrString{IntVal: 8080, Kind: util.IntstrInt},
									Protocol:    api.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	)
}

func TestUpdate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestUpdate(
		// valid
		validNewIngress(),
		// updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.Ingress)
			object.Spec.Rules[0].Host = "blabla"
			return object
		},
		// invalid updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.Ingress)
			object.UID = "newUID"
			return object
		},
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.Ingress)
			object.Name = ""
			return object
		},
	)
}

func TestDelete(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestDelete(validNewIngress())
}

func TestGet(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestGet(validNewIngress())
}

func TestList(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestList(validNewIngress())
}

func TestWatch(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestWatch(
		validNewIngress(),
		// matching labels
		[]labels.Set{},
		// not matching labels
		[]labels.Set{
			{"a": "c"},
			{"foo": "bar"},
		},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
			{"name": "foo"},
		},
	)
}
