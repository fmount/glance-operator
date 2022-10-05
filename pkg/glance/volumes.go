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
	corev1 "k8s.io/api/core/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
)

// GetVolumes - service volumes
func GetVolumes(name string, pvcName string, extraVol []storage.GlanceExtraVolMounts, svc []storage.ServiceType) []corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	var config0640AccessMode int32 = 0640
	g := storage.GlanceExtraVolMounts{}

	vm := []corev1.Volume{
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &scriptsVolumeDefaultMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-scripts",
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0640AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-config-data",
					},
				},
			},
		},
		{
			Name: "config-data-merged",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "lib-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
	return g.AppendVolume(vm, extraVol, svc)
}

// getInitVolumeMounts - general init task VolumeMounts
func getInitVolumeMounts(extraVol []storage.GlanceExtraVolMounts, svc []storage.ServiceType) []corev1.VolumeMount {
	g := storage.GlanceExtraVolMounts{}
	vm := []corev1.VolumeMount{
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/container-scripts",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/var/lib/config-data/default",
			ReadOnly:  true,
		},
		{
			Name:      "config-data-merged",
			MountPath: "/var/lib/config-data/merged",
			ReadOnly:  false,
		},
	}
	return g.AppendVolumeMount(vm, extraVol, svc)
}

// GetVolumeMounts - general VolumeMounts
func GetVolumeMounts(extraVol []storage.GlanceExtraVolMounts, svc []storage.ServiceType) []corev1.VolumeMount {
	g := storage.GlanceExtraVolMounts{}
	vm := []corev1.VolumeMount{
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/container-scripts",
			ReadOnly:  true,
		},
		{
			Name:      "config-data-merged",
			MountPath: "/var/lib/config-data/merged",
			ReadOnly:  false,
		},
		{
			Name:      "lib-data",
			MountPath: "/var/lib/glance",
			ReadOnly:  false,
		},
	}
	return g.AppendVolumeMount(vm, extraVol, svc)
}
