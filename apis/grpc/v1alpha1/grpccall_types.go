/*
Copyright 2025 The Crossplane Authors.

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

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// GrpcCallParameters are the configurable fields of a GrpcCall.
type GrpcCallParameters struct {
	// ServiceName is the fully qualified service name (e.g., "helloworld.Greeter").
	ServiceName string `json:"serviceName"`

	// MethodName is the method name to call (e.g., "SayHello").
	MethodName string `json:"methodName"`

	// RequestData is the JSON representation of the request message.
	// +kubebuilder:pruning:PreserveUnknownFields
	RequestData runtime.RawExtension `json:"requestData"`

	// Headers to include in the GRPC call.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Timeout for this specific call in seconds. Overrides provider config timeout.
	// +optional
	Timeout *int32 `json:"timeout,omitempty"`
}

// GrpcCallObservation are the observable fields of a GrpcCall.
type GrpcCallObservation struct {
	// ResponseData is the JSON representation of the response message.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ResponseData *runtime.RawExtension `json:"responseData,omitempty"`

	// LastCallTime is the timestamp of the last successful call.
	// +optional
	LastCallTime *metav1.Time `json:"lastCallTime,omitempty"`

	// CallCount is the number of times this call has been executed.
	// +optional
	CallCount *int64 `json:"callCount,omitempty"`

	// Headers received in the response.
	// +optional
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
}

// A GrpcCallSpec defines the desired state of a GrpcCall.
type GrpcCallSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       GrpcCallParameters `json:"forProvider"`
}

// A GrpcCallStatus represents the observed state of a GrpcCall.
type GrpcCallStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          GrpcCallObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A GrpcCall represents a GRPC method call.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.forProvider.serviceName"
// +kubebuilder:printcolumn:name="METHOD",type="string",JSONPath=".spec.forProvider.methodName"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,grpc}
type GrpcCall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrpcCallSpec   `json:"spec"`
	Status GrpcCallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GrpcCallList contains a list of GrpcCall
type GrpcCallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrpcCall `json:"items"`
}

// GrpcCall type metadata.
var (
	GrpcCallKind             = reflect.TypeOf(GrpcCall{}).Name()
	GrpcCallGroupKind        = schema.GroupKind{Group: Group, Kind: GrpcCallKind}.String()
	GrpcCallKindAPIVersion   = GrpcCallKind + "." + SchemeGroupVersion.String()
	GrpcCallGroupVersionKind = SchemeGroupVersion.WithKind(GrpcCallKind)
)

func init() {
	SchemeBuilder.Register(&GrpcCall{}, &GrpcCallList{})
}
