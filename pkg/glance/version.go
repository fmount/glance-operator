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

package glance

import (
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Version func
func Version(
	instance *glancev1.Glance,
	labels map[string]string,
	annotations map[string]string,
) *batchv1.Job {

	versionMounts := GetScriptVolumeMount()
	versionVolumes := GetScriptVolume()
	// add CA cert if defined from the first api
	for _, api := range instance.Spec.GlanceAPIs {
		if api.TLS.CaBundleSecretName != "" {
			versionVolumes = append(versionVolumes, api.TLS.CreateVolume())
			versionMounts = append(versionMounts, api.TLS.CreateVolumeMounts(nil)...)

			break
		}
	}

	args := []string{"-c", GlanceVersionCommand}

	envVars := map[string]env.Setter{}
	envVars["CM_NAME"] = env.SetValue(GlanceVersionMapName)
	envVars["NAMESPACE"] = env.SetValue(instance.Namespace)
	envVars["OWNER_APIVERSION"] = env.SetValue(instance.APIVersion)
	envVars["OWNER_KIND"] = env.SetValue(instance.Kind)
	envVars["OWNER_UID"] = env.SetValue(string(instance.ObjectMeta.UID))
	envVars["OWNER_NAME"] = env.SetValue(instance.ObjectMeta.Name)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName + "-detect-version",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: instance.RbacResourceName(),
					Containers: []corev1.Container{
						{
							Name: ServiceName + "-detect-version",
							Command: []string{
								"/bin/bash",
							},
							Args:            args,
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							Image:           instance.Spec.ContainerImage,
							SecurityContext: dbSyncSecurityContext(),
							VolumeMounts:    versionMounts,
						},
					},
				},
			},
		},
	}

	job.Spec.Template.Spec.Volumes = versionVolumes
	if instance.Spec.NodeSelector != nil {
		job.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}
	return job
}
