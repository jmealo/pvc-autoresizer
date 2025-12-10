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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("parent resource patching", func() {
	It("should patch the parent resource instead of resizing PVC directly", func() {
		ctx := context.Background()
		ns := "default"
		pvcName := "test-parent-patch-pvc"
		parentName := "test-parent-cm"

		// 1. Create Parent (ConfigMap)
		parent := &unstructured.Unstructured{}
		parent.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "ConfigMap",
		})
		parent.SetName(parentName)
		parent.SetNamespace(ns)
		parent.Object["data"] = map[string]interface{}{
			"storage": "10Gi",
		}
		err := k8sClient.Create(ctx, parent)
		Expect(err).NotTo(HaveOccurred())

		// 2. Create PVC with annotations
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ns,
				Annotations: map[string]string{
					pvcautoresizer.ResizeThresholdAnnotation:     "50%",
					pvcautoresizer.ResizeTargetKindAnnotation:    "ConfigMap",
					pvcautoresizer.ResizeTargetGroupAnnotation:   "",
					pvcautoresizer.ResizeTargetVersionAnnotation: "v1",
					pvcautoresizer.ResizeTargetNameAnnotation:    parentName,
					pvcautoresizer.ResizeTargetPathAnnotation:    "data/storage",
					pvcautoresizer.StorageLimitAnnotation:        "100Gi",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &scName,
				VolumeMode:       func() *corev1.PersistentVolumeMode { m := corev1.PersistentVolumeFilesystem; return &m }(),
			},
		}
		err = k8sClient.Create(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		// Update Status
		pvc.Status.Phase = corev1.ClaimBound
		pvc.Status.Capacity = map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceStorage: resource.MustParse("10Gi"),
		}
		err = k8sClient.Status().Update(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		// 3. Set Metrics to trigger resize
		// Threshold 50% of 10Gi = 5Gi. Available = 4Gi (< 5Gi) -> Trigger Resize.
		// Increase default 10% -> 11Gi.
		setMetrics(ns, pvcName, 4<<30, 10<<30, 1000, 1000)

		// 4. Verify Parent Patch
		Eventually(func() error {
			updatedParent := &unstructured.Unstructured{}
			updatedParent.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: parentName}, updatedParent)
			if err != nil {
				return err
			}

			val, found, err := unstructured.NestedString(updatedParent.Object, "data", "storage")
			if !found || err != nil {
				return fmt.Errorf("data.storage not found")
			}

			// Expected: 11Gi
			q, err := resource.ParseQuantity(val)
			if err != nil {
				return err
			}
			if q.Value() != 11<<30 {
				return fmt.Errorf("expected 11Gi, got %s", val)
			}
			return nil
		}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

		// 5. Verify PVC is NOT resized in Spec
		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, updatedPVC)
		Expect(err).NotTo(HaveOccurred())

		// Spec should still be 10Gi
		Expect(updatedPVC.Spec.Resources.Requests.Storage().Value()).To(Equal(int64(10 << 30)))

		// Annotation should be updated
		val, ok := updatedPVC.Annotations[pvcautoresizer.PreviousCapacityBytesAnnotation]
		Expect(ok).To(BeTrue())
		Expect(val).To(Equal(strconv.FormatInt(10<<30, 10)))
	})
})
