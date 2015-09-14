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
)

func newStorage(t *testing.T) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "experimental")
	return NewREST(etcdStorage), fakeClient
}

func validNewIngressPoint() *experimental.IngressPoint {
	return &experimental.IngressPoint{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: experimental.IngressPointSpec{
			Host: "bar",
			PathList: []experimental.PathRef{
				{
					Domain: "foo.example.com",
					Path:   "/images",
					Service: experimental.ServiceRef{
						Name:      "fooservice",
						Namespace: "default",
						Port:      8080,
					},
				},
			},
		},
	}
}

var validIngressPoint = *validNewIngressPoint()

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	IngressPoint := validNewIngressPoint()
	IngressPoint.ObjectMeta = api.ObjectMeta{}
	test.TestCreate(
		// valid
		IngressPoint,
		// Invalid IngressPoint.Spec.PathList.Path
		&experimental.IngressPoint{
			Spec: experimental.IngressPointSpec{
				Host: "bar",
				PathList: []experimental.PathRef{
					{
						Domain: "foo.example.com",
						Path:   "images",
						Service: experimental.ServiceRef{
							Name:      "fooservice",
							Namespace: "default",
							Port:      8080,
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
		validNewIngressPoint(),
		// updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.IngressPoint)
			object.Spec.Host = "blabla"
			return object
		},
		// invalid updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.IngressPoint)
			object.UID = "newUID"
			return object
		},
		func(obj runtime.Object) runtime.Object {
			object := obj.(*experimental.IngressPoint)
			object.Name = ""
			return object
		},
	)
}

func TestDelete(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestDelete(validNewIngressPoint())
}

func TestGet(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestGet(validNewIngressPoint())
}

func TestList(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestList(validNewIngressPoint())
}

func TestWatch(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestWatch(
		validNewIngressPoint(),
		// matching labels
		[]labels.Set{},
		// not matching labels
		[]labels.Set{
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
