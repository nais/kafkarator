// +build !ignore_autogenerated

// Copyright 2020 NAV (Arbeids- og velferdsdirektoratet) - The Norwegian Labour
// and Welfare Administration
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
// of the Software, and to permit persons to whom the Software is furnished to do
// so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Code generated by controller-gen. DO NOT EDIT.

package kafka_nais_io_v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Config) DeepCopyInto(out *Config) {
	*out = *in
	if in.CleanupPolicy != nil {
		in, out := &in.CleanupPolicy, &out.CleanupPolicy
		*out = new(string)
		**out = **in
	}
	if in.MinimumInSyncReplicas != nil {
		in, out := &in.MinimumInSyncReplicas, &out.MinimumInSyncReplicas
		*out = new(int)
		**out = **in
	}
	if in.Partitions != nil {
		in, out := &in.Partitions, &out.Partitions
		*out = new(int)
		**out = **in
	}
	if in.Replication != nil {
		in, out := &in.Replication, &out.Replication
		*out = new(int)
		**out = **in
	}
	if in.RetentionBytes != nil {
		in, out := &in.RetentionBytes, &out.RetentionBytes
		*out = new(int)
		**out = **in
	}
	if in.RetentionHours != nil {
		in, out := &in.RetentionHours, &out.RetentionHours
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Config.
func (in *Config) DeepCopy() *Config {
	if in == nil {
		return nil
	}
	out := new(Config)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Topic) DeepCopyInto(out *Topic) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(TopicStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Topic.
func (in *Topic) DeepCopy() *Topic {
	if in == nil {
		return nil
	}
	out := new(Topic)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Topic) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicACL) DeepCopyInto(out *TopicACL) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicACL.
func (in *TopicACL) DeepCopy() *TopicACL {
	if in == nil {
		return nil
	}
	out := new(TopicACL)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in TopicACLs) DeepCopyInto(out *TopicACLs) {
	{
		in := &in
		*out = make(TopicACLs, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicACLs.
func (in TopicACLs) DeepCopy() TopicACLs {
	if in == nil {
		return nil
	}
	out := new(TopicACLs)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicList) DeepCopyInto(out *TopicList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Topic, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicList.
func (in *TopicList) DeepCopy() *TopicList {
	if in == nil {
		return nil
	}
	out := new(TopicList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TopicList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicSpec) DeepCopyInto(out *TopicSpec) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(Config)
		(*in).DeepCopyInto(*out)
	}
	if in.ACL != nil {
		in, out := &in.ACL, &out.ACL
		*out = make(TopicACLs, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicSpec.
func (in *TopicSpec) DeepCopy() *TopicSpec {
	if in == nil {
		return nil
	}
	out := new(TopicSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicStatus) DeepCopyInto(out *TopicStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicStatus.
func (in *TopicStatus) DeepCopy() *TopicStatus {
	if in == nil {
		return nil
	}
	out := new(TopicStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *User) DeepCopyInto(out *User) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new User.
func (in *User) DeepCopy() *User {
	if in == nil {
		return nil
	}
	out := new(User)
	in.DeepCopyInto(out)
	return out
}
