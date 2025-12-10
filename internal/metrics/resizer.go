package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metrics subsystem and all of the keys used by the resizer.
const (
	ResizerSuccessResizeTotalKey = "success_resize_total"
	ResizerFailedResizeTotalKey  = "failed_resize_total"
	ResizerLoopSecondsTotalKey   = "loop_seconds_total"
	ResizerLimitReachedTotalKey  = "limit_reached_total"
	CrPatchTotalKey              = "cr_patch_total"
	CrPatchErrorsTotalKey        = "cr_patch_errors_total"
)

func init() {
	registerResizerMetrics()
}

type resizerSuccessResizeTotalAdapter struct {
	metric prometheus.CounterVec
}

func (a *resizerSuccessResizeTotalAdapter) Increment(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Inc()
}

// SpecifyLabels helps output metrics before the first resize event.
// This method specifies the metric labels and add 0 to the metric value.
func (a *resizerSuccessResizeTotalAdapter) SpecifyLabels(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Add(0)
}

type resizerFailedResizeTotalAdapter struct {
	metric prometheus.CounterVec
}

func (a *resizerFailedResizeTotalAdapter) Increment(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Inc()
}

// SpecifyLabels helps output metrics before the first fail event of resize.
// This method specifies the metric labels and add 0 to the metric value.
func (a *resizerFailedResizeTotalAdapter) SpecifyLabels(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Add(0)
}

type resizerLoopSecondsTotalAdapter struct {
	metric prometheus.Counter
}

func (a *resizerLoopSecondsTotalAdapter) Add(value float64) {
	a.metric.Add(value)
}

type resizerLimitReachedTotalAdapter struct {
	metric prometheus.CounterVec
}

func (a *resizerLimitReachedTotalAdapter) Increment(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Inc()
}

// SpecifyLabels helps output metrics before the first limit reached event of resize.
// This method specifies the metric labels and add 0 to the metric value.
func (a *resizerLimitReachedTotalAdapter) SpecifyLabels(pvcname string, pvcns string) {
	a.metric.With(prometheus.Labels{"persistentvolumeclaim": pvcname, "namespace": pvcns}).Add(0)
}

type crPatchTotalAdapter struct {
	metric prometheus.CounterVec
}

func (a *crPatchTotalAdapter) Increment(namespace, crKind, status string) {
	a.metric.With(prometheus.Labels{"namespace": namespace, "cr_kind": crKind, "status": status}).Inc()
}

type crPatchErrorsTotalAdapter struct {
	metric prometheus.CounterVec
}

func (a *crPatchErrorsTotalAdapter) Increment(namespace, crKind, errorType string) {
	a.metric.With(prometheus.Labels{"namespace": namespace, "cr_kind": crKind, "error_type": errorType}).Inc()
}

var (
	resizerSuccessResizeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      ResizerSuccessResizeTotalKey,
		Help:      "counter that indicates how many volume expansion processing resized succeed.",
	}, []string{"persistentvolumeclaim", "namespace"})

	resizerFailedResizeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      ResizerFailedResizeTotalKey,
		Help:      "counter that indicates how many volume expansion processing resizes fail.",
	}, []string{"persistentvolumeclaim", "namespace"})

	resizerLoopSecondsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      ResizerLoopSecondsTotalKey,
		Help:      "counter that indicates the sum of seconds spent on volume expansion processing loops.",
	})

	resizerLimitReachedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      ResizerLimitReachedTotalKey,
		Help:      "counter that indicates how many storage limits were reached.",
	}, []string{"persistentvolumeclaim", "namespace"})

	crPatchTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      CrPatchTotalKey,
		Help:      "counter of CR patches (success/failure).",
	}, []string{"namespace", "cr_kind", "status"})

	crPatchErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      CrPatchErrorsTotalKey,
		Help:      "counter by error type.",
	}, []string{"namespace", "cr_kind", "error_type"})

	ResizerSuccessResizeTotal *resizerSuccessResizeTotalAdapter = &resizerSuccessResizeTotalAdapter{
		metric: *resizerSuccessResizeTotal,
	}
	ResizerFailedResizeTotal *resizerFailedResizeTotalAdapter = &resizerFailedResizeTotalAdapter{
		metric: *resizerFailedResizeTotal,
	}
	ResizerLoopSecondsTotal *resizerLoopSecondsTotalAdapter = &resizerLoopSecondsTotalAdapter{
		metric: resizerLoopSecondsTotal,
	}
	ResizerLimitReachedTotal *resizerLimitReachedTotalAdapter = &resizerLimitReachedTotalAdapter{
		metric: *resizerLimitReachedTotal,
	}
	CrPatchTotal *crPatchTotalAdapter = &crPatchTotalAdapter{
		metric: *crPatchTotal,
	}
	CrPatchErrorsTotal *crPatchErrorsTotalAdapter = &crPatchErrorsTotalAdapter{
		metric: *crPatchErrorsTotal,
	}
)

func registerResizerMetrics() {
	runtimemetrics.Registry.MustRegister(resizerSuccessResizeTotal)
	runtimemetrics.Registry.MustRegister(resizerFailedResizeTotal)
	runtimemetrics.Registry.MustRegister(resizerLoopSecondsTotal)
	runtimemetrics.Registry.MustRegister(resizerLimitReachedTotal)
	runtimemetrics.Registry.MustRegister(crPatchTotal)
	runtimemetrics.Registry.MustRegister(crPatchErrorsTotal)
}
