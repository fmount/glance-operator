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
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
)

// CronJobType -
type CronJobType string

// PvcType -
type PvcType string

const (
	// ServiceName -
	ServiceName = "glance"
	// ServiceType -
	ServiceType = "image"
	// DatabaseName -
	DatabaseName = "glance"
	// Component -
	Component = "glance-api"
	// GlanceAPIName -
	GlanceAPIName = "glanceAPI"
	// PvcLocal for a generic glanceAPI instance
	PvcLocal PvcType = "local"
	// PvcCache is used to define a PVC mounted for image caching purposes
	PvcCache PvcType = "cache"

	// GlancePublicPort -
	GlancePublicPort int32 = 9292
	// GlanceInternalPort -
	GlanceInternalPort int32 = 9292

	// DefaultsConfigFileName -
	DefaultsConfigFileName = "00-config.conf"
	// CustomConfigFileName -
	CustomConfigFileName = "01-config.conf"
	// CustomServiceConfigFileName -
	CustomServiceConfigFileName = "02-config.conf"
	// CustomServiceConfigSecretsFileName -
	CustomServiceConfigSecretsFileName = "03-config.conf"

	// GlanceExtraVolTypeUndefined can be used to label an extraMount which
	// is not associated with a specific backend
	GlanceExtraVolTypeUndefined storage.ExtraVolType = "Undefined"
	// GlanceExtraVolTypeCeph can be used to label an extraMount which
	// is associated to a Ceph backend
	GlanceExtraVolTypeCeph storage.ExtraVolType = "Ceph"
	// GlanceAPI defines the glance-api group
	GlanceAPI storage.PropagationType = "GlanceAPI"
	// Glance is the global ServiceType that refers to all the components deployed
	// by the glance operator
	Glance storage.PropagationType = "Glance"
	// CinderName - Cinder CR Name Glance expects to find in the namespace
	CinderName = "cinder"
	// GlanceLogPath is the path used by GlanceAPI to stream/store its logs
	GlanceLogPath = "/var/log/glance/"
	// LogVolume is the default logVolume name used to mount logs on both
	// GlanceAPI and the sidecar container
	LogVolume = "logs"
	// KeystoneEndpoint - indicates whether the glanceAPI should register the
	// endpoints in keystone
	KeystoneEndpoint = "keystoneEndpoint"

	//DBPurge -
	DBPurge CronJobType = "purge"
	//CacheCleaner -
	CacheCleaner CronJobType = "cleaner"
	//CachePruner -
	CachePruner CronJobType = "pruner"
	//CacheCleanerDefaultSchedule -
	CacheCleanerDefaultSchedule = "1 0 * * *"
	//CachePrunerDefaultSchedule -
	CachePrunerDefaultSchedule = "*/30 * * * *"
	//ImageCacheDir -
	ImageCacheDir = "/var/lib/glance/image-cache"
)

// DBPurgeCommandBase -
var DBPurgeCommandBase = [...]string{"/usr/bin/glance-manage", "--debug", "--config-dir /etc/glance/glance.conf.d", "db purge "}

// CacheCleanerCommandBase -
var CacheCleanerCommandBase = [...]string{"/usr/bin/glance-cache-cleaner", "--debug", "--config-dir /etc/glance/glance.conf.d"}

// CachePrunerCommandBase -
var CachePrunerCommandBase = [...]string{"/usr/bin/glance-cache-pruner", "--debug", "--config-dir /etc/glance/glance.conf.d"}

// DbsyncPropagation keeps track of the DBSync Service Propagation Type
var DbsyncPropagation = []storage.PropagationType{storage.DBSync}

// GlanceAPIPropagation is the  definition of the GlanceAPI propagation group
// It allows the GlanceAPI pod to mount volumes destined to Glance related
// ServiceTypes
var GlanceAPIPropagation = []storage.PropagationType{Glance, GlanceAPI}
