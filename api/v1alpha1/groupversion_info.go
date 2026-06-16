// Package v1alpha1 contains the API types for kargo-plugin-ext. Its
// CustomPromotionStep type binds a named promotion step to a HashiCorp
// go-plugin RPC plugin that the controller invokes to compute the step's
// output.
//
// +kubebuilder:object:generate=true
// +groupName=plugin.kargo.akuity.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// GroupVersion is the group/version for the kargo-plugin-ext API.
var GroupVersion = schema.GroupVersion{
	Group:   "plugin.kargo.akuity.io",
	Version: "v1alpha1",
}

// SchemeBuilder collects the types in this API group for registration with a
// runtime.Scheme.
var SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

// AddToScheme registers the types in this API group with a runtime.Scheme.
var AddToScheme = SchemeBuilder.AddToScheme

func init() {
	SchemeBuilder.Register(
		&CustomPromotionStep{},
		&CustomPromotionStepList{},
	)
}
