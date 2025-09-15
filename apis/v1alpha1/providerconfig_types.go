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
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// A ProviderConfigSpec defines the desired state of a ProviderConfig.
type ProviderConfigSpec struct {
	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`

	// GrpcServerConfig defines the GRPC server connection configuration.
	GrpcServerConfig GrpcServerConfig `json:"grpcServerConfig"`

	// ProtobufConfig defines the protobuf file configuration.
	ProtobufConfig ProtobufConfig `json:"protobufConfig"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=None;Secret;InjectedIdentity;Environment;Filesystem
	Source xpv1.CredentialsSource `json:"source"`

	xpv1.CommonCredentialSelectors `json:",inline"`
}

// GrpcServerConfig defines GRPC server connection details.
type GrpcServerConfig struct {
	// Host is the GRPC server hostname or IP address.
	Host string `json:"host"`

	// Port is the GRPC server port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// TLS configuration for the GRPC connection.
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// Timeout for GRPC calls in seconds.
	// +optional
	// +kubebuilder:default=30
	Timeout *int32 `json:"timeout,omitempty"`
}

// TLSConfig defines TLS configuration for GRPC connections.
type TLSConfig struct {
	// Enabled indicates whether TLS should be used.
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// InsecureSkipVerify skips TLS certificate verification.
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

	// ServerName for TLS verification.
	// +optional
	ServerName *string `json:"serverName,omitempty"`
}

// ProtobufConfig defines protobuf file configuration.
type ProtobufConfig struct {
	// Source of the protobuf file.
	// +kubebuilder:validation:Enum=Secret;ConfigMap;Inline
	Source ProtobufSource `json:"source"`

	// SecretRef references a secret containing the protobuf file.
	// Required when source is Secret.
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`

	// ConfigMapRef references a configmap containing the protobuf file.
	// Required when source is ConfigMap.
	// +optional
	ConfigMapRef *ConfigMapKeySelector `json:"configMapRef,omitempty"`

	// Inline protobuf content.
	// Required when source is Inline.
	// +optional
	Inline *string `json:"inline,omitempty"`
}

// ProtobufSource represents the source of protobuf configuration.
type ProtobufSource string

const (
	// ProtobufSourceSecret indicates protobuf is stored in a Secret.
	ProtobufSourceSecret ProtobufSource = "Secret"

	// ProtobufSourceConfigMap indicates protobuf is stored in a ConfigMap.
	ProtobufSourceConfigMap ProtobufSource = "ConfigMap"

	// ProtobufSourceInline indicates protobuf is provided inline.
	ProtobufSourceInline ProtobufSource = "Inline"
)

// SecretKeySelector selects a key from a Secret.
type SecretKeySelector struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Key to select from the Secret.
	Key string `json:"key"`

	// Namespace of the Secret.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// ConfigMapKeySelector selects a key from a ConfigMap.
type ConfigMapKeySelector struct {
	// Name of the ConfigMap.
	Name string `json:"name"`

	// Key to select from the ConfigMap.
	Key string `json:"key"`

	// Namespace of the ConfigMap.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// A ProviderConfigStatus reflects the observed state of a ProviderConfig.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures a Template provider.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderConfig type metadata.
var (
	ProviderConfigKind             = reflect.TypeOf(ProviderConfig{}).Name()
	ProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigKind}.String()
	ProviderConfigKindAPIVersion   = ProviderConfigKind + "." + SchemeGroupVersion.String()
	ProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigKind)
)

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
