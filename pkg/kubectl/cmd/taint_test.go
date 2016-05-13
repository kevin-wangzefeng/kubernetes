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

package cmd

import (
	//"bytes"
	//"net/http"
	"reflect"
	//"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	//"k8s.io/kubernetes/pkg/api/testapi"
	//"k8s.io/kubernetes/pkg/client/restclient"
	//"k8s.io/kubernetes/pkg/client/unversioned/fake"
	//"k8s.io/kubernetes/pkg/runtime"
)

func TestParseTaints(t *testing.T) {
	tests := []struct {
		test           string
		taints         []string
		expected       []api.Taint
		expectedRemove []string
		expectErr      bool
	}{
		{
			test:   "only one valid taint input",
			taints: []string{"dedicated=bigData:NoSchedule"},
			expected: []api.Taint{{
				Key:    "dedicated",
				Value:  "bigData",
				Effect: api.TaintEffectNoSchedule,
			}},
			expectErr: false,
		},
		{
			test:   "two valid taint input, one is new, another is to remove the existing",
			taints: []string{"dedicated=bigData:NoSchedule", "foo-"},
			expected: []api.Taint{{
				Key:    "dedicated",
				Value:  "bigData",
				Effect: api.TaintEffectNoSchedule,
			}},
			expectedRemove: []string{"foo"},
			expectErr: false,
		},
		{
			test:   "invalid taint key",
			taints: []string{"Foo=bar:NoSchedule"},
			expected: []api.Taint{},
			expectErr: true,
		},
		{
			test:   "invalid taint effect",
			taints: []string{"foo=bar:NoExecute"},
			expected: []api.Taint{},
			expectErr: true,
		},
	}
	for _, test := range tests {
		taints, remove, err := parseTaints(test.taints)
		if test.expectErr && err == nil {
			t.Errorf("%s, expected error, got nothing", test.test)
		}
		if !test.expectErr && err != nil {
			t.Errorf("%s, unexpected error: %v", test.test, err)
		}
		if !reflect.DeepEqual(taints, test.expected) {
			t.Errorf("%s, expected: %v, got %v", test.test, test.expected, taints)
		}
		if !reflect.DeepEqual(remove, test.expectedRemove) {
			t.Errorf("%s, expected: %v, got %v", test.test, test.expectedRemove, remove)
		}
	}
}
