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
	"github.com/openstack-k8s-operators/glance-operator/internal/glance"
	"github.com/openstack-k8s-operators/lib-common/modules/users"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// datadirSetupScript makes the file backend image store directory writable by
// the glance user, and refuses to let the pod start if it cannot. It runs as
// uid 0.
//
// The kubelet does not apply fsGroup ownership management to NFS volumes -- the
// NFS volume plugin reports Managed=false, so SetVolumeOwnership is never
// called -- which means an export created as root:root 0755 is not writable by
// uid 42415. A PVC is block backed and does get fsGroup applied, so this is a
// no-op there.
//
// The chown can only help on an export mounted no_root_squash, because it is
// issued as uid 0 and root_squash maps that to the anonymous user. A
// root_squash export whose ownership was prepared correctly server-side is
// already fine and must not be touched, so the writability check runs first and
// skips the chown entirely.
//
// Failing has to stop the pod: glance_store evaluates WRITE_ACCESS once, in
// configure_add(), and the filesystem driver does not override
// update_capabilities(), so an API that starts against an unwritable datadir
// stays unable to accept uploads for the lifetime of the process while still
// reporting healthy, rejecting every upload with StoreAddDisabled.
const datadirSetupScript = `set -u
DATADIR="%[1]s"
WANT_UID=%[2]d
WANT_GID=%[3]d

# Whether uid WANT_UID could create entries in the directory, which needs both
# write and execute. Evaluated from the mode rather than with test -w, because
# this container runs as root and root bypasses the permission check.
glance_can_write() {
    local st uid gid mode u g o
    st="$(stat -c '%%u %%g %%a' "$1")" || return 1
    uid="${st%%%% *}"; st="${st#* }"
    gid="${st%%%% *}"; mode="${st##* }"
    # keep the low three octal digits, dropping any setuid/setgid/sticky prefix
    mode="00${mode}"; mode="${mode: -3}"
    u="${mode:0:1}"; g="${mode:1:1}"; o="${mode:2:1}"
    [ "${uid}" = "${WANT_UID}" ] && [ $(( u & 3 )) -eq 3 ] && return 0
    [ "${gid}" = "${WANT_GID}" ] && [ $(( g & 3 )) -eq 3 ] && return 0
    [ $(( o & 3 )) -eq 3 ] && return 0
    return 1
}

# Last resort before failing: ask the kernel as the glance user. This catches a
# directory made writable by an ACL, which glance_can_write cannot see. Only
# ever consulted to pass, never to fail, so a missing or uncooperative su can
# never block a working deployment.
glance_can_write_acl() {
    command -v su >/dev/null 2>&1 || return 1
    su -s /bin/sh glance -c 'test -w "$1"' sh "$1" >/dev/null 2>&1
}

fail() {
    echo "ERROR: ${DATADIR} is not writable by ${WANT_UID}:${WANT_GID}." >&2
    echo "  ls -ld: $(ls -ld "${DATADIR}" 2>&1)" >&2
    echo "  The kubelet does not apply fsGroup to NFS volumes, so the export must be" >&2
    echo "  owned by ${WANT_UID}:${WANT_GID}, or group writable by ${WANT_GID}, on the" >&2
    echo "  NFS server. Under root_squash this cannot be fixed from the client." >&2
    exit 1
}

if [ ! -d "${DATADIR}" ]; then
    echo "Creating ${DATADIR}"
    if ! mkdir -p "${DATADIR}"; then
        echo "ERROR: could not create ${DATADIR}." >&2
        echo "  Under root_squash our uid 0 is mapped to the anonymous user; create" >&2
        echo "  the directory owned by ${WANT_UID}:${WANT_GID} on the NFS server." >&2
        exit 1
    fi
fi

if glance_can_write "${DATADIR}"; then
    echo "${DATADIR} is already writable by ${WANT_UID}:${WANT_GID} ($(stat -c '%%U:%%G %%A' "${DATADIR}")), nothing to do"
    exit 0
fi

echo "Setting ownership of ${DATADIR} from $(stat -c '%%u:%%g' "${DATADIR}") to ${WANT_UID}:${WANT_GID}"
# Deliberately non-recursive. On a populated image store a recursive chown over
# NFS is expensive, and images written by a correctly configured glance are
# already owned by ${WANT_UID}.
if ! chown "${WANT_UID}:${WANT_GID}" "${DATADIR}"; then
    echo "WARNING: chown of ${DATADIR} failed; most likely a root_squash export." >&2
    glance_can_write_acl "${DATADIR}" || fail
    echo "${DATADIR} is writable by ${WANT_UID} via an ACL, continuing"
    exit 0
fi

# chown can report success while the server squashed it to the anonymous uid,
# so re-check rather than trusting the exit status.
if ! glance_can_write "${DATADIR}"; then
    glance_can_write_acl "${DATADIR}" || fail
fi

echo "${DATADIR} is now owned by $(stat -c '%%u:%%g' "${DATADIR}")"
`

// DatadirInitContainers returns the init container that makes the file backend
// image store usable before the API starts.
//
// volumeMounts must be the same set the glance-api container gets, so the init
// container resolves the datadir to exactly the mount the API will use.
//
// This is an init container rather than a separate Job on purpose. The kubelet
// completes volume SetUp before running any container in the pod, so an init
// container sees the very mount the API container will use, in the pod that is
// about to serve. A Job runs against its own mount of the share, which works
// for NFS -- ownership is state on the NFS server and persists across pods --
// but cannot see a StatefulSet VolumeClaimTemplate PVC at all, and cannot
// re-verify anything on a later pod restart.
func DatadirInitContainers(
	instance *glancev1.GlanceAPI,
	volumeMounts []corev1.VolumeMount,
) []corev1.Container {
	datadir := glancev1.GetFilesystemStoreDatadir(instance.Spec.CustomServiceConfig)

	return []corev1.Container{
		{
			Name:    glance.ServiceName + "-datadir-setup",
			Image:   instance.Spec.ContainerImage,
			Command: []string{"/bin/bash"},
			Args: []string{
				"-c",
				fmt.Sprintf(datadirSetupScript, datadir, users.GlanceUID, users.GlanceGID),
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
			VolumeMounts: volumeMounts,
			Resources:    instance.Spec.Resources,
		},
	}
}
