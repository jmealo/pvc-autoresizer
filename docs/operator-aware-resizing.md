# Operator-Aware Resizing

## Overview

Operator-aware resizing enables pvc-autoresizer to work harmoniously with Kubernetes operators that manage PersistentVolumeClaims. Instead of directly modifying a PVC's storage request (which operators may treat as configuration drift), pvc-autoresizer can patch a Custom Resource field, allowing the operator to reconcile the PVC size.

## Problem Statement

When pvc-autoresizer directly patches a PVC managed by an operator:

1. **Configuration Drift**: The operator sees the PVC size differs from its spec and may treat it as an unintended change
2. **Reconciliation Failures**: Some operators (e.g., RabbitMQ) may fail to reconcile or enter error states
3. **Reversion**: The operator might revert the resize or attempt to "fix" what it perceives as drift
4. **Complex Workarounds**: Operators may require pod restarts or other manual intervention

## Solution

pvc-autoresizer can patch a field in the operator's Custom Resource instead of the PVC directly. The operator then reconciles the PVC size based on its own logic, maintaining the desired workflow.

## Security Model

Operator-aware resizing implements **defense-in-depth** security with two independent layers:

### Layer 1: RBAC (Helm-Generated)

The pvc-autoresizer controller must be granted explicit RBAC permissions to patch each Custom Resource type. RBAC rules are automatically generated from your Helm values.

**Configuration in values.yaml:**
```yaml
operatorAwareResizing:
  enabled: true
  allowedResources:
    - apiGroup: "rabbitmq.com"
      kind: "RabbitmqCluster"
      resource: "rabbitmqclusters"
```

**Helm automatically generates:**
```yaml
- apiGroups: ["rabbitmq.com"]
  resources: ["rabbitmqclusters"]
  verbs: ["get", "list", "patch"]
```

### Layer 2: JSONPath Validation (Code Enforced)

For additional security, the controller enforces that JSON paths can only target fields under `/spec/`. Paths targeting `/metadata`, `/status`, or other sensitive fields are rejected with a clear error message.

**Valid paths:**
- `.spec.storage.size` ✅
- `/spec/persistence/storage` ✅
- `.spec.resources.requests.storage` ✅

**Invalid paths (rejected):**
- `.metadata.annotations.foo` ❌
- `/status/conditions` ❌
- `.metadata.labels.app` ❌

### Why Two Layers Are Sufficient

This approach is **simpler and more secure** than having a separate ConfigMap allowlist:

1. **Single source of truth**: values.yaml controls both RBAC and configuration
2. **No configuration drift**: RBAC always matches what you configured
3. **Standard pattern**: Helm-generated RBAC is a common Kubernetes practice
4. **Defense-in-depth**: RBAC controls CR access, JSONPath prevents field tampering

### Prerequisites

Before using operator-aware resizing, you must:

1. ✅ Enable the feature in values.yaml: `operatorAwareResizing.enabled: true`
2. ✅ List allowed CR types with correct resource names in `allowedResources`
3. ✅ Run `helm upgrade` to apply the configuration
4. ✅ Ensure PVC annotations use valid JSON paths targeting `/spec/*` only

**If the feature is disabled or no resources are listed, operator-aware resizing is completely disabled for security.**

## How It Works

```
┌──────────────────────────────────────────────────────────────┐
│ 1. Metrics indicate low disk space on PVC                   │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. pvc-autoresizer checks for CR target annotations         │
└──────────────────────────────┬───────────────────────────────┘
                               │
                ┌──────────────┴──────────────┐
                │                             │
                ▼                             ▼
┌───────────────────────────┐   ┌───────────────────────────┐
│ Annotations present?      │   │ No annotations            │
│ Patch CR field            │   │ Patch PVC directly        │
│ (operator-aware mode)     │   │ (standard mode)           │
└─────────────┬─────────────┘   └───────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. Operator detects CR change and reconciles                │
└──────────────────────────────────────────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. Operator updates PVC spec.resources.requests.storage     │
└──────────────────────────────────────────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. Kubernetes resizes the underlying volume                 │
└──────────────────────────────────────────────────────────────┘
```

## Configuration

### Annotations

Add these annotations to your PVC to enable operator-aware resizing:

| Annotation | Required | Description | Example |
|-----------|----------|-------------|---------|
| `resize.topolvm.io/target-resource-api-version` | Yes | API version of the target CR | `rabbitmq.com/v1beta1` |
| `resize.topolvm.io/target-resource-kind` | Yes | Kind of the target CR | `RabbitmqCluster` |
| `resize.topolvm.io/target-resource-name` | Yes | Name of the target CR instance | `my-rabbitmq` |
| `resize.topolvm.io/target-resource-json-path` | Yes | JSON path to the storage field in the CR | `.spec.persistence.storage` |
| `resize.topolvm.io/target-resource-namespace` | No | Namespace of the target CR (defaults to PVC namespace) | `rabbitmq-system` |

**Note**: All existing autoresizer annotations (`storage_limit`, `threshold`, `increase`, etc.) continue to work as normal.

### JSON Path Format

The `target-resource-json-path` annotation accepts two formats:

- **Dot notation** (user-friendly): `.spec.persistence.storage`
- **JSON Pointer** (RFC 6901): `/spec/persistence/storage`

Both formats are automatically normalized internally to JSON Pointer format.

## Examples

### RabbitMQ Operator

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-rabbitmq-0
  namespace: rabbitmq
  annotations:
    # Standard autoresizer annotations
    resize.topolvm.io/storage_limit: "100Gi"
    resize.topolvm.io/threshold: "20%"
    resize.topolvm.io/increase: "10Gi"

    # Operator-aware resizing annotations
    resize.topolvm.io/target-resource-api-version: "rabbitmq.com/v1beta1"
    resize.topolvm.io/target-resource-kind: "RabbitmqCluster"
    resize.topolvm.io/target-resource-name: "my-rabbitmq"
    resize.topolvm.io/target-resource-json-path: ".spec.persistence.storage"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
  storageClassName: topolvm-provisioner
```

### CloudNativePG (CNPG)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pg-cluster-1
  namespace: postgresql
  annotations:
    resize.topolvm.io/storage_limit: "500Gi"
    resize.topolvm.io/threshold: "15%"
    resize.topolvm.io/increase: "20Gi"

    resize.topolvm.io/target-resource-api-version: "postgresql.cnpg.io/v1"
    resize.topolvm.io/target-resource-kind: "Cluster"
    resize.topolvm.io/target-resource-name: "my-cluster"
    resize.topolvm.io/target-resource-json-path: ".spec.storage.size"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 50Gi
  storageClassName: topolvm-provisioner
```

### DragonflyDB Operator

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: dragonfly-data-0
  namespace: dragonfly
  annotations:
    resize.topolvm.io/storage_limit: "200Gi"
    resize.topolvm.io/threshold: "25%"
    resize.topolvm.io/increase: "15Gi"

    resize.topolvm.io/target-resource-api-version: "dragonflydb.io/v1alpha1"
    resize.topolvm.io/target-resource-kind: "Dragonfly"
    resize.topolvm.io/target-resource-name: "my-dragonfly"
    resize.topolvm.io/target-resource-json-path: ".spec.resources.requests.storage"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 25Gi
  storageClassName: topolvm-provisioner
```

### Cross-Namespace Example

If your CR is in a different namespace than the PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  namespace: app-namespace
  annotations:
    resize.topolvm.io/storage_limit: "100Gi"
    resize.topolvm.io/threshold: "20%"

    resize.topolvm.io/target-resource-api-version: "myoperator.io/v1"
    resize.topolvm.io/target-resource-kind: "Database"
    resize.topolvm.io/target-resource-name: "prod-db"
    resize.topolvm.io/target-resource-namespace: "database-namespace"  # Different namespace
    resize.topolvm.io/target-resource-json-path: ".spec.storage.size"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
  storageClassName: topolvm-provisioner
```

## RBAC Configuration

### Helm Chart (Recommended)

When deploying via Helm, RBAC rules are automatically generated from your values.yaml configuration. This is the **recommended approach** as it ensures your RBAC always matches your intended configuration.

**Step 1: Configure values.yaml**

```yaml
operatorAwareResizing:
  enabled: true
  allowedResources:
    # RabbitMQ Operator
    - apiGroup: "rabbitmq.com"
      kind: "RabbitmqCluster"
      resource: "rabbitmqclusters"

    # CloudNativePG
    - apiGroup: "postgresql.cnpg.io"
      kind: "Cluster"
      resource: "clusters"

    # DragonflyDB
    - apiGroup: "dragonflydb.io"
      kind: "Dragonfly"
      resource: "dragonflies"
```

**Step 2: Deploy or upgrade**

```bash
helm upgrade --install pvc-autoresizer pvc-autoresizer/pvc-autoresizer \
  -n pvc-autoresizer \
  -f values.yaml
```

**Step 3: Verify RBAC was generated**

```bash
kubectl get clusterrole pvc-autoresizer-controller -o yaml
```

You should see RBAC rules for each resource you configured.

### Manual RBAC (Non-Helm Deployments)

If not using Helm, you must manually create RBAC rules for each Custom Resource type:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pvc-autoresizer-operator-aware
rules:
# RabbitMQ Operator
- apiGroups: ["rabbitmq.com"]
  resources: ["rabbitmqclusters"]
  verbs: ["get", "list", "patch"]

# CloudNativePG
- apiGroups: ["postgresql.cnpg.io"]
  resources: ["clusters"]
  verbs: ["get", "list", "patch"]

# DragonflyDB
- apiGroups: ["dragonflydb.io"]
  resources: ["dragonflies"]
  verbs: ["get", "list", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pvc-autoresizer-operator-aware
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: pvc-autoresizer-operator-aware
subjects:
- kind: ServiceAccount
  name: pvc-autoresizer-controller
  namespace: pvc-autoresizer  # Adjust to your deployment namespace
```

### Security Best Practices

1. **Feature disabled by default**: `operatorAwareResizing.enabled: false` prevents any CR patching
2. **Explicit allowlist**: Only CRs in `allowedResources` receive RBAC permissions
3. **No wildcard RBAC**: The controller has no broad permissions to patch arbitrary resources
4. **JSONPath validation**: Even with RBAC, only `/spec/*` fields can be modified

## Monitoring

### Metrics

Two new Prometheus metrics are available for operator-aware resizing:

#### `pvc_autoresizer_cr_patch_success_total`

Counter for successful CR patch operations.

**Labels:**
- `persistentvolumeclaim`: PVC name
- `namespace`: PVC namespace
- `target_kind`: CR kind being patched
- `target_namespace`: CR namespace

**Example:**
```
pvc_autoresizer_cr_patch_success_total{
  persistentvolumeclaim="data-rabbitmq-0",
  namespace="rabbitmq",
  target_kind="RabbitmqCluster",
  target_namespace="rabbitmq"
} 5
```

#### `pvc_autoresizer_cr_patch_failed_total`

Counter for failed CR patch operations.

**Labels:** (same as success metric)

**Example:**
```
pvc_autoresizer_cr_patch_failed_total{
  persistentvolumeclaim="data-rabbitmq-0",
  namespace="rabbitmq",
  target_kind="RabbitmqCluster",
  target_namespace="rabbitmq"
} 1
```

### Events

pvc-autoresizer emits Kubernetes events on the PVC for CR operations:

#### Normal Events

- **`ResizedCR`**: Successfully patched the target CR
  ```
  CR RabbitmqCluster/rabbitmq/my-rabbitmq field /spec/persistence/storage resized to 30Gi
  ```

#### Warning Events

- **`ResizeCRInvalidConfig`**: Invalid CR target annotations
  ```
  Invalid CR target configuration: incomplete CR target configuration: missing required annotations: json-path
  ```

- **`ResizeCRFailed`**: Failed to patch the target CR
  ```
  Failed to resize CR RabbitmqCluster/rabbitmq/my-rabbitmq: target CR not found
  ```

### Viewing Events

```bash
# View events for a specific PVC
kubectl describe pvc data-rabbitmq-0 -n rabbitmq

# Watch events in real-time
kubectl get events -n rabbitmq --watch
```

## Troubleshooting

### Error: "Target CR not found"

**Symptom**: Event shows `Failed to resize CR: target CR not found`

**Causes:**
- The CR doesn't exist
- Wrong CR name in annotation
- Wrong namespace in annotation

**Resolution:**
```bash
# Verify the CR exists
kubectl get <kind> <name> -n <namespace>

# Example for RabbitMQ
kubectl get rabbitmqcluster my-rabbitmq -n rabbitmq

# Check annotation values
kubectl get pvc data-rabbitmq-0 -n rabbitmq -o yaml | grep target-resource
```

### Error: "Insufficient permissions to patch CR"

**Symptom**: Event shows `insufficient permissions to patch CR RabbitmqCluster/...`

**Cause**: The CR type is not in values.yaml, so RBAC was not generated for it

**Resolution:**

1. Add the CR type to your Helm values:
```yaml
operatorAwareResizing:
  enabled: true
  allowedResources:
    - apiGroup: "rabbitmq.com"
      kind: "RabbitmqCluster"
      resource: "rabbitmqclusters"
```

2. Upgrade the Helm release:
```bash
helm upgrade pvc-autoresizer pvc-autoresizer/pvc-autoresizer -n pvc-autoresizer -f values.yaml
```

3. Verify RBAC was generated:
```bash
kubectl get clusterrole pvc-autoresizer-controller -o yaml | grep rabbitmq
```

4. (Optional) Check permissions directly:
```bash
kubectl auth can-i patch rabbitmqclusters.rabbitmq.com \
  --as=system:serviceaccount:pvc-autoresizer:pvc-autoresizer-controller \
  -n rabbitmq
# Should output: yes
```

### Error: "Invalid JSON path: for security reasons..."

**Symptom**: Event shows `invalid JSON path ".metadata.annotations.foo": for security reasons, only paths starting with /spec/ are allowed`

**Cause**: The JSON path targets a forbidden field (metadata, status, etc.)

**Resolution:**

JSON paths must target fields under `/spec/` only. Update your PVC annotation:

**Invalid paths:**
- `.metadata.annotations.storage` ❌ (security violation)
- `.status.capacity` ❌ (security violation)
- `/spec` ❌ (must target a specific field)

**Valid paths:**
- `.spec.persistence.storage` ✅
- `.spec.storage.size` ✅
- `/spec/resources/requests/storage` ✅

Common paths for popular operators:
- RabbitMQ: `.spec.persistence.storage`
- CNPG: `.spec.storage.size`
- Strimzi Kafka: `.spec.kafka.storage.size`

### Error: "Failed to set field at path"

**Symptom**: Event shows `failed to set field at path "/spec/persistence/storage"`

**Causes:**
- JSON path doesn't exist in the CR
- Field name is misspelled
- Path structure doesn't match CR schema

**Resolution:**

1. Verify the CR structure:
```bash
kubectl get rabbitmqcluster my-rabbitmq -n rabbitmq -o yaml
```

2. Check if the path exists:
```bash
# For path .spec.persistence.storage
kubectl get rabbitmqcluster my-rabbitmq -n rabbitmq -o jsonpath='{.spec.persistence.storage}'
```

3. Compare your path with the CR's actual schema

### Error: "Conflict while patching CR"

**Symptom**: Logs show `conflict while patching CR (will retry)`

**Causes:**
- Concurrent modification of the CR
- Operator reconciled the CR at the same time

**Resolution:**
- This is normal and expected
- pvc-autoresizer will automatically retry on the next interval
- If persistent, check if multiple PVCs are targeting the same CR field

### PVC Not Resizing

**Symptom**: Disk space is low but PVC size doesn't change

**Checklist:**

1. Verify all annotations are present:
```bash
kubectl get pvc data-rabbitmq-0 -n rabbitmq -o jsonpath='{.metadata.annotations}' | jq
```

2. Check pvc-autoresizer logs:
```bash
kubectl logs -n topolvm-system deployment/pvc-autoresizer -f | grep "data-rabbitmq-0"
```

3. Verify the StorageClass has the enabled annotation:
```bash
kubectl get storageclass topolvm-provisioner -o jsonpath='{.metadata.annotations.resize\.topolvm\.io/enabled}'
# Should output: true
```

4. Check if storage limit is reached:
```bash
kubectl get pvc data-rabbitmq-0 -n rabbitmq -o jsonpath='{.spec.resources.requests.storage}'
kubectl get pvc data-rabbitmq-0 -n rabbitmq -o jsonpath='{.metadata.annotations.resize\.topolvm\.io/storage_limit}'
```

5. Verify metrics are being collected:
```bash
# Check if pvc-autoresizer can see volume stats
kubectl logs -n topolvm-system deployment/pvc-autoresizer | grep "available"
```

## Limitations and Considerations

### Field Type Requirements

The target field in the CR must accept Kubernetes `Quantity` format strings (e.g., `"30Gi"`, `"500Mi"`). Most storage-related fields use this format.

### Operator Reconciliation Timing

There may be a delay between when pvc-autoresizer patches the CR and when the operator reconciles the PVC. This is operator-dependent.

### Multiple PVCs Targeting Same CR Field

If multiple PVCs target the same CR field:
- Each PVC will independently calculate its resize
- Last write wins
- The operator will reconcile based on the CR's final value
- This scenario is generally not recommended

### No Validation of CR Schema

pvc-autoresizer does not validate that the JSON path exists or accepts the value type. The Kubernetes API server performs validation when applying the patch.

### PVC Annotation Updates

Even in operator-aware mode, pvc-autoresizer still updates the `resize.topolvm.io/pre_capacity_bytes` annotation on the PVC for tracking purposes. This does not modify the PVC spec.

## Migration from Direct PVC Patching

To migrate existing PVCs from direct patching to operator-aware resizing:

1. Add the CR target annotations to the PVC
2. pvc-autoresizer will automatically use operator-aware mode on the next resize
3. No restart or reconfiguration of pvc-autoresizer is required

**Note**: Existing resizing behavior is unchanged for PVCs without CR target annotations.

## Best Practices

1. **Test in Non-Production First**: Verify the operator reconciles correctly with CR patches before production use

2. **Monitor Metrics**: Set up alerts on `pvc_autoresizer_cr_patch_failed_total` to catch configuration issues early

3. **Use Helm for RBAC**: Let Helm auto-generate RBAC from values.yaml to prevent configuration drift

4. **Explicit Resource List**: Only add CR types to `allowedResources` that you actually need to resize

5. **Document JSON Paths**: Maintain documentation of JSON paths for each operator you use

6. **Version Compatibility**: Verify JSON paths when upgrading operators, as CR schemas may change

7. **Validate Annotations**: Use admission webhooks or policy engines to validate annotation correctness

8. **Restrict to /spec/***: Never attempt to patch metadata or status fields - the controller enforces this

## Supported Operators

This feature has been tested with:
- RabbitMQ Operator (rabbitmq.com/v1beta1)
- CloudNativePG (postgresql.cnpg.io/v1)
- Strimzi Kafka Operator (kafka.strimzi.io/v1beta2)

It should work with any operator that:
- Uses a CR to define storage size
- Reconciles PVC specs based on CR changes
- Accepts Kubernetes Quantity format for storage fields

## Additional Resources

- [Kubernetes Quantity Format](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/)
- [JSON Pointer (RFC 6901)](https://datatracker.ietf.org/doc/html/rfc6901)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
