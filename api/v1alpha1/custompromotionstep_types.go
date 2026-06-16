package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomPromotionStepSpec binds a named promotion step to a go-plugin RPC
// plugin. When a Promotion step references this resource by name (via "uses"),
// the controller launches the named plugin, passes it the step's evaluated
// config (and, optionally, the promotion's working directory), and uses the
// plugin's response as the step output.
type CustomPromotionStepSpec struct {
	// Plugin is the name of the go-plugin executable to invoke. The controller
	// resolves it within its configured plugin directory. Defaults to the
	// resource's own name when omitted.
	//
	// +optional
	Plugin string `json:"plugin,omitempty"`
	// SharePromotionFolder, when true, passes the promotion's working directory
	// to the plugin so it can read and write the same files as built-in steps
	// (e.g. a git checkout). Because the plugin runs as a subprocess on the
	// controller host, it shares the controller's filesystem.
	//
	// +optional
	SharePromotionFolder bool `json:"sharePromotionFolder,omitempty"`
	// DefaultTimeout is the default soft maximum interval for which the step may
	// remain Running. It can be overridden by step-level configuration.
	//
	// +optional
	DefaultTimeout *metav1.Duration `json:"defaultTimeout,omitempty"`
	// DefaultErrorThreshold is the number of consecutive failures tolerated
	// before the Promotion is failed. It can be overridden by step-level
	// configuration.
	//
	// +optional
	DefaultErrorThreshold uint32 `json:"defaultErrorThreshold,omitempty"`
}

// CustomPromotionStepStatus describes the observed state of a
// CustomPromotionStep.
type CustomPromotionStepStatus struct {
	// ObservedGeneration is the most recent generation observed by the
	// controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions describe the current state of the CustomPromotionStep.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// CustomPromotionStep binds a promotion step name to a go-plugin RPC plugin.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={cps}
// +kubebuilder:printcolumn:name="Plugin",type=string,JSONPath=`.spec.plugin`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type CustomPromotionStep struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec binds the step to a plugin.
	Spec CustomPromotionStepSpec `json:"spec"`
	// Status describes the observed state of the CustomPromotionStep.
	//
	// +optional
	Status CustomPromotionStepStatus `json:"status,omitempty"`
}

// CustomPromotionStepList is a list of CustomPromotionStep resources.
//
// +kubebuilder:object:root=true
type CustomPromotionStepList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomPromotionStep `json:"items"`
}
