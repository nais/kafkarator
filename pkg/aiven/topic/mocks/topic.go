// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	aiven "github.com/aiven/aiven-go-client"
	mock "github.com/stretchr/testify/mock"
)

// Topic is an autogenerated mock type for the Topic type
type Topic struct {
	mock.Mock
}

// Create provides a mock function with given fields: project, service, req
func (_m *Topic) Create(project string, service string, req aiven.CreateKafkaTopicRequest) error {
	ret := _m.Called(project, service, req)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, aiven.CreateKafkaTopicRequest) error); ok {
		r0 = rf(project, service, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: project, service, _a2
func (_m *Topic) Get(project string, service string, _a2 string) (*aiven.KafkaTopic, error) {
	ret := _m.Called(project, service, _a2)

	var r0 *aiven.KafkaTopic
	if rf, ok := ret.Get(0).(func(string, string, string) *aiven.KafkaTopic); ok {
		r0 = rf(project, service, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*aiven.KafkaTopic)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(project, service, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: project, service, _a2, req
func (_m *Topic) Update(project string, service string, _a2 string, req aiven.UpdateKafkaTopicRequest) error {
	ret := _m.Called(project, service, _a2, req)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, aiven.UpdateKafkaTopicRequest) error); ok {
		r0 = rf(project, service, _a2, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}