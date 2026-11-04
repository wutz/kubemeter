/*
Copyright 2026 Taizeng Wu.

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
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceMeterSpec defines the desired state of ResourceMeter
type ResourceMeterSpec struct {
	// Selector defines which resources to meter
	// +required
	Selector ResourceSelector `json:"selector"`

	// Resources defines which resource types to meter (cpu, memory, gpu, storage)
	// +required
	// +kubebuilder:validation:MinItems=1
	Resources []ResourceMetric `json:"resources"`

	// CollectionInterval defines how often to collect usage data
	// +optional
	// +kubebuilder:default="1m"
	CollectionInterval *metav1.Duration `json:"collectionInterval,omitempty"`

	// RetentionPeriod defines how long to keep usage data
	// +optional
	// +kubebuilder:default="168h"
	RetentionPeriod *metav1.Duration `json:"retentionPeriod,omitempty"`
}

// ResourceSelector defines which resources to meter
type ResourceSelector struct {
	// Namespaces to include (empty means all namespaces)
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// LabelSelector to filter resources
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// ResourceTypes to meter (Pod, PersistentVolumeClaim, etc.)
	// +optional
	ResourceTypes []string `json:"resourceTypes,omitempty"`
}

// ResourceMetric defines a resource type to meter
type ResourceMetric struct {
	// Type of resource (cpu, memory, gpu, storage)
	// +required
	// +kubebuilder:validation:Enum=cpu;memory;gpu;storage
	Type ResourceType `json:"type"`

	// Unit for billing (e.g., core-hour, GiB-hour, gpu-hour, GiB)
	// +required
	Unit string `json:"unit"`
}

// ResourceType defines the type of resource to meter
// +kubebuilder:validation:Enum=cpu;memory;gpu;storage
type ResourceType string

const (
	ResourceTypeCPU     ResourceType = "cpu"
	ResourceTypeMemory  ResourceType = "memory"
	ResourceTypeGPU     ResourceType = "gpu"
	ResourceTypeStorage ResourceType = "storage"
)

// ResourceMeterStatus defines the observed state of ResourceMeter.
type ResourceMeterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ResourceMeter resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ResourceMeter is the Schema for the resourcemeters API
type ResourceMeter struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ResourceMeter
	// +required
	Spec ResourceMeterSpec `json:"spec"`

	// status defines the observed state of ResourceMeter
	// +optional
	Status ResourceMeterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ResourceMeterList contains a list of ResourceMeter
type ResourceMeterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ResourceMeter `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ResourceMeter{}, &ResourceMeterList{})
		return nil
	})
}
