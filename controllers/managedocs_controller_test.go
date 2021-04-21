package controllers

import (
	"context"
	"fmt"
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ocsv1 "github.com/openshift/ocs-operator/pkg/apis/ocs/v1"
	v1 "github.com/openshift/ocs-osd-deployer/api/v1alpha1"
	utils "github.com/openshift/ocs-osd-deployer/testutils"
	ctrlutils "github.com/openshift/ocs-osd-deployer/utils"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ManagedOCS controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 3
		interval = time.Millisecond * 250
	)

	ctx := context.Background()
	managedOCSTemplate := &v1.ManagedOCS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedOCSName,
			Namespace: testPrimaryNamespace,
		},
	}
	scTemplate := ocsv1.StorageCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      storageClusterName,
			Namespace: testPrimaryNamespace,
		},
	}
	promTemplate := promv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prometheusName,
			Namespace: testPrimaryNamespace,
		},
	}
	promStsTemplate := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("prometheus-%s", prometheusName),
			Namespace: testPrimaryNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"label": "value"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"label": "value"},
				},
			},
		},
	}
	amTemplate := promv1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertmanagerName,
			Namespace: testPrimaryNamespace,
		},
	}
	amStsTemplate := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("alertmanager-%s", alertmanagerName),
			Namespace: testPrimaryNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"label": "value"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"label": "value"},
				},
			},
		},
	}
	podMonitorTemplate := promv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-monitor",
			Namespace: testPrimaryNamespace,
		},
		Spec: promv1.PodMonitorSpec{
			PodMetricsEndpoints: []promv1.PodMetricsEndpoint{},
		},
	}
	serviceMonitorTemplate := promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-monitor",
			Namespace: testPrimaryNamespace,
		},
		Spec: promv1.ServiceMonitorSpec{
			Endpoints: []promv1.Endpoint{},
		},
	}
	promRuleTemplate := promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-rule",
			Namespace: testPrimaryNamespace,
		},
	}
	addonParamsSecretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAddonParamsSecretName,
			Namespace: testPrimaryNamespace,
		},
		Data: map[string][]byte{},
	}
	pdSecretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPagerdutySecretName,
			Namespace: testPrimaryNamespace,
		},
		Data: map[string][]byte{},
	}
	amConfigSecretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertmanagerConfigSecretName,
			Namespace: testPrimaryNamespace,
		},
		Data: map[string][]byte{},
	}
	addonConfigMapTemplate := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAddonConfigMapName,
			Namespace: testPrimaryNamespace,
		},
	}
	pvc1StorageClassName := storageClassRbdName
	pvc1Template := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-1",
			Namespace: testPrimaryNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &pvc1StorageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	pvc2StorageClassName := storageClassCephFSName
	pvc2Template := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-2",
			Namespace: testPrimaryNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &pvc2StorageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	subscriptionTemplate := opv1a1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSubscriptionName,
			Namespace: testPrimaryNamespace,
		},
	}
	csvTemplate := opv1a1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeployerCSVName,
			Namespace: testPrimaryNamespace,
		},
	}

	setupUninstallConditions := func(
		shouldAddonConfigMapExist bool,
		addonConfigMapDeleteLabel string,
		shouldStorageClusterBeReady bool,
		shouldPrometheusBeReady bool,
		shouldAlertmanagerBeReady bool,
		shouldPVC1Exist bool,
		shouldPVC2Exist bool,
	) {
		// Delete the configmap to ensure that we will not trigger uninstall accidentally
		// via and intermediate state
		configMap := addonConfigMapTemplate.DeepCopy()
		err := k8sClient.Delete(ctx, configMap)
		Expect(err == nil || errors.IsNotFound(err)).Should(BeTrue())

		// Setup storagecluster state
		sc := scTemplate.DeepCopy()
		Expect(k8sClient.Get(ctx, utils.GetResourceKey(sc), sc)).Should(Succeed())
		if shouldStorageClusterBeReady {
			sc.Status.Phase = "Ready"
		} else {
			sc.Status.Phase = ""
		}
		Expect(k8sClient.Status().Update(ctx, sc)).Should(Succeed())

		// Setup prometheus state
		prom := promTemplate.DeepCopy()
		Expect(k8sClient.Get(ctx, utils.GetResourceKey(prom), prom)).Should(Succeed())
		desiredReplicas := int32(1)
		if prom.Spec.Replicas != nil {
			desiredReplicas = *prom.Spec.Replicas
		}
		promSts := promStsTemplate.DeepCopy()
		Expect(k8sClient.Get(ctx, utils.GetResourceKey(promSts), promSts)).Should(Succeed())
		if shouldPrometheusBeReady {
			promSts.Status.Replicas = desiredReplicas
			promSts.Status.ReadyReplicas = desiredReplicas
		} else {
			promSts.Status.Replicas = 0
			promSts.Status.ReadyReplicas = 0
		}
		Expect(k8sClient.Status().Update(ctx, promSts)).Should(Succeed())

		// Setup alertmanager state
		am := amTemplate.DeepCopy()
		Expect(k8sClient.Get(ctx, utils.GetResourceKey(am), am)).Should(Succeed())
		desiredReplicas = int32(1)
		if am.Spec.Replicas != nil {
			desiredReplicas = *am.Spec.Replicas
		}
		amSts := amStsTemplate.DeepCopy()
		Expect(k8sClient.Get(ctx, utils.GetResourceKey(amSts), amSts)).Should(Succeed())
		if shouldAlertmanagerBeReady {
			amSts.Status.Replicas = desiredReplicas
			amSts.Status.ReadyReplicas = desiredReplicas
		} else {
			amSts.Status.Replicas = 0
			amSts.Status.ReadyReplicas = 0
		}
		Expect(k8sClient.Status().Update(ctx, amSts)).Should(Succeed())

		// Setup pvc1 state (an rbd backed pvc in the primary namespace)
		pvc1 := pvc1Template.DeepCopy()
		if shouldPVC1Exist {
			err := k8sClient.Create(ctx, pvc1)
			Expect(err == nil || errors.IsAlreadyExists(err)).Should(BeTrue())
		} else {
			err := k8sClient.Get(ctx, utils.GetResourceKey(pvc1), pvc1)
			if err == nil {
				pvc1.SetFinalizers(ctrlutils.Remove(pvc1.GetFinalizers(), "kubernetes.io/pvc-protection"))
				Expect(k8sClient.Status().Update(ctx, pvc1)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, pvc1)).Should(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).Should(BeTrue())
			}
		}

		// Setup pvc2 state (an cephfs backed pvc in the secondary namespace)
		pvc2 := pvc2Template.DeepCopy()
		if shouldPVC2Exist {
			err := k8sClient.Create(ctx, pvc2)
			Expect(err == nil || errors.IsAlreadyExists(err)).Should(BeTrue())
		} else {
			err := k8sClient.Get(ctx, utils.GetResourceKey(pvc2), pvc2)
			if err == nil {
				pvc2.SetFinalizers(ctrlutils.Remove(pvc2.GetFinalizers(), "kubernetes.io/pvc-protection"))
				Expect(k8sClient.Status().Update(ctx, pvc2)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, pvc2)).Should(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).Should(BeTrue())
			}
		}

		// setup add-on configmap state
		if shouldAddonConfigMapExist {
			labels := map[string]string{}
			if addonConfigMapDeleteLabel != "" {
				labels[addonConfigMapDeleteLabel] = "dummy"
			}
			configMap.SetLabels(labels)
			Expect(k8sClient.Create(ctx, configMap)).Should(Succeed())
		}
	}

	Context("reconcile()", func() {
		When("there is no add-on parameters secret in the cluster", func() {
			It("should not create a reconciled resources", func() {
				// Verify that a secret is not present
				secret := addonParamsSecretTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(secret), secret)).Should(
					WithTransform(errors.IsNotFound, BeTrue()),
				)

				// Ensure, over a period of time, that the resources are not created
				resList := []runtime.Object{
					scTemplate.DeepCopy(),
					promTemplate.DeepCopy(),
					amTemplate.DeepCopy(),
				}
				utils.EnsureNoResources(k8sClient, ctx, resList, timeout, interval)
			})
		})
		When("there is no size field in the add-on parameters secret", func() {
			It("should not create a reconciled resources", func() {
				// Create empty add-on parameters secret
				secret := addonParamsSecretTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				// Ensure, over a period of time, that the resources are not created
				resList := []runtime.Object{
					scTemplate.DeepCopy(),
					promTemplate.DeepCopy(),
					amTemplate.DeepCopy(),
				}
				utils.EnsureNoResources(k8sClient, ctx, resList, timeout, interval)

				// Remove the secret for future cases
				Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			})
		})
		When("there is an invalid size value in the add-on parameters secret", func() {
			It("should not create reconciled resources", func() {
				// Create a invalid add-on parameters secret
				secret := addonParamsSecretTemplate.DeepCopy()
				secret.Data["size"] = []byte("AA")
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				// Ensure, over a period of time, that the resources are not created
				resList := []runtime.Object{
					scTemplate.DeepCopy(),
					promTemplate.DeepCopy(),
					amTemplate.DeepCopy(),
				}
				utils.EnsureNoResources(k8sClient, ctx, resList, timeout, interval)

				// Remove the secret for future cases
				Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			})
		})
		When("there is a valid size in the add-on parameter secret", func() {
			It("should create reconciled resources", func() {
				// Create a valid add-on parameters secret
				secret := addonParamsSecretTemplate.DeepCopy()
				secret.Data["size"] = []byte("1")
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				By("Creating a storagecluster resource")
				utils.WaitForResource(k8sClient, ctx, scTemplate.DeepCopy(), timeout, interval)

				By("Creating a prometous resource")
				utils.WaitForResource(k8sClient, ctx, promTemplate.DeepCopy(), timeout, interval)

				By("Creating an alertmanager resource")
				utils.WaitForResource(k8sClient, ctx, amTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("size is increased in the add-on parameters secret", func() {
			It("should increase storagecluster's storage device set count", func() {
				secret := addonParamsSecretTemplate.DeepCopy()
				secret.Data["size"] = []byte("4")
				Expect(k8sClient.Update(ctx, secret)).Should(Succeed())

				// wait for the storagecluster to update
				Eventually(func() bool {
					sc := scTemplate.DeepCopy()
					err := k8sClient.Get(ctx, utils.GetResourceKey(sc), sc)
					if err != nil {
						return false
					}

					var ds *ocsv1.StorageDeviceSet = nil
					for index := range sc.Spec.StorageDeviceSets {
						item := &sc.Spec.StorageDeviceSets[index]
						if item.Name == deviceSetName {
							ds = item
							break
						}
					}
					return ds != nil && ds.Count == 4
				}, timeout, interval).Should(BeTrue())

			})
		})
		When("size is decreased in the add-on parameters secret", func() {
			It("should not decrease storagecluster's storage device set count", func() {
				secret := addonParamsSecretTemplate.DeepCopy()
				secret.Data["size"] = []byte("1")
				Expect(k8sClient.Update(ctx, secret)).Should(Succeed())

				Consistently(func() bool {
					sc := scTemplate.DeepCopy()
					err := k8sClient.Get(ctx, utils.GetResourceKey(sc), sc)
					if err != nil {
						return false
					}

					var ds *ocsv1.StorageDeviceSet = nil
					for index := range sc.Spec.StorageDeviceSets {
						item := &sc.Spec.StorageDeviceSets[index]
						if item.Name == deviceSetName {
							ds = item
							break
						}
					}
					return ds != nil && ds.Count == 4
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("the storagecluster is not ready", func() {
			BeforeEach(func() {
				// Ensure that the storagecluster is not ready
				// This test assumes a StorageCluster is already created.
				sc := scTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(sc), sc)).Should(Succeed())

				// Updating the Status of the StorageCluster should trigger a reconcile for managed-ocs
				sc.Status.Phase = "Pending"
				Expect(k8sClient.Status().Update(ctx, sc)).Should(Succeed())
			})
			It("should reflect that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.StorageCluster.State to Pending")
				Eventually(func() v1.ComponentState {
					managedOCS := managedOCSTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, utils.GetResourceKey(managedOCS), managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.StorageCluster.State
				}, timeout, interval).Should(Equal(v1.ComponentPending))
			})
		})
		When("the storeagecluster is ready", func() {
			BeforeEach(func() {
				// Ensure that the storagecluster is ready
				// This test assumes a StorageCluster is already created.
				sc := scTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(sc), sc)).Should(Succeed())

				// Updating the Status of the storagecluster resource should trigger a reconcile for managedocs
				sc.Status.Phase = "Ready"
				Expect(k8sClient.Status().Update(ctx, sc)).Should(Succeed())
			})
			It("should refelct that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.StorageCluster.State to Ready")
				Eventually(func() v1.ComponentState {
					managedOCS := managedOCSTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, utils.GetResourceKey(managedOCS), managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.StorageCluster.State
				}, timeout, interval).Should(Equal(v1.ComponentReady))
			})
		})
		When("prometheus has non-ready replicas", func() {
			It("should reflect that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.Prometheus.State to Pending")
				// Updating the status of the prometheus statefulset should trigger a reconcile for managedocs
				promSts := promStsTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, promSts)).Should(Succeed())

				promSts.Status.Replicas = 0
				promSts.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, promSts)).Should(Succeed())

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey(managedOCS)
				Eventually(func() v1.ComponentState {
					Expect(k8sClient.Get(ctx, key, managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.Prometheus.State
				}, timeout, interval).Should(Equal(v1.ComponentPending))
			})
		})
		When("all prometheus replicas are ready", func() {
			It("should reflect that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.Prometheus.State to Ready")
				prom := promTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(prom), prom)).Should(Succeed())
				desiredReplicas := int32(1)
				if prom.Spec.Replicas != nil {
					desiredReplicas = *prom.Spec.Replicas
				}

				// Updating the status of the prometheus statefulset should trigger a reconcile for managedocs
				promSts := promStsTemplate.DeepCopy()
				promSts.Status.Replicas = desiredReplicas
				promSts.Status.ReadyReplicas = desiredReplicas
				Expect(k8sClient.Status().Update(ctx, promSts)).Should(Succeed())

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey(managedOCS)
				Eventually(func() v1.ComponentState {
					Expect(k8sClient.Get(ctx, key, managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.Prometheus.State
				}, timeout, interval).Should(Equal(v1.ComponentReady))
			})
		})
		When("alertmanager has non-ready replicas", func() {
			It("should reflect that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.Alertmanager.State to Pending")
				// Updating the status of the alertmanager statefulset should trigger a reconcile for managedocs
				amSts := amStsTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, amSts)).Should(Succeed())

				amSts.Status.Replicas = 0
				amSts.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, amSts)).Should(Succeed())

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey(managedOCS)
				Eventually(func() v1.ComponentState {
					Expect(k8sClient.Get(ctx, key, managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.Alertmanager.State
				}, timeout, interval).Should(Equal(v1.ComponentPending))
			})
		})
		When("all alertmanager replicas are ready", func() {
			It("should reflect that in the ManagedOCS resource status", func() {
				By("by setting Status.Components.Alertmanager.State to Ready")
				am := amTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(am), am)).Should(Succeed())
				desiredReplicas := int32(1)
				if am.Spec.Replicas != nil {
					desiredReplicas = *am.Spec.Replicas
				}

				// Updating the status of the alertmanager statefulset should trigger a reconcile for managedocs
				amSts := amStsTemplate.DeepCopy()
				amSts.Status.Replicas = desiredReplicas
				amSts.Status.ReadyReplicas = desiredReplicas
				Expect(k8sClient.Status().Update(ctx, amSts)).Should(Succeed())

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey(managedOCS)
				Eventually(func() v1.ComponentState {
					Expect(k8sClient.Get(ctx, key, managedOCS)).Should(Succeed())
					return managedOCS.Status.Components.Alertmanager.State
				}, timeout, interval).Should(Equal(v1.ComponentReady))
			})
		})
		When("the storagecluster resource is deleted", func() {
			It("should create a new storagecluster in the namespace", func() {
				// Delete the storagecluster resource
				Expect(k8sClient.Delete(ctx, scTemplate.DeepCopy())).Should(Succeed())

				// Wait for the storagecluster to be recreated
				utils.WaitForResource(k8sClient, ctx, scTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("the storagecluster resource is modified while the reconcile strategy is not set", func() {
			It("should revert the changes and bring the resource back to its managed state", func() {
				// Set managed OCS to reconcile strategy to strict
				managedOCS := managedOCSTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(managedOCS), managedOCS)).Should(Succeed())
				managedOCS.Spec.ReconcileStrategy = ""
				Expect(k8sClient.Update(ctx, managedOCS)).Should(Succeed())

				// Get an updated storagecluster
				sc := scTemplate.DeepCopy()
				scKey := utils.GetResourceKey(sc)
				Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())

				// Update to empty spec
				spec := sc.Spec.DeepCopy()
				sc.Spec = ocsv1.StorageClusterSpec{}
				Expect(k8sClient.Update(ctx, sc)).Should(Succeed())

				// Wait for the spec changes to be reverted
				Eventually(func() *ocsv1.StorageClusterSpec {
					sc := scTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())
					return &sc.Spec
				}, timeout, interval).Should(Equal(spec))
			})
		})
		When("the storagecluster resource is modified while the reconcile strategy is set to strict", func() {
			It("should revert the changes and bring the resource back to its managed state", func() {
				// Set managed OCS to reconcile strategy to strict
				managedOCS := managedOCSTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(managedOCS), managedOCS)).Should(Succeed())
				managedOCS.Spec.ReconcileStrategy = v1.ReconcileStrategyStrict
				Expect(k8sClient.Update(ctx, managedOCS)).Should(Succeed())

				// Get an updated storagecluster
				sc := scTemplate.DeepCopy()
				scKey := utils.GetResourceKey(sc)
				Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())

				// Update to empty spec
				spec := sc.Spec.DeepCopy()
				sc.Spec = ocsv1.StorageClusterSpec{}
				Expect(k8sClient.Update(ctx, sc)).Should(Succeed())

				// Wait for the spec changes to be reverted
				Eventually(func() *ocsv1.StorageClusterSpec {
					sc := scTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())
					return &sc.Spec
				}, timeout, interval).Should(Equal(spec))
			})
		})
		When("the storagecluster resource is modified while the reconcile strategy is set to none", func() {
			It("should not revert any changes back to the managed state", func() {
				// Set managed OCS to reconcile strategy to none
				managedOCS := managedOCSTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(managedOCS), managedOCS)).Should(Succeed())
				managedOCS.Spec.ReconcileStrategy = v1.ReconcileStrategyNone
				Expect(k8sClient.Update(ctx, managedOCS)).Should(Succeed())

				// Get an updated storagecluster
				sc := scTemplate.DeepCopy()
				scKey := utils.GetResourceKey(sc)
				Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())

				// Update to empty spec
				sc.Spec = ocsv1.StorageClusterSpec{}
				Expect(k8sClient.Update(ctx, sc)).Should(Succeed())

				// Verify that the spec changes are not reverted
				Consistently(func() *ocsv1.StorageClusterSpec {
					sc := scTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, scKey, sc)).Should(Succeed())
					return &sc.Spec
				}, timeout, interval).Should(Equal(&sc.Spec))
			})
		})
		When("the prometheus resource is modified", func() {
			It("should revert the changes and bring the resource back to its managed state", func() {
				// Get an updated prometheus
				prom := promTemplate.DeepCopy()
				promKey := utils.GetResourceKey(prom)
				Expect(k8sClient.Get(ctx, promKey, prom)).Should(Succeed())

				// Update to empty spec
				spec := prom.Spec.DeepCopy()
				prom.Spec = promv1.PrometheusSpec{}
				Expect(k8sClient.Update(ctx, prom)).Should(Succeed())

				// Wait for the spec changes to be reverted
				Eventually(func() *promv1.PrometheusSpec {
					prom := promTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, promKey, prom)).Should(Succeed())
					return &prom.Spec
				}, timeout, interval).Should(Equal(spec))
			})
		})
		When("the prometheus resource is deleted", func() {
			It("should create a new prometheus in the namespace", func() {
				// Delete the prometheus resource
				Expect(k8sClient.Delete(ctx, promTemplate.DeepCopy())).Should(Succeed())

				// Wait for the prometheus to be recreated
				utils.WaitForResource(k8sClient, ctx, promTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("the alertmanager resource is modified", func() {
			It("should revert the changes and bring the resource back to its managed state", func() {
				// Get an updated alertmanager
				am := amTemplate.DeepCopy()
				amKey := utils.GetResourceKey(am)
				Expect(k8sClient.Get(ctx, amKey, am)).Should(Succeed())

				// Update to empty spec
				spec := am.Spec.DeepCopy()
				am.Spec = promv1.AlertmanagerSpec{}
				Expect(k8sClient.Update(ctx, am)).Should(Succeed())

				// Wait for the spec changes to be reverted
				Eventually(func() *promv1.AlertmanagerSpec {
					am := amTemplate.DeepCopy()
					Expect(k8sClient.Get(ctx, amKey, am)).Should(Succeed())
					return &am.Spec
				}, timeout, interval).Should(Equal(spec))
			})
		})
		When("the alertmanager resource is deleted", func() {
			It("should create a new alertmanager in the namespace", func() {
				// Delete the alertmanager resource
				Expect(k8sClient.Delete(ctx, amTemplate.DeepCopy())).Should(Succeed())

				// Wait for the alertmanager to be recreated
				utils.WaitForResource(k8sClient, ctx, promTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("there is no pagerduty secret in the cluster", func() {
			It("should not create alertmanager config secret", func() {
				// Verify that a pagerduty secret is not present
				secret := pdSecretTemplate.DeepCopy()
				Expect(k8sClient.Get(ctx, utils.GetResourceKey(secret), secret)).Should(
					WithTransform(errors.IsNotFound, BeTrue()),
				)

				// Ensure, over a period of time, that the resources are not created
				utils.EnsureNoResource(k8sClient, ctx, amConfigSecretTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("there is no value for PAGERDUTY_KEY in the pagerduty secret", func() {
			It("should not create alertmanager config secret", func() {
				// Create empty pagerduty secret
				secret := pdSecretTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				// Ensure, over a period of time, that the resources are not created
				utils.EnsureNoResource(k8sClient, ctx, amConfigSecretTemplate.DeepCopy(), timeout, interval)

				// Remove the secret for future cases
				Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			})
		})
		When("there is a value for PAGERDUTY_KEY in the pagerduty secret", func() {
			It("should create alertmanager config secret", func() {
				// Create pagerduty secret with valid key
				secret := pdSecretTemplate.DeepCopy()
				secret.Data["PAGERDUTY_KEY"] = []byte("test-key")
				By("Creating an alertmanager config secret")
				Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

				utils.WaitForResource(k8sClient, ctx, amConfigSecretTemplate.DeepCopy(), timeout, interval)
			})
		})
		When("there is a pod monitor without an ocs-dedicated label", func() {
			It("should add the label to the pod monitor resource", func() {
				pm := podMonitorTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, pm)).Should(Succeed())

				Eventually(func() bool {
					return utils.ResourceHasLabel(k8sClient, ctx, pm, monLabelKey, monLabelValue)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("there is a service monitor without an ocs-dedicated label", func() {
			It("should add the label to the service monitor resource", func() {
				sm := serviceMonitorTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, sm)).Should(Succeed())

				Eventually(func() bool {
					return utils.ResourceHasLabel(k8sClient, ctx, sm, monLabelKey, monLabelValue)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("there is a prometheus rule without a ocs-dedicated label", func() {
			It("should add the label to the prometheus rule resource", func() {
				pr := promRuleTemplate.DeepCopy()
				Expect(k8sClient.Create(ctx, pr)).Should(Succeed())

				Eventually(func() bool {
					return utils.ResourceHasLabel(k8sClient, ctx, pr, monLabelKey, monLabelValue)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("the addon config map does not exist while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(false, testAddonConfigMapDeleteLabelKey, true, true, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("the addon config map does not have a delete label while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, "", true, true, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("the addon config map does not have a valid delete label while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, "invalid-label", true, true, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("the storagecluster is not ready while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, false, true, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("prometheus is not ready while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, true, false, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("alertmanager is not ready while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, true, true, false, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("there are pvcs in the primary namespace while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, true, true, false, true, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("there are pvcs in a secondary namespace while all other uninstall conditions are met", func() {
			It("should not delete the managedOCS resource", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, true, true, false, false, true)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Consistently(func() error {
					return k8sClient.Get(ctx, key, managedOCS)
				}, timeout, interval).Should(Succeed())
			})
		})
		When("All uninstall conditions are met", func() {
			It("should delete the managedOCS", func() {
				setupUninstallConditions(true, testAddonConfigMapDeleteLabelKey, true, true, true, false, false)

				managedOCS := managedOCSTemplate.DeepCopy()
				key := utils.GetResourceKey((managedOCS))
				Eventually(func() bool {
					err := k8sClient.Get(ctx, key, managedOCS)
					return err != nil && errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
			It("should delete the deployer subscription", func() {
				sub := subscriptionTemplate.DeepCopy()
				key := utils.GetResourceKey(sub)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, key, sub)
					return err != nil && errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
			It("should delete the deployer csv", func() {
				csv := csvTemplate.DeepCopy()
				key := utils.GetResourceKey(csv)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, key, csv)
					return err != nil && errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
