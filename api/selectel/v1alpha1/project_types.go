/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ProjectFinalizer blocks Project deletion while a CDN Resource still references it.
	ProjectFinalizer = "selectel.tapclap.com/project"

	// ConditionReady is true when the operator can issue a project-scoped IAM token.
	ConditionReady = "Ready"
)

// SecretReference points at a Secret in the operator namespace.
type SecretReference struct {
	// name of the Secret.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// namespace of the Secret. It must be the operator credentials namespace.
	// +kubebuilder:validation:MinLength=1
	// +required
	Namespace string `json:"namespace"`
}

// AllowedCertificate is a Selectel CDN certificate sequence the namespace may bind.
// The object shape leaves room for a future reference to a Certificate in the cluster.
type AllowedCertificate struct {
	// id is the certificate seq in the Selectel CDN API.
	// +kubebuilder:validation:Minimum=1
	// +required
	ID int32 `json:"id"`
}

// NamespaceAccess grants one namespace, or every namespace when name is "*",
// the right to use this Project and an explicit set of certificates and personal domains.
type NamespaceAccess struct {
	// name is a namespace name. "*" matches every namespace that has no own entry.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// certificates lists Selectel certificate seq values this namespace may attach.
	// An empty list allows CDN resources without a custom certificate.
	// +optional
	Certificates []AllowedCertificate `json:"certificates,omitempty"`

	// domains lists personal domains spec.names may use in this namespace.
	// An empty list allows no personal domain. A pattern is an exact name, a glob, or a /regex/.
	// +optional
	Domains []string `json:"domains,omitempty"`
}

// ProjectSpec defines how the operator authenticates to one Selectel project
// and which namespaces may use it.
type ProjectSpec struct {
	// authURL is the Keystone identity root. Defaults to https://cloud.api.selcloud.ru/identity/v3.
	// +optional
	AuthURL string `json:"authURL,omitempty"`

	// allowedNamespaces lists namespaces that may reference this Project.
	// An empty list allows none. A namespace entry replaces a "*" entry and does not inherit its certificates or domains.
	// +optional
	AllowedNamespaces []NamespaceAccess `json:"allowedNamespaces,omitempty"`

	// credentialsSecretRef is the Secret with accountId, username, password, and projectName.
	// +required
	CredentialsSecretRef SecretReference `json:"credentialsSecretRef"`
}

// ProjectStatus defines the observed state of Project.
type ProjectStatus struct {
	// conditions represent the current state of the Project resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project is a cluster-scoped Selectel project credential and access policy.
type Project struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Project
	// +required
	Spec ProjectSpec `json:"spec"`

	// status defines the observed state of Project
	// +optional
	Status ProjectStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}
