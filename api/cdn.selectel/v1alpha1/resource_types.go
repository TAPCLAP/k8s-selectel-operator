/*
Copyright 2026.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceFinalizer is set until the Selectel CDN resource is deactivated or orphaned.
	ResourceFinalizer = "cdn.selectel.tapclap.com/resource"

	// ConditionReady reports whether the CDN resource matches the spec.
	ConditionReady = "Ready"
)

// DeletionPolicy controls finalizer behavior. The CDN API has no delete operation.
// +kubebuilder:validation:Enum=Deactivate;Orphan
type DeletionPolicy string

const (
	// DeletionPolicyDeactivate sends active=false and then drops the finalizer.
	DeletionPolicyDeactivate DeletionPolicy = "Deactivate"
	// DeletionPolicyOrphan drops the finalizer and leaves the Selectel resource unchanged.
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// ManagementPolicy chooses whether the operator changes the Selectel resource.
// +kubebuilder:validation:Enum=Observe;Manage
type ManagementPolicy string

const (
	// ManagementPolicyObserve links the resource and reports drift without calling the mutating CDN API.
	ManagementPolicyObserve ManagementPolicy = "Observe"
	// ManagementPolicyManage creates and updates the Selectel resource to match the spec.
	ManagementPolicyManage ManagementPolicy = "Manage"
)

// ResourcePhase is the operator's view of the Selectel resource.
// +kubebuilder:validation:Enum=Pending;Processing;Ready;Error;Drift
type ResourcePhase string

const (
	PhasePending    ResourcePhase = "Pending"
	PhaseProcessing ResourcePhase = "Processing"
	PhaseReady      ResourcePhase = "Ready"
	PhaseError      ResourcePhase = "Error"
	PhaseDrift      ResourcePhase = "Drift"
)

// FieldDrift is one spec field Manage would send because it differs from Selectel.
type FieldDrift struct {
	// path is the JSON path inside the CDN resource body.
	Path string `json:"path"`
	// desired is the JSON value from the spec, truncated.
	// +optional
	Desired string `json:"desired,omitempty"`
	// actual is the JSON value from Selectel, truncated.
	// +optional
	Actual string `json:"actual,omitempty"`
}

// ProjectRef names a cluster-scoped Project.
type ProjectRef struct {
	// name of the Project.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
}

// CertificateSource selects the certificate bound to the CDN resource.
// Only id is accepted today. A future Certificate object will add a sibling ref field.
type CertificateSource struct {
	// id is the Selectel certificate seq. It must be listed on the Project for this namespace.
	// +kubebuilder:validation:Minimum=1
	// +optional
	ID *int32 `json:"id,omitempty"`
}

// HTTPMethod is an extra method allowed besides GET, HEAD, and OPTIONS.
// +kubebuilder:validation:Enum=POST;PUT;PATCH;DELETE;MKCOL;COPY;MOVE;PROPFIND;PROPPATCH;LOCK;UNLOCK
type HTTPMethod string

// SSLProtocol is a TLS version the CDN edge may use.
// +kubebuilder:validation:Enum=TLSv1;TLSv1.1;TLSv1.2;TLSv1.3;SSLv3
type SSLProtocol string

// Tuning selects distribution optimization.
// +kubebuilder:validation:Enum=default;large;live
type Tuning string

// AccessAction is allow or deny.
// +kubebuilder:validation:Enum=allow;deny
type AccessAction string

// StaleCondition is a reason to serve stale cache.
// +kubebuilder:validation:Enum=error;timeout;invalid_header;updating;http_500;http_502;http_503;http_504;http_403;http_404;http_429
type StaleCondition string

// RobotsType controls robots.txt handling.
// +kubebuilder:validation:Enum=deny;custom;cached
type RobotsType string

// RewriteFlag stops further rewrite processing.
// +kubebuilder:validation:Enum=break
type RewriteFlag string

// RewriteScope is where a rewrite runs.
// +kubebuilder:validation:Enum=edge;shield
type RewriteScope string

// OutputProtocol is a packaged video protocol.
// +kubebuilder:validation:Enum=MPEG-DASH;HLS
type OutputProtocol string

// AWSAuth holds origin credentials for AWS-style authorization.
type AWSAuth struct {
	// +optional
	AccessKey *string `json:"access_key,omitempty"`
	// +optional
	SecretKey *string `json:"secret_key,omitempty"`
}

// OriginAWS is AWS authorization for origin requests.
type OriginAWS struct {
	// +optional
	Auth *AWSAuth `json:"auth,omitempty"`
}

// OriginServer is one origin behind a domain or IP key.
type OriginServer struct {
	// +optional
	Backup *bool `json:"backup,omitempty"`
	// +optional
	MaxFails *int32 `json:"max_fails,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// Origin is the content source.
type Origin struct {
	// +optional
	AWS *OriginAWS `json:"aws,omitempty"`
	// +optional
	ConnectTimeout *string `json:"connect_timeout,omitempty"`
	// +optional
	ForwardHostHeader *bool `json:"forward_host_header,omitempty"`
	// +optional
	Hostname *string `json:"hostname,omitempty"`
	// +optional
	HTTPS *bool `json:"https,omitempty"`
	// +optional
	ReadTimeout *string `json:"read_timeout,omitempty"`
	// +optional
	S3Bucket *string `json:"s3_bucket,omitempty"`
	// +optional
	SendTimeout *string `json:"send_timeout,omitempty"`
	// +optional
	Servers map[string]OriginServer `json:"servers,omitempty"`
	// +optional
	SNIHostname *string `json:"sni_hostname,omitempty"`
	// +optional
	SSLVerify *bool `json:"ssl_verify,omitempty"`
}

// AuthMD5 is signature authorization.
type AuthMD5 struct {
	// +optional
	Anywhere *bool `json:"anywhere,omitempty"`
	// +optional
	Forever *bool `json:"forever,omitempty"`
	// +optional
	Secret *string `json:"secret,omitempty"`
}

// ResourceAuth is request authorization.
type ResourceAuth struct {
	// +optional
	Forbidden *bool `json:"forbidden,omitempty"`
	// +optional
	MD5 *AuthMD5 `json:"md5,omitempty"`
	// +optional
	URL *string `json:"url,omitempty"`
}

// CacheValid is cache lifetime per status class. Values match ^(\d+)[smhd]$.
type CacheValid struct {
	// +optional
	XX2 *string `json:"2xx,omitempty"`
	// +optional
	XX3 *string `json:"3xx,omitempty"`
	// +optional
	XX4 *string `json:"4xx,omitempty"`
	// +optional
	XX5 *string `json:"5xx,omitempty"`
	// +optional
	Browser *string `json:"browser,omitempty"`
	// +optional
	Force *bool `json:"force,omitempty"`
}

// ResourceCache is cache behavior.
type ResourceCache struct {
	// +optional
	ArgsBlacklist []string `json:"args_blacklist,omitempty"`
	// +optional
	ArgsWhitelist []string `json:"args_whitelist,omitempty"`
	// +optional
	ConsiderArgs *bool `json:"consider_args,omitempty"`
	// +optional
	ConsiderCookies *bool `json:"consider_cookies,omitempty"`
	// +optional
	CookiesBlacklist []string `json:"cookies_blacklist,omitempty"`
	// +optional
	CookiesWhitelist []string `json:"cookies_whitelist,omitempty"`
	// +optional
	Disable *bool `json:"disable,omitempty"`
	// +optional
	StaleConditions []StaleCondition `json:"stale_conditions,omitempty"`
	// +optional
	UseStale *bool `json:"use_stale,omitempty"`
	// +optional
	Valid *CacheValid `json:"valid,omitempty"`
}

// ResourceCompress is compression.
type ResourceCompress struct {
	// +optional
	Brotli *bool `json:"brotli,omitempty"`
	// +optional
	Gzip *bool `json:"gzip,omitempty"`
}

// ResourceCORS is CORS behavior.
type ResourceCORS struct {
	// +optional
	Always *bool `json:"always,omitempty"`
	// +optional
	Credentials *bool `json:"credentials,omitempty"`
	// +optional
	Disable *bool `json:"disable,omitempty"`
	// +optional
	Domains []string `json:"domains,omitempty"`
	// +optional
	Expose []string `json:"expose,omitempty"`
	// +optional
	Headers []string `json:"headers,omitempty"`
	// +optional
	MaxAge *int32 `json:"max_age,omitempty"`
	// +optional
	Methods []string `json:"methods,omitempty"`
}

// ResourceHeaders adds or hides headers.
type ResourceHeaders struct {
	// +optional
	HideInResponse []string `json:"hide_in_response,omitempty"`
	// +optional
	Request map[string]string `json:"request,omitempty"`
	// +optional
	Response map[string]string `json:"response,omitempty"`
}

// LimitationTime bounds a restriction.
type LimitationTime struct {
	// +optional
	End *string `json:"end,omitempty"`
	// +optional
	Start *string `json:"start,omitempty"`
}

// GeoExclude is a geographic exception.
type GeoExclude struct {
	// +optional
	Action *AccessAction `json:"action,omitempty"`
	// +optional
	Country *string `json:"country,omitempty"`
	// +optional
	Region *string `json:"region,omitempty"`
}

// GeoLimitation restricts distribution by country.
type GeoLimitation struct {
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// +optional
	Exclude []GeoExclude `json:"exclude,omitempty"`
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// IPExclude is an IP exception.
type IPExclude struct {
	// +optional
	IP *string `json:"ip,omitempty"`
}

// IPLimitation restricts distribution by IP.
type IPLimitation struct {
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// +optional
	Exclude []IPExclude `json:"exclude,omitempty"`
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// RefererExclude is a Referer exception.
type RefererExclude struct {
	// +optional
	Referer *string `json:"referer,omitempty"`
}

// RefererLimitation restricts distribution by Referer.
type RefererLimitation struct {
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// +optional
	Exclude []RefererExclude `json:"exclude,omitempty"`
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// UserAgentExclude is a User-Agent exception.
type UserAgentExclude struct {
	// +optional
	UserAgent *string `json:"useragent,omitempty"`
}

// UserAgentLimitation restricts distribution by User-Agent.
type UserAgentLimitation struct {
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// +optional
	Exclude []UserAgentExclude `json:"exclude,omitempty"`
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// ResourceLimitations groups distribution restrictions.
type ResourceLimitations struct {
	// +optional
	Geo []GeoLimitation `json:"geo,omitempty"`
	// +optional
	IP []IPLimitation `json:"ip,omitempty"`
	// +optional
	Referer []RefererLimitation `json:"referer,omitempty"`
	// +optional
	UserAgent []UserAgentLimitation `json:"useragent,omitempty"`
}

// PackagingMP4 converts MP4 output.
type PackagingMP4 struct {
	// +optional
	OutputProtocols []OutputProtocol `json:"output_protocols,omitempty"`
}

// ResourcePackaging is video conversion.
type ResourcePackaging struct {
	// +optional
	MP4 *PackagingMP4 `json:"mp4,omitempty"`
}

// ResourceReturn forces a response.
type ResourceReturn struct {
	// +optional
	Body *string `json:"body,omitempty"`
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	// +optional
	HTTPStatusCode *int32 `json:"http_status_code,omitempty"`
	// +optional
	URL *string `json:"url,omitempty"`
}

// ResourceRewrite rewrites the request path.
type ResourceRewrite struct {
	// +optional
	Flag *RewriteFlag `json:"flag,omitempty"`
	// +optional
	From *string `json:"from,omitempty"`
	// +optional
	Scope *RewriteScope `json:"scope,omitempty"`
	// +optional
	To *string `json:"to,omitempty"`
}

// ResourceRobots controls robots.txt.
type ResourceRobots struct {
	// +optional
	RobotsContent *string `json:"robotsContent,omitempty"`
	// +optional
	Type *RobotsType `json:"type,omitempty"`
}

// ResourceLocation is path-specific configuration.
type ResourceLocation struct {
	// +optional
	AllowedHTTPMethods []HTTPMethod `json:"allowed_http_methods,omitempty"`
	// +optional
	Auth *ResourceAuth `json:"auth,omitempty"`
	// +optional
	Cache *ResourceCache `json:"cache,omitempty"`
	// +optional
	Compress *ResourceCompress `json:"compress,omitempty"`
	// +optional
	CORS *ResourceCORS `json:"cors,omitempty"`
	// +optional
	Headers *ResourceHeaders `json:"headers,omitempty"`
	// +optional
	IOSS *bool `json:"ioss,omitempty"`
	// +optional
	Limitations *ResourceLimitations `json:"limitations,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=999999
	// +optional
	Order *int32 `json:"order,omitempty"`
	// +optional
	Origin *Origin `json:"origin,omitempty"`
	// +optional
	Packaging *ResourcePackaging `json:"packaging,omitempty"`
	// +optional
	Return *ResourceReturn `json:"return,omitempty"`
	// +optional
	Rewrite []ResourceRewrite `json:"rewrite,omitempty"`
}

// ResourceSpec is the desired Selectel CDN resource plus operator-only fields.
// Selectel fields use the CDN API JSON names. projectRef, deletionPolicy, managementPolicy,
// resourceID, and the certificate object are operator fields and are not sent as-is.
// +kubebuilder:validation:XValidation:rule="!has(self.certificate) || has(self.certificate.id)",message="certificate.id is required when certificate is set"
type ResourceSpec struct {
	// projectRef is the cluster-scoped Project. The namespace is not part of the reference.
	// +required
	ProjectRef ProjectRef `json:"projectRef"`

	// deletionPolicy is Deactivate by default. Orphan leaves the Selectel resource in place.
	// +kubebuilder:default=Deactivate
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`

	// managementPolicy is Manage by default.
	// Observe links an existing resource by resourceID or name and reports drift.
	// Manage creates and updates the Selectel resource.
	// +kubebuilder:default=Manage
	// +optional
	ManagementPolicy ManagementPolicy `json:"managementPolicy,omitempty"`

	// resourceID adopts an existing Selectel resource instead of creating one.
	// The operator writes the created UUID to status.id and does not fill this field.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`

	// certificate binds a Selectel certificate seq allowed on the Project for this namespace.
	// Omit it to serve the default *.selcdn.net certificate.
	// +optional
	Certificate *CertificateSource `json:"certificate,omitempty"`

	// +optional
	Active *bool `json:"active,omitempty"`
	// +optional
	AllowedHTTPMethods []HTTPMethod `json:"allowed_http_methods,omitempty"`
	// +optional
	Auth *ResourceAuth `json:"auth,omitempty"`
	// +optional
	Cache *ResourceCache `json:"cache,omitempty"`
	// +optional
	Compress *ResourceCompress `json:"compress,omitempty"`
	// +optional
	CORS *ResourceCORS `json:"cors,omitempty"`
	// +optional
	FollowRedirects *bool `json:"follow_redirects,omitempty"`
	// +optional
	Headers *ResourceHeaders `json:"headers,omitempty"`
	// +optional
	HTTP2HTTPS *bool `json:"http2https,omitempty"`
	// +optional
	HTTPSOnly *bool `json:"https_only,omitempty"`
	// +optional
	IOSS *bool `json:"ioss,omitempty"`
	// +optional
	Limitations *ResourceLimitations `json:"limitations,omitempty"`
	// +optional
	Locations map[string]ResourceLocation `json:"locations,omitempty"`
	// +optional
	ModernTLSOnly *bool `json:"modern_tls_only,omitempty"`
	// name is the Selectel resource name. Empty means metadata.name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=`^[a-zA-Zа-яА-ЯёЁ0-9\-_ ]{1,50}$`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Names []string `json:"names,omitempty"`
	// +optional
	NoHTTP2 *bool `json:"no_http2,omitempty"`
	// +optional
	Origin *Origin `json:"origin,omitempty"`
	// +optional
	Packaging *ResourcePackaging `json:"packaging,omitempty"`
	// +optional
	Return *ResourceReturn `json:"return,omitempty"`
	// +optional
	Rewrite []ResourceRewrite `json:"rewrite,omitempty"`
	// +optional
	Robots *ResourceRobots `json:"robots,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1024
	// +optional
	SliceSizeMegabytes *int32 `json:"slice_size_megabytes,omitempty"`
	// +optional
	SSLProtocols []SSLProtocol `json:"ssl_protocols,omitempty"`
	// +optional
	StrongSSLCiphers *bool `json:"strong_ssl_ciphers,omitempty"`
	// +optional
	Tuning *Tuning `json:"tuning,omitempty"`
	// +optional
	UseHTTP3 *bool `json:"use_http3,omitempty"`
}

// ResourceStatus is the observed Selectel CDN resource.
type ResourceStatus struct {
	// id is the Selectel resource UUID.
	// +optional
	ID string `json:"id,omitempty"`

	// cdnDomain is the *.selcdn.net domain assigned by Selectel.
	// +optional
	CDNDomain string `json:"cdnDomain,omitempty"`

	// creationTimestamp is when Selectel created the resource.
	// +optional
	CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty"`

	// phase is Pending, Processing, Ready, Error, or Drift.
	// +optional
	Phase ResourcePhase `json:"phase,omitempty"`

	// taskID is the last Selectel configuration task id.
	// +optional
	TaskID string `json:"taskID,omitempty"`

	// drift lists spec fields Manage would update. Empty when the spec matches Selectel.
	// +optional
	Drift []FieldDrift `json:"drift,omitempty"`

	// conditions represent the current state of the Resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Policy",type=string,JSONPath=`.spec.managementPolicy`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.status.cdnDomain`
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.id`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Resource is a Selectel CDN resource.
type Resource struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Resource
	// +required
	Spec ResourceSpec `json:"spec"`

	// status defines the observed state of Resource
	// +optional
	Status ResourceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ResourceList contains a list of Resource.
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
