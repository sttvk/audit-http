package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterScanSpec defines the desired state of ClusterScan
type ClusterScanSpec struct {
	// Resources to be scanned (e.g., Pods)
	Resources []string `json:"resources,omitempty"`
	// Schedule in Cron format, empty if it is a one-off execution
	Schedule string `json:"schedule,omitempty"`
}

// ClusterScanStatus defines the observed state of ClusterScan
type ClusterScanStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastRunTime is the last time the scan was run.
	// +optional
	LastRunTime metav1.Time `json:"lastRunTime,omitempty"`

	// ResultMessage stores a simple message about the last scan, e.g., success or failure reason.
	ResultMessage string `json:"resultMessage,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterScan is the Schema for the clusterscans API
type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec,omitempty"`
	Status ClusterScanStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterScanList contains a list of ClusterScan
type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}
