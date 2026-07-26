package controllers

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"
)

func TestParameterStoreController(t *testing.T) {
	// A Memcached object with metadata and spec.
	parameterStore := &v1alpha1.ParameterStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "memcached",
			Namespace: "memcached-operator",
			Labels: map[string]string{
				"label-key": "label-value",
			},
		},
	}
	// Objects to track in the fake client.
	parameterStoreList := &v1alpha1.ParameterStoreList{}

	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeBuilder.GroupVersion, parameterStore, parameterStoreList)

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(parameterStore).Build()

	// List Memcached objects filtering by labels
	opt := client.MatchingLabels(map[string]string{"label-key": "label-value"})
	err := cl.List(context.TODO(), parameterStoreList, opt)
	if err != nil {
		t.Fatalf("list memcached: (%v)", err)
	}
	fmt.Printf("%+v", parameterStoreList)
}
