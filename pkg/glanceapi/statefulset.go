/*

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

package glanceapi

import (
	"fmt"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	glance "github.com/openstack-k8s-operators/glance-operator/pkg/glance"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	// GlanceAPIServiceCommand -
	GlanceAPIServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
	// GlanceAPIHttpdCommand -
	GlanceAPIHttpdCommand = "/usr/sbin/httpd -DFOREGROUND"
)

// StatefulSet func
func StatefulSet(
	instance *glancev1.GlanceAPI,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
	privileged bool,
) (*appsv1.StatefulSet, error) {
	runAsUser := int64(0)
	var config0644AccessMode int32 = 0644

	startupProbe := &corev1.Probe{
		FailureThreshold: 6,
		PeriodSeconds:    10,
	}
	livenessProbe := &corev1.Probe{
		PeriodSeconds:       3,
		InitialDelaySeconds: 3,
	}
	readinessProbe := &corev1.Probe{
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
		InitialDelaySeconds: 5,
	}

	args := []string{"-c", GlanceAPIServiceCommand}
	//
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
	//

	port := int32(glance.GlancePublicPort)
	tlsEnabled := instance.Spec.TLS.API.Enabled(service.EndpointPublic)

	if instance.Spec.APIType == glancev1.APIInternal {
		port = int32(glance.GlanceInternalPort)
		tlsEnabled = instance.Spec.TLS.API.Enabled(service.EndpointInternal)
	}

	livenessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: port},
	}
	readinessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: port},
	}

	if tlsEnabled {
		livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
		readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	}
	startupProbe.Exec = &corev1.ExecAction{
		Command: []string{
			"/bin/true",
		},
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	apiVolumes := []corev1.Volume{
		{
			Name: "config-data-custom",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Name + "-config-data",
				},
			},
		},
	}
	// Append LogVolume to the apiVolumes: this will be used to stream
	// logging
	apiVolumes = append(apiVolumes, glance.GetLogVolume()...)
	apiVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "glance-api-config.json",
			ReadOnly:  true,
		},
	}
	// Append LogVolume to the apiVolumes: this will be used to stream
	// logging
	apiVolumeMounts = append(apiVolumeMounts, glance.GetLogVolumeMount()...)

	// If cache is provided, we expect the main glance_controller to request a
	// PVC that should be used for that purpose (according to ImageCacheSize)
	if len(instance.Spec.ImageCacheSize) > 0 {
		apiVolumes = append(apiVolumes, glance.GetCacheVolume(glance.ServiceName+"-cache")...)
		apiVolumeMounts = append(apiVolumeMounts, glance.GetCacheVolumeMount()...)
	}

	extraVolPropagation := append(glance.GlanceAPIPropagation,
		storage.PropagationType(glance.GetGlanceAPIName(instance.Name)))
	httpdVolumeMount := glance.GetHttpdVolumeMount()

	// Add the CA bundle to the apiVolumes and httpdVolumeMount
	if instance.Spec.TLS.CaBundleSecretName != "" {
		apiVolumes = append(apiVolumes, instance.Spec.TLS.CreateVolume())
		apiVolumeMounts = append(apiVolumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
		httpdVolumeMount = append(httpdVolumeMount, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	for endpt := range GetGlanceEndpoints(instance.Spec.APIType) {
		if instance.Spec.TLS.API.Enabled(endpt) {
			var tlsEndptCfg tls.GenericService
			switch endpt {
			case service.EndpointPublic:
				tlsEndptCfg = instance.Spec.TLS.API.Public
			case service.EndpointInternal:
				tlsEndptCfg = instance.Spec.TLS.API.Internal
			}

			svc, err := tlsEndptCfg.ToService()
			if err != nil {
				return nil, err
			}
			// httpd container is not using kolla, mount the certs to its dst
			svc.CertMount = ptr.To(fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String()))
			svc.KeyMount = ptr.To(fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String()))

			apiVolumes = append(apiVolumes, svc.CreateVolume(endpt.String()))
			httpdVolumeMount = append(httpdVolumeMount, svc.CreateVolumeMounts(endpt.String())...)
		}
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Spec.ServiceAccount,
					// When using Cinder we run as privileged, but also some
					// commands need to be run on the host using nsenter (eg:
					// iscsi commands) so we need to share the PID namespace
					// with the host.
					HostPID: privileged,
					Containers: []corev1.Container{
						{
							Name: glance.ServiceName + "-log",
							Command: []string{
								"/usr/bin/dumb-init",
							},
							Args: []string{
								"--single-child",
								"--",
								"/usr/bin/tail",
								"-n+1",
								"-F",
								string(glance.GlanceLogPath + instance.Name + ".log"),
							},
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:            env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:   glance.GetLogVolumeMount(),
							Resources:      instance.Spec.Resources,
							StartupProbe:   startupProbe,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
						{
							Name: glance.ServiceName + "-httpd",
							Command: []string{
								"/bin/bash",
							},
							Args:  []string{"-c", GlanceAPIHttpdCommand},
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:            env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:   httpdVolumeMount,
							Resources:      instance.Spec.Resources,
							StartupProbe:   startupProbe,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
						{
							Name: glance.ServiceName + "-api",
							Command: []string{
								"/bin/bash",
							},
							Args:  args,
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &runAsUser,
								Privileged: &privileged,
							},
							Env: env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts: append(glance.GetVolumeMounts(
								instance.Spec.CustomServiceConfigSecrets,
								privileged,
								instance.Spec.ExtraMounts,
								extraVolPropagation),
								apiVolumeMounts...,
							),
							Resources:      instance.Spec.Resources,
							StartupProbe:   startupProbe,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
					},
				},
			},
		},
	}
	localPvc, err := glance.GetPvc(instance, labels, glance.PvcLocal)
	if err != nil {
		return statefulset, err
	}
	statefulset.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{localPvc}

	if len(instance.Spec.ImageCacheSize) > 0 {
		cachePvc, err := glance.GetPvc(instance, labels, glance.PvcCache)
		if err != nil {
			return statefulset, err
		}
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, cachePvc)
	}
	statefulset.Spec.Template.Spec.Volumes = append(glance.GetVolumes(
		instance.Name,
		glance.ServiceName,
		privileged,
		instance.Spec.CustomServiceConfigSecrets,
		instance.Spec.ExtraMounts,
		extraVolPropagation),
		apiVolumes...)

	// If possible two pods of the same service should not
	// run on the same worker node. If this is not possible
	// the get still created on the same worker node.
	statefulset.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			glance.ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		statefulset.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	return statefulset, err
}
