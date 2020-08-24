package kafka_nais_io_v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TopicSpec   `json:"spec"`
	Status            TopicStatus `json:"status"`
}

type TopicSpec struct {
	Pool   string            `json:"pool"`
	Config map[string]string `json:"config"`
	ACL    []TopicACL        `json:"acl"`
}

type TopicStatus struct {
	SynchronizationStatus string   `json:"synchronizationStatus"`
	SynchronizationHash   string   `json:"synchronizationHash"`
	SynchronizationTime   string   `json:"synchronizationTime"`
	Errors                []string `json:"errors"`
	Message               string   `json:"message"`
}

type TopicACL struct {
	// +kubebuilder:validation:Enum=read;write;readwrite
	Access string `json:"access"`
	Team   string `json:"team"`
}
