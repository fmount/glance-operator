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
	"path"
	"strconv"
	"strings"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// datadirSetupScriptName is the entry in the "glance-scripts" Secret that
// calls the datadir-setup script, which is rendered from
// templates/common/bin/datadir-setup.sh by the top-level Glance controller and
// mounted into the API pods via the "scripts" volume.
const datadirSetupScriptName = "datadir-setup.sh"

// DatadirInitContainers returns the init container that runs when the NFS (file)
// backend is detected.
func DatadirInitContainers(
	instance *glancev1.GlanceAPI,
	dataMounts []corev1.VolumeMount,
) ([]corev1.Container, error) {
	datadir := glance.GetFilesystemStoreDatadir(instance.Spec.CustomServiceConfig)
	mount, err := datadirMount(datadir, dataMounts)
	if err != nil {
		return nil, err
	}
	mounts := append([]corev1.VolumeMount{mount}, glance.GetScriptVolumeMount()...)

	return []corev1.Container{
		{
			Name:    glance.ServiceName + "-datadir-setup",
			Image:   instance.Spec.ContainerImage,
			Command: []string{path.Join(glance.ScriptMountPath, datadirSetupScriptName)},
			Args: []string{
				datadir,
				strconv.FormatInt(users.GlanceUID, 10),
				strconv.FormatInt(users.GlanceGID, 10),
			},
			SecurityContext: &corev1.SecurityContext{
				// Changing ownership on an NFS export is privileged on the NFS
				// server, and uid 0 is the only identity a client can present
				// that the server may accept for it. Scoped to this init
				// container: the API containers stay unprivileged as 42415.
				RunAsUser:    ptr.To(int64(0)),
				RunAsGroup:   ptr.To(int64(0)),
				RunAsNonRoot: ptr.To(false),
				// Capabilities are deliberately left at the runtime default
				// rather than Drop:[ALL] plus Add:[CHOWN]. chown(2) needs
				// CAP_CHOWN, and the anyuid-family SCCs set allowedCapabilities:
				// null, so a dropped capability could not be re-added without
				// the pod being refused at admission.
				AllowPrivilegeEscalation: ptr.To(false),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			VolumeMounts: mounts,
			Resources:    instance.Spec.Resources,
		},
	}, nil
}

// datadirMount returns the most specific writable data mount the datadir lives
// on, so the init container prepares the directory at the same path, and on
// the same volume, that glance-api will later write images to. Nothing else
// the API mounts is given to the root init container.
func datadirMount(datadir string, dataMounts []corev1.VolumeMount) (corev1.VolumeMount, error) {
	datadirPath := path.Clean(datadir)
	var selected corev1.VolumeMount
	selectedPath := ""
	for _, mount := range dataMounts {
		mountPath := path.Clean(mount.MountPath)
		if mount.ReadOnly || !pathContains(mountPath, datadirPath) ||
			(selectedPath != "" && len(mountPath) <= len(selectedPath)) {
			continue
		}
		selected, selectedPath = mount, mountPath
	}
	if selectedPath == "" {
		return selected, fmt.Errorf("filesystem_store_datadir %q is not on a writable image store volume", datadir)
	}
	return selected, nil
}

// pathContains reports whether target is dir itself or lives below it. Both
// paths are expected to be cleaned already.
func pathContains(dir string, target string) bool {
	if dir == target {
		return true
	}
	if !strings.HasSuffix(dir, "/") {
		// path.Clean only keeps a trailing slash for the root directory
		dir += "/"
	}
	return strings.HasPrefix(target, dir)
}
