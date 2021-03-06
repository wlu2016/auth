// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	ktesting "k8s.io/client-go/testing"
)

type fakeCa struct{}

func (ca fakeCa) Generate(name string) (key, cert []byte) {
	key = []byte("fake key")
	cert = []byte("fake cert")
	return
}

func createSecret(name string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"key":  []byte("fake key"),
			"cert": []byte("fake cert"),
		},
	}
}

func createServiceAccount(name, namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

type updatedSas struct {
	curSa *v1.ServiceAccount
	oldSa *v1.ServiceAccount
}

func TestSecretController(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Resource: "secrets",
		Version:  "v1",
	}
	testCases := map[string]struct {
		saToAdd         *v1.ServiceAccount
		saToDelete      *v1.ServiceAccount
		sasToUpdate     *updatedSas
		expectedActions []ktesting.Action
	}{
		"adding service account creates new secret": {
			saToAdd: createServiceAccount("test", "test-ns"),
			expectedActions: []ktesting.Action{
				ktesting.NewUpdateAction(gvr, "test-ns", createSecret("istio.test")),
			},
		},
		"removing service account deletes existing secret": {
			saToDelete: createServiceAccount("deleted", "deleted-ns"),
			expectedActions: []ktesting.Action{
				ktesting.NewDeleteAction(gvr, "deleted-ns", "istio.deleted"),
			},
		},
		"updating service accounts does nothing if name and namespace are not changed": {
			sasToUpdate: &updatedSas{
				curSa: createServiceAccount("name", "ns"),
				oldSa: createServiceAccount("name", "ns"),
			},
			expectedActions: []ktesting.Action{},
		},
		"updating service accounts deletes old secret and creates a new one": {
			sasToUpdate: &updatedSas{
				curSa: createServiceAccount("new-name", "new-ns"),
				oldSa: createServiceAccount("old-name", "old-ns"),
			},
			expectedActions: []ktesting.Action{
				ktesting.NewDeleteAction(gvr, "old-ns", "istio.old-name"),
				ktesting.NewUpdateAction(gvr, "new-ns", createSecret("istio.new-name")),
			},
		},
	}

	for k, tc := range testCases {
		client := fake.NewSimpleClientset()
		controller := NewSecretController(fakeCa{}, client.CoreV1())

		if tc.saToAdd != nil {
			controller.addFunc(tc.saToAdd)
		}
		if tc.saToDelete != nil {
			controller.deleteFunc(tc.saToDelete)
		}
		if tc.sasToUpdate != nil {
			controller.updateFunc(tc.sasToUpdate.oldSa, tc.sasToUpdate.curSa)
		}

		actions := client.Actions()
		if !reflect.DeepEqual(actions, tc.expectedActions) {
			t.Errorf("%s: expect actions to be \n\t%v\n but actual actions are \n\t%v", k, tc.expectedActions, actions)
		}
	}
}
