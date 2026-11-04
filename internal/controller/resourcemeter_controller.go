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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meteringv1alpha1 "github.com/wutz/kubemeter/api/v1alpha1"
)

// ResourceMeterReconciler reconciles a ResourceMeter object
type ResourceMeterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourcemeters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourcemeters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourcemeters/finalizers,verbs=update
// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourceusages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourceusages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceMeterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ResourceMeter instance
	meter := &meteringv1alpha1.ResourceMeter{}
	if err := r.Get(ctx, req.NamespacedName, meter); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ResourceMeter")
		return ctrl.Result{}, err
	}

	// Get collection interval
	interval := time.Minute
	if meter.Spec.CollectionInterval != nil {
		interval = meter.Spec.CollectionInterval.Duration
	}

	// List Pods matching the selector
	if err := r.reconcilePods(ctx, meter); err != nil {
		log.Error(err, "Failed to reconcile Pods")
		return ctrl.Result{}, err
	}

	// List PVCs matching the selector
	if err := r.reconcilePVCs(ctx, meter); err != nil {
		log.Error(err, "Failed to reconcile PVCs")
		return ctrl.Result{}, err
	}

	// Update status
	meta.SetStatusCondition(&meter.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciling",
		Message:            "Metering resources",
		ObservedGeneration: meter.Generation,
	})

	if err := r.Status().Update(ctx, meter); err != nil {
		log.Error(err, "Failed to update ResourceMeter status")
		return ctrl.Result{}, err
	}

	// Requeue after collection interval
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceMeterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meteringv1alpha1.ResourceMeter{}).
		Owns(&meteringv1alpha1.ResourceUsage{}).
		Named("resourcemeter").
		Complete(r)
}

// reconcilePods meters all Pods matching the selector
func (r *ResourceMeterReconciler) reconcilePods(ctx context.Context, meter *meteringv1alpha1.ResourceMeter) error {
	log := log.FromContext(ctx)

	// List Pods based on selector
	podList := &corev1.PodList{}
	listOpts := r.buildListOptions(meter.Spec.Selector)

	if err := r.List(ctx, podList, listOpts...); err != nil {
		return err
	}

	log.Info("Found Pods to meter", "count", len(podList.Items))

	for i := range podList.Items {
		pod := &podList.Items[i]

		// Skip pods that are not running
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		if err := r.createOrUpdateUsage(ctx, meter, pod, nil); err != nil {
			log.Error(err, "Failed to create or update usage for Pod", "pod", pod.Name)
			return err
		}
	}

	return nil
}

// reconcilePVCs meters all PVCs matching the selector
func (r *ResourceMeterReconciler) reconcilePVCs(ctx context.Context, meter *meteringv1alpha1.ResourceMeter) error {
	log := log.FromContext(ctx)

	pvcList := &corev1.PersistentVolumeClaimList{}
	listOpts := r.buildListOptions(meter.Spec.Selector)

	if err := r.List(ctx, pvcList, listOpts...); err != nil {
		return err
	}

	log.Info("Found PVCs to meter", "count", len(pvcList.Items))

	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]

		if err := r.createOrUpdateUsage(ctx, meter, nil, pvc); err != nil {
			log.Error(err, "Failed to create or update usage for PVC", "pvc", pvc.Name)
			return err
		}
	}

	return nil
}

// buildListOptions creates list options from ResourceSelector
func (r *ResourceMeterReconciler) buildListOptions(selector meteringv1alpha1.ResourceSelector) []client.ListOption {
	opts := []client.ListOption{}

	// Filter by namespaces
	if len(selector.Namespaces) > 0 {
		// For multiple namespaces, we need to list each one separately
		// For now, use the first namespace
		if len(selector.Namespaces) > 0 {
			opts = append(opts, client.InNamespace(selector.Namespaces[0]))
		}
	}

	// Filter by labels
	if selector.LabelSelector != nil {
		labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err == nil {
			opts = append(opts, client.MatchingLabelsSelector{Selector: labelSelector})
		}
	}

	return opts
}

// createOrUpdateUsage creates or updates a ResourceUsage for a Pod or PVC
func (r *ResourceMeterReconciler) createOrUpdateUsage(ctx context.Context, meter *meteringv1alpha1.ResourceMeter, pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim) error {
	log := log.FromContext(ctx)

	var (
		name      string
		namespace string
		uid       string
		kind      string
		labels    map[string]string
		startTime metav1.Time
	)

	if pod != nil {
		name = pod.Name
		namespace = pod.Namespace
		uid = string(pod.UID)
		kind = "Pod"
		labels = pod.Labels
		if pod.Status.StartTime != nil {
			startTime = *pod.Status.StartTime
		} else {
			startTime = pod.CreationTimestamp
		}
	} else if pvc != nil {
		name = pvc.Name
		namespace = pvc.Namespace
		uid = string(pvc.UID)
		kind = "PersistentVolumeClaim"
		labels = pvc.Labels
		startTime = pvc.CreationTimestamp
	} else {
		return fmt.Errorf("both pod and pvc are nil")
	}

	// Create ResourceUsage name
	usageName := fmt.Sprintf("%s-%s", meter.Name, name)

	usage := &meteringv1alpha1.ResourceUsage{}
	err := r.Get(ctx, types.NamespacedName{Name: usageName, Namespace: meter.Namespace}, usage)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	isNew := errors.IsNotFound(err)

	if isNew {
		usage = &meteringv1alpha1.ResourceUsage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      usageName,
				Namespace: meter.Namespace,
			},
			Spec: meteringv1alpha1.ResourceUsageSpec{
				MeterRef: meter.Name,
				ResourceRef: meteringv1alpha1.ResourceReference{
					Namespace: namespace,
					Name:      name,
					Kind:      kind,
					UID:       uid,
					Labels:    labels,
				},
				ParentRefs: r.extractParentRefs(labels),
				TimeRange: meteringv1alpha1.TimeRange{
					StartTime: startTime,
				},
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(meter, usage, r.Scheme); err != nil {
			return err
		}
	}

	// Calculate allocations
	allocations := meteringv1alpha1.ResourceAllocations{}

	if pod != nil {
		allocations = r.calculatePodAllocations(pod)
	} else if pvc != nil {
		allocations = r.calculatePVCAllocations(pvc)
	}

	// Calculate running hours
	now := time.Now()
	duration := now.Sub(startTime.Time)
	hours := duration.Hours()

	usage.Status.Allocations = allocations
	usage.Status.RunningHours = fmt.Sprintf("%.2f", hours)
	usage.Status.LastUpdateTime = &metav1.Time{Time: now}

	// Update conditions
	meta.SetStatusCondition(&usage.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Metering",
		Message: "Resource usage calculated",
	})

	if isNew {
		log.Info("Creating ResourceUsage", "name", usageName)
		if err := r.Create(ctx, usage); err != nil {
			return err
		}
	} else {
		log.Info("Updating ResourceUsage", "name", usageName)
		if err := r.Status().Update(ctx, usage); err != nil {
			return err
		}
	}

	return nil
}

// calculatePodAllocations extracts resource requests from a Pod
func (r *ResourceMeterReconciler) calculatePodAllocations(pod *corev1.Pod) meteringv1alpha1.ResourceAllocations {
	allocations := meteringv1alpha1.ResourceAllocations{}

	var totalCPU, totalMemory int64
	var totalGPU int32

	for _, container := range pod.Spec.Containers {
		if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			totalCPU += cpu.MilliValue()
		}
		if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			totalMemory += mem.Value()
		}
		if gpu, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
			totalGPU += int32(gpu.Value())
		}
	}

	if totalCPU > 0 {
		allocations.CPU = fmt.Sprintf("%dm", totalCPU)
	}
	if totalMemory > 0 {
		allocations.Memory = fmt.Sprintf("%d", totalMemory)
	}
	if totalGPU > 0 {
		allocations.GPU = totalGPU
	}

	return allocations
}

// calculatePVCAllocations extracts storage request from a PVC
func (r *ResourceMeterReconciler) calculatePVCAllocations(pvc *corev1.PersistentVolumeClaim) meteringv1alpha1.ResourceAllocations {
	allocations := meteringv1alpha1.ResourceAllocations{}

	if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		allocations.Storage = storage.String()
	}

	return allocations
}

// extractParentRefs extracts parent resource information from labels
func (r *ResourceMeterReconciler) extractParentRefs(labels map[string]string) []meteringv1alpha1.ResourceReference {
	refs := []meteringv1alpha1.ResourceReference{}

	// Common parent label patterns
	parentLabels := []struct {
		nameKey string
		kindKey string
	}{
		{"app.kubernetes.io/name", "app.kubernetes.io/component"},
		{"app", "component"},
		{"owner", "owner-kind"},
	}

	for _, pattern := range parentLabels {
		if name, ok := labels[pattern.nameKey]; ok {
			ref := meteringv1alpha1.ResourceReference{
				Name:   name,
				Labels: labels,
			}
			if kind, ok := labels[pattern.kindKey]; ok {
				ref.Kind = kind
			}
			refs = append(refs, ref)
			break
		}
	}

	return refs
}
