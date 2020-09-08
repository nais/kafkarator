// +versionName=v1
package kafka_nais_io_v1

import (
	"github.com/nais/kafkarator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EventRolloutComplete       = "RolloutComplete"
	EventFailedPrepare         = "FailedPrepare"
	EventFailedSynchronization = "FailedSynchronization"

	MaxServiceUserNameLength = 40
)

// +genclient
// +kubebuilder:object:root=true
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topic `json:"items"`
}

// +genclient
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Team",type="string",JSONPath=".metadata.labels.team"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.synchronizationState"
// +kubebuilder:object:root=true
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TopicSpec    `json:"spec"`
	Status            *TopicStatus `json:"status,omitempty"`
}

type Config struct {
	CleanupPolicy         *string `json:"cleanupPolicy,omitempty"`
	MinimumInSyncReplicas *int    `json:"minimumInSyncReplicas,omitempty"`
	Partitions            *int    `json:"partitions,omitempty"`
	Replication           *int    `json:"replication,omitempty"`
	RetentionBytes        *int    `json:"retentionBytes,omitempty"`
	RetentionHours        *int    `json:"retentionHours,omitempty"`
}

type TopicSpec struct {
	Pool   string    `json:"pool"`
	Config *Config   `json:"config,omitempty"`
	ACL    TopicACLs `json:"acl"`
}

type TopicStatus struct {
	SynchronizationState string   `json:"synchronizationState,omitempty"`
	SynchronizationHash  string   `json:"synchronizationHash,omitempty"`
	SynchronizationTime  string   `json:"synchronizationTime,omitempty"`
	Errors               []string `json:"errors,omitempty"`
	Message              string   `json:"message,omitempty"`
}

type TopicACLs []TopicACL

type TopicACL struct {
	// +kubebuilder:validation:Enum=read;write;readwrite
	Access      string `json:"access"`
	Application string `json:"application"`
	Team        string `json:"team"`
}

type User struct {
	Username    string
	Application string
	Team        string
}

func (in Topic) FullName() string {
	return in.Namespace + "." + in.Name
}

func (in TopicACL) Username() string {
	username := in.Team + "." + in.Application
	username, err := utils.ShortName(username, MaxServiceUserNameLength)
	if err != nil {
		panic(err)
	}
	return username
}

func (in TopicACL) User() User {
	return User{
		Username:    in.Username(),
		Application: in.Application,
		Team:        in.Team,
	}
}

func (in TopicACLs) Users() []User {
	users := make(map[User]interface{})
	result := make([]User, 0, len(in))
	for _, acl := range in {
		users[acl.User()] = new(interface{})
	}
	for k := range users {
		result = append(result, k)
	}
	return result
}

func init() {
	SchemeBuilder.Register(&Topic{}, &TopicList{})
}
