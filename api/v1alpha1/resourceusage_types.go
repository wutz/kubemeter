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

// ResourceUsageSpec defines the desired state of ResourceUsage
type ResourceUsageSpec struct {
	// MeterRef references the ResourceMeter that created this usage record
	// +required
	MeterRef string `json:"meterRef"`

	// ResourceRef identifies the metered resource
	// +required
	ResourceRef ResourceReference `json:"resourceRef"`

	// ParentRefs contains hierarchical parent resource information for billing aggregation
	// +optional
	ParentRefs []ResourceReference `json:"parentRefs,omitempty"`

	// TimeRange specifies the time period for this usage record
	// +required
	TimeRange TimeRange `json:"timeRange"`
}

// ResourceReference identifies a Kubernetes resource
type ResourceReference struct {
	// Namespace of the resource
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the resource
	// +required
	Name string `json:"name"`

	// Kind of the resource (Pod, PersistentVolumeClaim, etc.)
	// +required
	Kind string `json:"kind"`

	// UID of the resource
	// +optional
	UID string `json:"uid,omitempty"`

	// Labels of the resource for business grouping
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// TimeRange defines a time period
type TimeRange struct {
	// StartTime is when the resource started
	// +required
	StartTime metav1.Time `json:"startTime"`

	// EndTime is when the resource ended (empty if still running)
	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`
}

// ResourceUsageStatus defines the observed state of ResourceUsage.
type ResourceUsageStatus struct {
	// Allocations contains the resource requests (not actual usage)
	// +optional
	Allocations ResourceAllocations `json:"allocations,omitempty"`

	// RunningHours is the total running time in hours (as a decimal string)
	// +optional
	RunningHours string `json:"runningHours,omitempty"`

	// Cost is the calculated billing amount (if pricing is configured)
	// +optional
	Cost string `json:"cost,omitempty"`

	// LastUpdateTime is when this usage was last calculated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// conditions represent the current state of the ResourceUsage resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceAllocations contains resource requests from Pod/PVC specs
type ResourceAllocations struct {
	// CPU cores requested
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory bytes requested
	// +optional
	Memory string `json:"memory,omitempty"`

	// GPU count requested
	// +optional
	GPU int32 `json:"gpu,omitempty"`

	// Storage bytes requested (from PVC)
	// +optional
	Storage string `json:"storage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ResourceUsage is the Schema for the resourceusages API
type ResourceUsage struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ResourceUsage
	// +required
	Spec ResourceUsageSpec `json:"spec"`

	// status defines the observed state of ResourceUsage
	// +optional
	Status ResourceUsageStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ResourceUsageList contains a list of ResourceUsage
type ResourceUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ResourceUsage `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ResourceUsage{}, &ResourceUsageList{})
		return nil
	})
}
