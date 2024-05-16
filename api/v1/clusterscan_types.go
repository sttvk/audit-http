package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HTTPCheck defines the structure for HTTP checks for each pod
type HTTPCheck struct {
	PodName   string `json:"podName"`
	Namespace string `json:"namespace"`
	URL       string `json:"url"`
}

// ClusterScanSpec defines the desired state of ClusterScan
type ClusterScanSpec struct {
	HTTPChecks []HTTPCheck `json:"httpChecks,omitempty"`
	Schedule   string      `json:"schedule,omitempty"`
}

// ClusterScanStatus defines the observed state of ClusterScan
type ClusterScanStatus struct {
	LastRunTime    metav1.Time `json:"lastRunTime,omitempty"`
	ResultMessages []string    `json:"resultMessages,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterScan is the Schema for the clusterscans API
type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec,omitempty"`
	Status ClusterScanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterScanList contains a list of ClusterScan
type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}
