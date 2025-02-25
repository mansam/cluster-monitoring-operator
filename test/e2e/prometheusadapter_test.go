// Copyright 2019 The Cluster Monitoring Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/openshift/cluster-monitoring-operator/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiservicesv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

func isNodeInNodesList(node string, nodes []corev1.Node) bool {
	for _, n := range nodes {
		if n.Name == node {
			return true
		}
	}
	return false
}

func isPodInPodsList(pod string, ns string, pods []corev1.Pod) bool {
	for _, p := range pods {
		if p.Name == pod && p.Namespace == ns {
			return true
		}
	}
	return false
}

func isAPIServiceAvailable(conditions []apiservicesv1.APIServiceCondition) bool {
	for _, condition := range conditions {
		if condition.Type == apiservicesv1.Available && condition.Status == apiservicesv1.ConditionTrue {
			return true
		}
	}
	return false
}

func TestMetricsAPIAvailability(t *testing.T) {
	for _, tc := range []struct {
		cmoConfig string
	}{
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: false
`,
		},
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: true
`,
		},
	} {
		f.MustCreateOrUpdateConfigMap(t, f.BuildCMOConfigMap(t, tc.cmoConfig))

		f.AssertOperatorCondition(configv1.OperatorDegraded, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorProgressing, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorAvailable, configv1.ConditionTrue)(t)

		checkMetricsAPIAvailability(t)
	}
}

func checkMetricsAPIAvailability(t *testing.T) {
	ctx := context.Background()
	var lastErr error
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		metricsService, err := f.APIServicesClient.ApiregistrationV1().APIServices().Get(ctx, "v1beta1.metrics.k8s.io", metav1.GetOptions{})
		lastErr = errors.Wrap(err, "getting metrics APIService failed")
		if err != nil {
			return false, nil
		}
		if !isAPIServiceAvailable(metricsService.Status.Conditions) {
			lastErr = errors.New("v1beta1.metrics.k8s.io apiservice is not available")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if err == wait.ErrWaitTimeout && lastErr != nil {
			err = lastErr
		}
		t.Fatal(err)
	}
}

func TestNodeMetricsPresence(t *testing.T) {
	for _, tc := range []struct {
		cmoConfig string
	}{
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: false
`,
		},
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: true
`,
		},
	} {
		f.MustCreateOrUpdateConfigMap(t, f.BuildCMOConfigMap(t, tc.cmoConfig))

		f.AssertOperatorCondition(configv1.OperatorDegraded, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorProgressing, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorAvailable, configv1.ConditionTrue)(t)

		checkNodeMetricsPresence(t)
	}
}

func checkNodeMetricsPresence(t *testing.T) {
	ctx := context.Background()
	var lastErr error
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		nodes, err := f.KubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		lastErr = errors.Wrap(err, "getting nodes list failed")
		if err != nil {
			return false, nil
		}
		nodeMetrics, err := f.MetricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		lastErr = errors.Wrap(err, "getting metrics list failed")
		if err != nil {
			return false, nil
		}
		if len(nodes.Items) != len(nodeMetrics.Items) {
			lastErr = errors.New("number of nodes doesn't match number of node metrics reported")
			return false, nil
		}
		for _, item := range nodeMetrics.Items {
			if !isNodeInNodesList(item.Name, nodes.Items) {
				lastErr = errors.New("node reporting metrics couldn't be found in nodes list")
				return false, nil
			}
			if item.Usage.Cpu() == nil || item.Usage.Memory() == nil {
				lastErr = errors.New("node cpu or memory metric not found")
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		if err == wait.ErrWaitTimeout && lastErr != nil {
			err = lastErr
		}
		t.Fatal(err)
	}
}

func TestPodMetricsPresence(t *testing.T) {
	for _, tc := range []struct {
		cmoConfig string
	}{
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: false
`,
		},
		{
			cmoConfig: `prometheusK8s:
  logLevel: debug
  k8sPrometheusAdapter:
    dedicatedServiceMonitors:
      enabled: true
`,
		},
	} {
		f.MustCreateOrUpdateConfigMap(t, f.BuildCMOConfigMap(t, tc.cmoConfig))

		f.AssertOperatorCondition(configv1.OperatorDegraded, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorProgressing, configv1.ConditionFalse)(t)
		f.AssertOperatorCondition(configv1.OperatorAvailable, configv1.ConditionTrue)(t)

		checkPodMetricsPresence(t)
	}
}

func checkPodMetricsPresence(t *testing.T) {
	var lastErr error
	ctx := context.Background()
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		pods, err := f.KubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Running"})
		lastErr = errors.Wrap(err, "getting pods list failed")
		if err != nil {
			return false, nil
		}
		podMetrics, err := f.MetricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
		lastErr = errors.Wrap(err, "getting metrics list failed")
		if err != nil {
			return false, nil
		}
		if len(pods.Items) != len(podMetrics.Items) {
			lastErr = fmt.Errorf("number of running pods (%d) doesn't match number of pods reporting metrics (%d)", len(pods.Items), len(podMetrics.Items))
			return false, nil
		}

		for _, pod := range podMetrics.Items {
			if !isPodInPodsList(pod.Name, pod.Namespace, pods.Items) {
				lastErr = errors.New("pod reporting metrics couldn't be found in pods list")
				return false, nil
			}
			for _, item := range pod.Containers {
				if item.Usage.Cpu() == nil || item.Usage.Memory() == nil {
					lastErr = errors.New("container cpu or memory metric not found")
					return false, nil
				}
			}
		}
		return true, nil
	})
	if err != nil {
		if err == wait.ErrWaitTimeout && lastErr != nil {
			err = lastErr
		}
		t.Fatal(err)
	}
}

func TestAggregatedMetricPermissions(t *testing.T) {
	ctx := context.Background()
	present := func(where []string, what string) bool {
		sort.Strings(where)
		i := sort.SearchStrings(where, what)
		return i < len(where) && where[i] == what
	}

	type checkFunc func(clusterRole string) error

	hasRule := func(apiGroup, resource, verb string) checkFunc {
		return func(clusterRole string) error {
			return framework.Poll(time.Second, 5*time.Minute, func() error {
				viewRole, err := f.KubeClient.RbacV1().ClusterRoles().Get(ctx, clusterRole, metav1.GetOptions{})
				if err != nil {
					return errors.Wrapf(err, "getting %s cluster role failed", clusterRole)
				}

				for _, rule := range viewRole.Rules {
					if !present(rule.APIGroups, apiGroup) {
						continue
					}

					if !present(rule.Resources, resource) {
						continue
					}

					if !present(rule.Verbs, verb) {
						continue
					}

					return nil
				}

				return fmt.Errorf("could not find metrics in cluster role %s", clusterRole)
			})
		}
	}

	canGetPodMetrics := hasRule("metrics.k8s.io", "pods", "get")

	for _, tc := range []struct {
		clusterRole string
		check       checkFunc
	}{
		{
			clusterRole: "view",
			check:       canGetPodMetrics,
		},
		{
			clusterRole: "edit",
			check:       canGetPodMetrics,
		},
		{
			clusterRole: "admin",
			check:       canGetPodMetrics,
		},
	} {
		t.Run(tc.clusterRole, func(t *testing.T) {
			if err := tc.check(tc.clusterRole); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestPrometheusAdapterCARotation(t *testing.T) {
	ctx := context.Background()
	// Wait for prometheus-adapter deployment
	f.AssertDeploymentExistsAndRollout("prometheus-adapter", f.Ns)(t)

	tls, err := f.KubeClient.CoreV1().Secrets(f.Ns).Get(ctx, "prometheus-adapter-tls", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	apiAuth, err := f.KubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	adapterSecret, err := f.ManifestsFactory.PrometheusAdapterSecret(tls, apiAuth)
	if err != nil {
		t.Fatal(err)
	}

	// the secret might not have been created yet, so wait for it
	f.AssertSecretExists(adapterSecret.GetName(), f.Ns)(t)

	// Delete the signer secrets. This causes kube-system/extension-apiserver-authentication
	// to be reissued.
	err = f.KubeClient.CoreV1().Secrets("openshift-kube-controller-manager-operator").Delete(ctx, "csr-signer-signer", metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = f.KubeClient.CoreV1().Secrets("openshift-kube-controller-manager-operator").Delete(ctx, "csr-signer", metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the new secret to be deployed
	var newSecret corev1.Secret
	err = framework.Poll(5*time.Second, 15*time.Minute, func() error {
		secrets, err := f.KubeClient.CoreV1().Secrets("openshift-monitoring").List(ctx, metav1.ListOptions{
			LabelSelector: "monitoring.openshift.io/name=prometheus-adapter,monitoring.openshift.io/hash!=" + adapterSecret.Labels["monitoring.openshift.io/hash"],
		})

		if err != nil {
			return errors.Wrap(err, "error listing prometheus adapter secrets")
		}

		if len(secrets.Items) == 0 {
			return errors.New("expected prometheus adapter secret to have rotated, but it didn't")
		}

		if got := len(secrets.Items); got > 1 {
			return fmt.Errorf("expected exactly 1 prometheus adapter secret to be present, got %d", got)
		}

		newSecret = secrets.Items[0]
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for prometheus-adapter deployment to reference new secret
	err = framework.Poll(time.Second, 5*time.Minute, func() error {
		d, err := f.KubeClient.AppsV1().Deployments(f.Ns).Get(ctx, "prometheus-adapter", metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "getting prometheus-adapter deployment failed")
		}

		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.Name != "tls" {
				continue
			}

			if v.VolumeSource.Secret.SecretName != newSecret.GetName() {
				continue
			}

			return nil
		}

		return fmt.Errorf("expected secret %v to be referenced in prometheus-adapter but it didn't", newSecret.GetName())
	})
	if err != nil {
		t.Fatal(err)
	}
	f.AssertDeploymentExistsAndRollout("prometheus-adapter", f.Ns)(t)
}
