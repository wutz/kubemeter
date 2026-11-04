# KubeMeter - AI Agent Guide

## Project Overview

KubeMeter is a Kubernetes resource metering operator that bills based on **requested resources** (not actual usage):
- **Compute**: CPU, Memory, GPU
- **Storage**: PersistentVolumeClaims

## Project Structure

```
api/v1alpha1/
  resourcemeter_types.go       # ResourceMeter CRD (metering configuration)
  resourceusage_types.go        # ResourceUsage CRD (metering results)
internal/controller/
  resourcemeter_controller.go   # Main metering logic
  resourceusage_controller.go   # Usage record lifecycle
config/
  crd/bases/                    # Generated CRDs (DO NOT EDIT)
  rbac/role.yaml                # Generated RBAC (DO NOT EDIT)
  samples/                      # Example configurations (EDIT THESE)
```

## Critical Rules

### Never Edit Auto-Generated Files
- `config/crd/bases/*.yaml` - from `make manifests`
- `config/rbac/role.yaml` - from `make manifests`
- `api/v1alpha1/zz_generated.*.go` - from `make generate`

### Always Run After Editing Types
```bash
make manifests  # After editing *_types.go or markers
make generate   # After editing *_types.go
```

## Key Design Principles

### 1. Billing Based on Requests, Not Actual Usage

**What we meter:**
- Pod: `spec.containers[].resources.requests` (CPU, Memory, GPU)
- PVC: `spec.resources.requests.storage`

**What we DON'T meter:**
- Actual CPU/memory usage (that's Prometheus's job)
- Metrics from metrics-server

**Why:**
- Simple, deterministic, fair
- No dependency on metrics infrastructure
- Standard cloud billing practice

### 2. Multi-Level Parent References

ResourceUsage includes `spec.parentRefs[]` for hierarchical billing:

```yaml
spec:
  resourceRef:
    name: training-job-worker-1
    kind: Pod
  parentRefs:
    - name: training-platform
      kind: worker
      labels:
        project: nlp-research
        team: ai-lab
```

Business systems can aggregate costs by:
- Project (`labels.project`)
- Team (`labels.team`)
- Parent resource (`parentRefs[].name`)

## Implementation Details

### ResourceMeter Controller

**Reconciliation logic:**
1. List Pods/PVCs matching `spec.selector`
2. For each resource:
   - Extract `requests` from spec
   - Calculate running hours
   - Create/update ResourceUsage
3. Set owner reference (auto-cleanup when meter is deleted)
4. Requeue after `spec.collectionInterval`

**RBAC requirements:**
```go
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=metering.wutz.dev,resources=resourceusages,verbs=get;list;watch;create;update;patch;delete
```

### Parent Reference Extraction

Automatically extracts from common label patterns:
- `app.kubernetes.io/name` + `app.kubernetes.io/component`
- `app` + `component`
- `owner` + `owner-kind`

All labels are preserved in `resourceRef.labels` for flexible business-side grouping.

## Common Tasks

### Add New Resource Type

1. Add to `ResourceType` enum in `resourcemeter_types.go`:
```go
const (
    ResourceTypeCPU     ResourceType = "cpu"
    ResourceTypeMemory  ResourceType = "memory"
    ResourceTypeGPU     ResourceType = "gpu"
    ResourceTypeStorage ResourceType = "storage"
    ResourceTypeNewType ResourceType = "newtype"  // Add here
)
```

2. Update validation marker:
```go
// +kubebuilder:validation:Enum=cpu;memory;gpu;storage;newtype
```

3. Implement calculation logic in controller

4. Run `make manifests generate`

### Add Cost Calculation

Extend `ResourceUsageStatus` with pricing:
```go
type ResourceUsageStatus struct {
    Allocations ResourceAllocations `json:"allocations,omitempty"`
    RunningHours string `json:"runningHours,omitempty"`
    Cost string `json:"cost,omitempty"`  // Already exists
    CostBreakdown CostBreakdown `json:"costBreakdown,omitempty"`  // Add details
}

type CostBreakdown struct {
    CPUCost     string `json:"cpuCost,omitempty"`
    MemoryCost  string `json:"memoryCost,omitempty"`
    GPUCost     string `json:"gpuCost,omitempty"`
    StorageCost string `json:"storageCost,omitempty"`
}
```

Implement pricing in controller (e.g., read from ConfigMap).

### Watch Additional Resources

To meter other resources (Jobs, StatefulSets):

1. Add RBAC:
```go
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
```

2. Add watch in `SetupWithManager`:
```go
Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(r.mapJobToMeter))
```

3. Implement reconciliation logic

## Testing

```bash
# Unit tests
make test

# Manual testing
make install                    # Install CRDs
make run                        # Run locally
kubectl apply -f config/samples/  # Create test resources
kubectl get resourceusages      # Check results
```

## Deployment

```bash
# Build and push image
export IMG=ghcr.io/wutz/kubemeter:v0.1.0
make docker-build docker-push IMG=$IMG

# Deploy to cluster
make deploy IMG=$IMG

# Or use Helm
helm install kubemeter ./dist/chart/
```

## Monitoring Integration

KubeMeter focuses on **billing**, Prometheus focuses on **monitoring**:

**Workflow:**
1. KubeMeter calculates bills from `requests`
2. Prometheus shows actual usage
3. Users compare and optimize their `requests`

**Export metrics to Prometheus (optional):**
```go
// Add to controller
import "github.com/prometheus/client_golang/prometheus"

var (
    totalCost = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "kubemeter_resource_cost_total",
            Help: "Total cost by resource",
        },
        []string{"namespace", "name", "kind"},
    )
)
```

## Common Patterns

### Time Range Handling

```go
// Start time
if pod.Status.StartTime != nil {
    startTime = *pod.Status.StartTime
} else {
    startTime = pod.CreationTimestamp
}

// End time (for terminated resources)
if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
    usage.Spec.TimeRange.EndTime = &metav1.Time{Time: time.Now()}
}
```

### Idempotent Reconciliation

```go
// Always safe to call multiple times
err := r.Get(ctx, types.NamespacedName{...}, usage)
if errors.IsNotFound(err) {
    // Create
    usage = &ResourceUsage{...}
    return r.Create(ctx, usage)
} else {
    // Update
    usage.Status = calculateStatus(...)
    return r.Status().Update(ctx, usage)
}
```

### Finalizer for Cleanup

If metering external resources (cloud storage, databases):

```go
const finalizerName = "metering.wutz.dev/finalizer"

// Add finalizer
if !controllerutil.ContainsFinalizer(meter, finalizerName) {
    controllerutil.AddFinalizer(meter, finalizerName)
    return r.Update(ctx, meter)
}

// Handle deletion
if !meter.DeletionTimestamp.IsZero() {
    // Cleanup external resources
    if err := r.cleanupExternalResources(ctx, meter); err != nil {
        return ctrl.Result{}, err
    }
    // Remove finalizer
    controllerutil.RemoveFinalizer(meter, finalizerName)
    return r.Update(ctx, meter)
}
```

## References

- **Kubebuilder Book**: https://book.kubebuilder.io
- **API Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- **Operator Pattern**: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

## Contact

- Author: Taizeng Wu
- Repository: https://github.com/wutz/kubemeter
