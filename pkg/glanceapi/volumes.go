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
	corev1 "k8s.io/api/core/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
)

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
