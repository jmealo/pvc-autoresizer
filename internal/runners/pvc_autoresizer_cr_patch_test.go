package runners

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pvcautoresizer "github.com/topolvm/pvc-autoresizer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("test resizer with target resource patching", func() {
	ctx := context.Background()
	pvcNS := "default"

	It("should patch the target ConfigMap instead of resizing PVC directly", func() {
		pvcName := "test-cr-patch-pvc"
		cmName := "test-target-cm"

		// 1. Create the target ConfigMap
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: pvcNS,
			},
			Data: map[string]string{
				"storage": "10Gi",
			},
		}
		err := k8sClient.Create(ctx, cm)
		Expect(err).NotTo(HaveOccurred())

		// 2. Create the PVC with annotations
		increase := "10Gi"
		threshold := "50%"
		pvcSizeGi := int64(10)
		pvcCapSizeGi := int64(10)
		limit := int64(100 << 30)
		fsMode := corev1.PersistentVolumeFilesystem

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: pvcNS,
				Annotations: map[string]string{
					pvcautoresizer.ResizeTargetResourceAPIVersionAnnotation: "v1",
					pvcautoresizer.ResizeTargetResourceKindAnnotation:       "ConfigMap",
					pvcautoresizer.ResizeTargetResourceNameAnnotation:       cmName,
					pvcautoresizer.ResizeTargetResourceJSONPathAnnotation:   ".data.storage",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewQuantity(pvcSizeGi<<30, resource.BinarySI),
					},
				},
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &scName,
				VolumeMode:       &fsMode,
			},
		}

		if len(threshold) != 0 {
			pvc.Annotations[pvcautoresizer.ResizeThresholdAnnotation] = threshold
		}
		if len(increase) != 0 {
			pvc.Annotations[pvcautoresizer.ResizeIncreaseAnnotation] = increase
		}
		if limit != 0 {
			pvc.Annotations[pvcautoresizer.StorageLimitAnnotation] = strconv.FormatInt(limit, 10)
		}

		err = k8sClient.Create(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		pvc.Status.Phase = corev1.ClaimBound
		pvc.Status.Capacity = map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceStorage: *resource.NewQuantity(pvcCapSizeGi<<30, resource.BinarySI),
		}
		err = k8sClient.Status().Update(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		// 3. Set metrics to trigger resize
		// Available 4Gi < 5Gi (50% of 10Gi) -> Trigger resize
		// New size = 10Gi + 10Gi = 20Gi
		setMetrics(pvcNS, pvcName, 4<<30, pvcCapSizeGi<<30, 1000, 1000)

		// 4. Verify ConfigMap is updated
		Eventually(func() error {
			var currentCM corev1.ConfigMap
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: pvcNS, Name: cmName}, &currentCM)
			if err != nil {
				return err
			}
			val := currentCM.Data["storage"]
			if val != "20Gi" {
				return fmt.Errorf("ConfigMap storage value should be 20Gi, but is %s", val)
			}
			return nil
		}, 10*time.Second).ShouldNot(HaveOccurred())

		// 5. Verify PVC spec is NOT updated
		var currentPVC corev1.PersistentVolumeClaim
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: pvcNS, Name: pvcName}, &currentPVC)
		Expect(err).NotTo(HaveOccurred())

		req := currentPVC.Spec.Resources.Requests.Storage().Value()
		Expect(req).To(Equal(pvcSizeGi << 30))

		// 6. Verify PVC annotation is updated
		Eventually(func() error {
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: pvcNS, Name: pvcName}, &currentPVC)
			if err != nil {
				return err
			}
			preCap, ok := currentPVC.Annotations[pvcautoresizer.PreviousCapacityBytesAnnotation]
			if !ok {
				return fmt.Errorf("PreviousCapacityBytesAnnotation not found")
			}
			if preCap != strconv.FormatInt(pvcCapSizeGi<<30, 10) {
				return fmt.Errorf("PreviousCapacityBytesAnnotation should be %d, but is %s", pvcCapSizeGi<<30, preCap)
			}
			return nil
		}, 10*time.Second).ShouldNot(HaveOccurred())

		// 7. Verify Metrics
		// We can check if CrPatchTotal incremented.
		// Since we can't easily reset metrics or isolate this test fully in parallel execution without checking specific labels,
		// we check if the value > 0 for our specific labels.
		mfs, err := getMetricsFamily()
		Expect(err).NotTo(HaveOccurred())

		mf, ok := mfs["pvcautoresizer_cr_patch_total"]
		Expect(ok).To(BeTrue())

		found := false
		for _, m := range mf.Metric {
			hasNS := false
			hasKind := false
			hasStatus := false
			for _, label := range m.Label {
				if *label.Name == "namespace" && *label.Value == pvcNS {
					hasNS = true
				}
				if *label.Name == "cr_kind" && *label.Value == "ConfigMap" {
					hasKind = true
				}
				if *label.Name == "status" && *label.Value == "success" {
					hasStatus = true
				}
			}
			if hasNS && hasKind && hasStatus {
				found = true
				Expect(m.Counter).NotTo(BeNil())
				Expect(m.Counter.Value).NotTo(BeNil())
				Expect(*m.Counter.Value).To(BeNumerically(">=", 1))
			}
		}
		Expect(found).To(BeTrue(), "Metric pvcautoresizer_cr_patch_total for ConfigMap success not found")
	})
})
