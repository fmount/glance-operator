package glance

import (
	corev1 "k8s.io/api/core/v1"
)

// SecurityContext - currently used to make sure we don't run db-sync as
// root user
func SecurityContext() *corev1.SecurityContext {
	trueVal := true
	falseVal := false
	runAsUser := int64(GlanceUID)
	runAsGroup := int64(GlanceGID)

	return &corev1.SecurityContext{
		RunAsUser:                &runAsUser,
		RunAsGroup:               &runAsGroup,
		RunAsNonRoot:             &trueVal,
		AllowPrivilegeEscalation: &falseVal,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"MKNOD",
			},
		},
	}
}

// HttpdSecurityContext -
func HttpdSecurityContext() *corev1.SecurityContext {
	runAsUser := int64(0)
	return &corev1.SecurityContext{
		RunAsUser: &runAsUser,
	}
}

// APISecurityContext -
func APISecurityContext(privileged bool) *corev1.SecurityContext {
	runAsUser := int64(0)
	runAsGroup := int64(0)
	return &corev1.SecurityContext{
		RunAsUser:  &runAsUser,
		RunAsGroup: &runAsGroup,
		Privileged: &privileged,
	}
}
