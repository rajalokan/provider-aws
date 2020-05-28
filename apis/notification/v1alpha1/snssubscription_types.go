package v1alpha1

import (
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// SNSSubscription contains subscription
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.atProvider.Id"
// +kubebuilder:printcolumn:name="RRs",type="integer",JSONPath=".status.atProvider.ResourceRecordSetCount"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type SNSSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SNSSubscriptionSpec   `json:"spec"`
	Status SNSSubscriptionStatus `json:"status,omitempty"`
}

// SNSSubscriptionStatus is the status of AWS SNS Topic
type SNSSubscriptionStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	AtProvider                     SNSSubscriptionObservation `json:"atProvider,omitempty"`
}

// SNSSubscriptionSpec defined the desired state of a AWS SNS Topic
type SNSSubscriptionSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	ForProvider                  SNSSubscriptionParameters `json:"forProvider"`
}

// SNSSubscriptionParameters define the desired state of a AWS SNS Topic
type SNSSubscriptionParameters struct {

	// TopicArn is the Arn of the SNS Topic
	TopicArn string `json:"topicArn,omitempty"`

	// TopicArnRef references a SNS Topic and retrieves its TopicArn
	TopicArnRef *runtimev1alpha1.Reference `json:"topicArnRef,omitempty"`

	// TopicArnSelector selects a reference to a SNS Topic and retrieves
	// its TopicArn
	TopicArnSelector *runtimev1alpha1.Selector `json:"topicArnSelector,omitempty"`

	// The subscription's protocol.
	Protocol *string `json:"protocol"`

	Endpoint *string `json:"endpoint"`
}

// SNSSubscriptionObservation represents the observed state of a AWS SNS Topic
type SNSSubscriptionObservation struct {
	Arn *string `json:"arn"`
}

// +kubebuilder:object:root=true

//SNSSubscriptionList contains a list of SNSTopic
type SNSSubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SNSSubscription `json:"items"`
}
