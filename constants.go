package pvcautoresizer

// AutoResizeEnabledKey is the key of flag that enables pvc-autoresizer.
const AutoResizeEnabledKey = "resize.topolvm.io/enabled"

// ResizeThresholdAnnotation is the key of resize threshold.
const ResizeThresholdAnnotation = "resize.topolvm.io/threshold"

// ResizeInodesThresholdAnnotation is the key of resize threshold for inodes.
const ResizeInodesThresholdAnnotation = "resize.topolvm.io/inodes-threshold"

// ResizeIncreaseAnnotation is the key of amount increased.
const ResizeIncreaseAnnotation = "resize.topolvm.io/increase"

// StorageLimitAnnotation is the key of storage limit value
const StorageLimitAnnotation = "resize.topolvm.io/storage_limit"

// PreviousCapacityBytesAnnotation is the key of previous volume capacity.
const PreviousCapacityBytesAnnotation = "resize.topolvm.io/pre_capacity_bytes"

// InitialResizeGroupByAnnotation is the key of the initial-resize group by.
const InitialResizeGroupByAnnotation = "resize.topolvm.io/initial-resize-group-by"

// DefaultThreshold is the default value of ResizeThresholdAnnotation.
const DefaultThreshold = "10%"

// DefaultInodesThreshold is the default value of ResizeInodesThresholdAnnotation.
const DefaultInodesThreshold = "10%"

// DefaultIncrease is the default value of ResizeIncreaseAnnotation.
const DefaultIncrease = "10%"

// ResizeTargetKindAnnotation is the key of target resource kind.
const ResizeTargetKindAnnotation = "resize.pvc.io/target-kind"

// ResizeTargetGroupAnnotation is the key of target resource group.
const ResizeTargetGroupAnnotation = "resize.pvc.io/target-group"

// ResizeTargetVersionAnnotation is the key of target resource version.
const ResizeTargetVersionAnnotation = "resize.pvc.io/target-version"

// ResizeTargetNameAnnotation is the key of target resource name.
const ResizeTargetNameAnnotation = "resize.pvc.io/target-name"

// ResizeTargetPathAnnotation is the key of target resource path.
const ResizeTargetPathAnnotation = "resize.pvc.io/target-path"
