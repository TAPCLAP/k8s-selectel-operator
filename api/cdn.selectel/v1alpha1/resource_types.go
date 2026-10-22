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
	// accessKey is the AWS access key identifier.
	// +optional
	AccessKey *string `json:"access_key,omitempty"`
	// secretKey is the AWS secret access key.
	// +optional
	SecretKey *string `json:"secret_key,omitempty"`
}

// OriginAWS is AWS authorization for origin requests.
type OriginAWS struct {
	// auth holds the AWS authorization keys.
	// +optional
	Auth *AWSAuth `json:"auth,omitempty"`
}

// OriginServer is one origin behind a domain or IP key.
type OriginServer struct {
	// backup marks this origin as a backup.
	// +optional
	Backup *bool `json:"backup,omitempty"`
	// maxFails is the number of failed attempts used for load balancing.
	// +optional
	MaxFails *int32 `json:"max_fails,omitempty"`
	// port is the origin port. Selectel uses 80 without origin HTTPS and 443 with it.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
	// weight is the load-balancing weight.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// Origin is the content source.
type Origin struct {
	// aws is AWS authorization for origin requests.
	// +optional
	AWS *OriginAWS `json:"aws,omitempty"`
	// connectTimeout is the connect timeout, for example 5s.
	// +optional
	ConnectTimeout *string `json:"connect_timeout,omitempty"`
	// forwardHostHeader sends the Host header from the CDN request to the origin.
	// It cannot be set together with hostname.
	// +optional
	ForwardHostHeader *bool `json:"forward_host_header,omitempty"`
	// hostname is the Host header sent to the origin.
	// It cannot be set together with forwardHostHeader.
	// +optional
	Hostname *string `json:"hostname,omitempty"`
	// https uses HTTPS when the CDN contacts the origin.
	// +optional
	HTTPS *bool `json:"https,omitempty"`
	// readTimeout is the read timeout, for example 10s.
	// +optional
	ReadTimeout *string `json:"read_timeout,omitempty"`
	// s3Bucket is the allowed bucket when the origin is a shared S3 domain.
	// +optional
	S3Bucket *string `json:"s3_bucket,omitempty"`
	// sendTimeout is the send timeout, for example 10s.
	// +optional
	SendTimeout *string `json:"send_timeout,omitempty"`
	// servers lists origin servers. The key is a domain or IP.
	// +optional
	Servers map[string]OriginServer `json:"servers,omitempty"`
	// sniHostname overrides the name used for origin certificate verification and SNI.
	// It requires https to be true.
	// +optional
	SNIHostname *string `json:"sni_hostname,omitempty"`
	// sslVerify checks the origin certificate. It requires https to be true.
	// +optional
	SSLVerify *bool `json:"ssl_verify,omitempty"`
}

// AuthMD5 is signature authorization.
type AuthMD5 struct {
	// anywhere ignores the client IP address in the signature.
	// +optional
	Anywhere *bool `json:"anywhere,omitempty"`
	// forever applies no time limit to the signature.
	// +optional
	Forever *bool `json:"forever,omitempty"`
	// secret is the signature secret word.
	// +optional
	Secret *string `json:"secret,omitempty"`
}

// ResourceAuth is request authorization.
type ResourceAuth struct {
	// forbidden denies access when true.
	// +optional
	Forbidden *bool `json:"forbidden,omitempty"`
	// md5 is local signature authorization.
	// +optional
	MD5 *AuthMD5 `json:"md5,omitempty"`
	// url is the URL of an external authorization script.
	// +optional
	URL *string `json:"url,omitempty"`
}

// CacheValid is cache lifetime per status class. Values match ^(\d+)[smhd]$.
type CacheValid struct {
	// 2xx is how long successful responses stay in the CDN cache.
	// +optional
	XX2 *string `json:"2xx,omitempty"`
	// 3xx is how long redirects stay in the CDN cache.
	// +optional
	XX3 *string `json:"3xx,omitempty"`
	// 4xx is how long client errors stay in the CDN cache.
	// +optional
	XX4 *string `json:"4xx,omitempty"`
	// 5xx is how long server errors stay in the CDN cache.
	// +optional
	XX5 *string `json:"5xx,omitempty"`
	// browser is how long a browser may cache the response after receiving it from the CDN.
	// +optional
	Browser *string `json:"browser,omitempty"`
	// force makes the CDN ignore Cache-Control and Expires from the origin.
	// +optional
	Force *bool `json:"force,omitempty"`
}

// ResourceCache is cache behavior.
type ResourceCache struct {
	// argsBlacklist lists query parameters ignored when caching. It requires considerArgs.
	// +optional
	ArgsBlacklist []string `json:"args_blacklist,omitempty"`
	// argsWhitelist lists query parameters considered when caching. It requires considerArgs.
	// +optional
	ArgsWhitelist []string `json:"args_whitelist,omitempty"`
	// considerArgs includes the query string in the cache key.
	// +optional
	ConsiderArgs *bool `json:"consider_args,omitempty"`
	// considerCookies includes cookies in the cache key.
	// +optional
	ConsiderCookies *bool `json:"consider_cookies,omitempty"`
	// cookiesBlacklist lists cookies ignored when caching. It requires considerCookies.
	// +optional
	CookiesBlacklist []string `json:"cookies_blacklist,omitempty"`
	// cookiesWhitelist lists cookies considered when caching. It requires considerCookies.
	// +optional
	CookiesWhitelist []string `json:"cookies_whitelist,omitempty"`
	// disable turns caching off when true.
	// +optional
	Disable *bool `json:"disable,omitempty"`
	// staleConditions lists when the last cached copy is served. It requires useStale.
	// +optional
	StaleConditions []StaleCondition `json:"stale_conditions,omitempty"`
	// useStale serves the last cached copy when the origin is unavailable.
	// +optional
	UseStale *bool `json:"use_stale,omitempty"`
	// valid is the cache time for each response class.
	// +optional
	Valid *CacheValid `json:"valid,omitempty"`
}

// ResourceCompress is compression.
type ResourceCompress struct {
	// brotli compresses responses with Brotli.
	// +optional
	Brotli *bool `json:"brotli,omitempty"`
	// gzip compresses responses with Gzip.
	// +optional
	Gzip *bool `json:"gzip,omitempty"`
}

// ResourceCORS is CORS behavior.
type ResourceCORS struct {
	// always adds Access-Control-Allow-Origin for every status code.
	// False adds it only for specific status codes.
	// +optional
	Always *bool `json:"always,omitempty"`
	// credentials sets Access-Control-Allow-Credentials.
	// +optional
	Credentials *bool `json:"credentials,omitempty"`
	// disable turns CORS off when true.
	// +optional
	Disable *bool `json:"disable,omitempty"`
	// domains lists allowed origins.
	// +optional
	Domains []string `json:"domains,omitempty"`
	// expose lists headers available to the top-level API.
	// +optional
	Expose []string `json:"expose,omitempty"`
	// headers lists safe request headers.
	// +optional
	Headers []string `json:"headers,omitempty"`
	// maxAge is the preflight response lifetime in seconds.
	// +optional
	MaxAge *int32 `json:"max_age,omitempty"`
	// methods lists safe HTTP methods.
	// +optional
	Methods []string `json:"methods,omitempty"`
}

// ResourceHeaders adds or hides headers.
type ResourceHeaders struct {
	// hideInResponse lists origin headers the CDN omits from the client response.
	// +optional
	HideInResponse []string `json:"hide_in_response,omitempty"`
	// request adds headers to origin requests. The key is the header name.
	// +optional
	Request map[string]string `json:"request,omitempty"`
	// response adds headers to client responses. The key is the header name.
	// +optional
	Response map[string]string `json:"response,omitempty"`
}

// LimitationTime bounds a restriction.
type LimitationTime struct {
	// end is the end of the interval in ISO 8601.
	// +optional
	End *string `json:"end,omitempty"`
	// start is the start of the interval in ISO 8601.
	// +optional
	Start *string `json:"start,omitempty"`
}

// GeoExclude is a geographic exception.
type GeoExclude struct {
	// action is allow or deny for this exception.
	// +optional
	Action *AccessAction `json:"action,omitempty"`
	// country is an ISO 3166-1 alpha-2 country code.
	// +optional
	Country *string `json:"country,omitempty"`
	// region is an ISO 3166-2 region code.
	// +optional
	Region *string `json:"region,omitempty"`
}

// GeoLimitation restricts distribution by country.
type GeoLimitation struct {
	// defaultAction is allow or deny when no exception matches.
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// exclude lists geographic exceptions.
	// +optional
	Exclude []GeoExclude `json:"exclude,omitempty"`
	// times limits when this restriction applies.
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// IPExclude is an IP exception.
type IPExclude struct {
	// ip is an address in CIDR notation.
	// +optional
	IP *string `json:"ip,omitempty"`
}

// IPLimitation restricts distribution by IP.
type IPLimitation struct {
	// defaultAction is allow or deny when no exception matches.
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// exclude lists IP exceptions.
	// +optional
	Exclude []IPExclude `json:"exclude,omitempty"`
	// times limits when this restriction applies.
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// RefererExclude is a Referer exception.
type RefererExclude struct {
	// referer is a domain name or regular expression.
	// +optional
	Referer *string `json:"referer,omitempty"`
}

// RefererLimitation restricts distribution by Referer.
type RefererLimitation struct {
	// defaultAction is allow or deny when no exception matches.
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// exclude lists Referer exceptions.
	// +optional
	Exclude []RefererExclude `json:"exclude,omitempty"`
	// times limits when this restriction applies.
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// UserAgentExclude is a User-Agent exception.
type UserAgentExclude struct {
	// userAgent is a User-Agent value or regular expression.
	// +optional
	UserAgent *string `json:"useragent,omitempty"`
}

// UserAgentLimitation restricts distribution by User-Agent.
type UserAgentLimitation struct {
	// defaultAction is allow or deny when no exception matches.
	// +optional
	DefaultAction *AccessAction `json:"default_action,omitempty"`
	// exclude lists User-Agent exceptions.
	// +optional
	Exclude []UserAgentExclude `json:"exclude,omitempty"`
	// times limits when this restriction applies.
	// +optional
	Times []LimitationTime `json:"times,omitempty"`
}

// ResourceLimitations groups distribution restrictions.
type ResourceLimitations struct {
	// geo restricts distribution by country.
	// +optional
	Geo []GeoLimitation `json:"geo,omitempty"`
	// ip restricts distribution by IP address.
	// +optional
	IP []IPLimitation `json:"ip,omitempty"`
	// referer restricts distribution by Referer.
	// +optional
	Referer []RefererLimitation `json:"referer,omitempty"`
	// userAgent restricts distribution by User-Agent.
	// +optional
	UserAgent []UserAgentLimitation `json:"useragent,omitempty"`
}

// PackagingMP4 converts MP4 output.
type PackagingMP4 struct {
	// outputProtocols lists protocols used to deliver the converted video.
	// +optional
	OutputProtocols []OutputProtocol `json:"output_protocols,omitempty"`
}

// ResourcePackaging is video conversion.
type ResourcePackaging struct {
	// mp4 holds MPEG-4 conversion parameters.
	// +optional
	MP4 *PackagingMP4 `json:"mp4,omitempty"`
}

// ResourceReturn forces a response.
type ResourceReturn struct {
	// body is the forced response body.
	// +optional
	Body *string `json:"body,omitempty"`
	// httpStatusCode is the forced HTTP status code.
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	// +optional
	HTTPStatusCode *int32 `json:"http_status_code,omitempty"`
	// url is an absolute redirect URL for status codes 301, 302, 303, 307, and 308.
	// +optional
	URL *string `json:"url,omitempty"`
}

// ResourceRewrite rewrites the request path.
type ResourceRewrite struct {
	// flag stops further rewrite processing. The only value is break.
	// +optional
	Flag *RewriteFlag `json:"flag,omitempty"`
	// from is the regular expression matched against the source URL.
	// +optional
	From *string `json:"from,omitempty"`
	// scope runs the rewrite on edge nodes or on shielding nodes.
	// +optional
	Scope *RewriteScope `json:"scope,omitempty"`
	// to is the target address for the rewritten request.
	// +optional
	To *string `json:"to,omitempty"`
}

// ResourceRobots controls robots.txt.
type ResourceRobots struct {
	// robotsContent is the robots.txt body. It is used only when type is custom.
	// +optional
	RobotsContent *string `json:"robotsContent,omitempty"`
	// type is deny, custom, or cached.
	// +optional
	Type *RobotsType `json:"type,omitempty"`
}

// ResourceLocation is path-specific configuration.
type ResourceLocation struct {
	// allowedHttpMethods lists extra methods for this path. GET, HEAD, and OPTIONS are always allowed.
	// +optional
	AllowedHTTPMethods []HTTPMethod `json:"allowed_http_methods,omitempty"`
	// auth is request authorization for this path.
	// +optional
	Auth *ResourceAuth `json:"auth,omitempty"`
	// cache is cache behavior for this path.
	// +optional
	Cache *ResourceCache `json:"cache,omitempty"`
	// compress is compression for this path.
	// +optional
	Compress *ResourceCompress `json:"compress,omitempty"`
	// cors is CORS behavior for this path.
	// +optional
	CORS *ResourceCORS `json:"cors,omitempty"`
	// headers adds or hides headers for this path.
	// +optional
	Headers *ResourceHeaders `json:"headers,omitempty"`
	// ioss enables image optimization for this path.
	// +optional
	IOSS *bool `json:"ioss,omitempty"`
	// limitations restricts distribution for this path.
	// +optional
	Limitations *ResourceLimitations `json:"limitations,omitempty"`
	// order is the regular-expression check order for this location.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=999999
	// +optional
	Order *int32 `json:"order,omitempty"`
	// origin is the content source for this path.
	// +optional
	Origin *Origin `json:"origin,omitempty"`
	// packaging is video conversion for this path.
	// +optional
	Packaging *ResourcePackaging `json:"packaging,omitempty"`
	// return forces a response for this path.
	// +optional
	Return *ResourceReturn `json:"return,omitempty"`
	// rewrite rewrites the request path.
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

	// active enables the CDN resource.
	// +optional
	Active *bool `json:"active,omitempty"`
	// allowedHttpMethods lists extra methods. GET, HEAD, and OPTIONS are always allowed.
	// +optional
	AllowedHTTPMethods []HTTPMethod `json:"allowed_http_methods,omitempty"`
	// auth is request authorization.
	// +optional
	Auth *ResourceAuth `json:"auth,omitempty"`
	// cache is CDN cache behavior.
	// +optional
	Cache *ResourceCache `json:"cache,omitempty"`
	// compress is response compression.
	// +optional
	Compress *ResourceCompress `json:"compress,omitempty"`
	// cors is CORS behavior.
	// +optional
	CORS *ResourceCORS `json:"cors,omitempty"`
	// followRedirects follows redirects from the origin.
	// +optional
	FollowRedirects *bool `json:"follow_redirects,omitempty"`
	// headers adds or hides request and response headers.
	// +optional
	Headers *ResourceHeaders `json:"headers,omitempty"`
	// http2https redirects HTTP requests to HTTPS.
	// +optional
	HTTP2HTTPS *bool `json:"http2https,omitempty"`
	// httpsOnly serves the resource over HTTPS only.
	// +optional
	HTTPSOnly *bool `json:"https_only,omitempty"`
	// ioss enables image optimization and modification.
	// +optional
	IOSS *bool `json:"ioss,omitempty"`
	// limitations restricts distribution by geography, IP, Referer, or User-Agent.
	// +optional
	Limitations *ResourceLimitations `json:"limitations,omitempty"`
	// locations holds path-specific rules. The key is a path pattern such as /images or /video/*.
	// +optional
	Locations map[string]ResourceLocation `json:"locations,omitempty"`
	// modernTlsOnly allows only modern TLS versions.
	// +optional
	ModernTLSOnly *bool `json:"modern_tls_only,omitempty"`
	// name is the Selectel resource name. Empty means metadata.name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=`^[a-zA-Zа-яА-ЯёЁ0-9\-_ ]{1,50}$`
	// +optional
	Name string `json:"name,omitempty"`
	// names lists personal domains (CNAMEs) served by this CDN resource.
	// +optional
	Names []string `json:"names,omitempty"`
	// noHttp2 disables HTTP/2.
	// +optional
	NoHTTP2 *bool `json:"no_http2,omitempty"`
	// origin is the content source.
	// +optional
	Origin *Origin `json:"origin,omitempty"`
	// packaging converts video for delivery.
	// +optional
	Packaging *ResourcePackaging `json:"packaging,omitempty"`
	// return forces a response instead of origin content.
	// +optional
	Return *ResourceReturn `json:"return,omitempty"`
	// rewrite rewrites request paths.
	// +optional
	Rewrite []ResourceRewrite `json:"rewrite,omitempty"`
	// robots controls robots.txt.
	// +optional
	Robots *ResourceRobots `json:"robots,omitempty"`
	// sliceSizeMegabytes is the slice size in megabytes. It applies only when tuning is large.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1024
	// +optional
	SliceSizeMegabytes *int32 `json:"slice_size_megabytes,omitempty"`
	// sslProtocols lists allowed TLS versions.
	// +optional
	SSLProtocols []SSLProtocol `json:"ssl_protocols,omitempty"`
	// strongSslCiphers uses strong SSL ciphers. It requires modernTlsOnly.
	// +optional
	StrongSSLCiphers *bool `json:"strong_ssl_ciphers,omitempty"`
	// tuning selects distribution optimization: default, large, or live.
	// +optional
	Tuning *Tuning `json:"tuning,omitempty"`
	// useHttp3 enables HTTP/3.
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
