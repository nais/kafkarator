// Code generated by mockery v2.37.1. DO NOT EDIT.

package manager

import mock "github.com/stretchr/testify/mock"

// MockAclAdapter is an autogenerated mock type for the AclAdapter type
type MockAclAdapter struct {
	mock.Mock
}

// Create provides a mock function with given fields: project, service, acl
func (_m *MockAclAdapter) Create(project string, service string, acl Acl) (Acl, error) {
	ret := _m.Called(project, service, acl)

	var r0 Acl
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string, Acl) (Acl, error)); ok {
		return rf(project, service, acl)
	}
	if rf, ok := ret.Get(0).(func(string, string, Acl) Acl); ok {
		r0 = rf(project, service, acl)
	} else {
		r0 = ret.Get(0).(Acl)
	}

	if rf, ok := ret.Get(1).(func(string, string, Acl) error); ok {
		r1 = rf(project, service, acl)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: project, service, aclID
func (_m *MockAclAdapter) Delete(project string, service string, aclID string) error {
	ret := _m.Called(project, service, aclID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string) error); ok {
		r0 = rf(project, service, aclID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// List provides a mock function with given fields: project, service
func (_m *MockAclAdapter) List(project string, service string) ([]Acl, error) {
	ret := _m.Called(project, service)

	var r0 []Acl
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string) ([]Acl, error)); ok {
		return rf(project, service)
	}
	if rf, ok := ret.Get(0).(func(string, string) []Acl); ok {
		r0 = rf(project, service)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]Acl)
		}
	}

	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(project, service)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockAclAdapter creates a new instance of MockAclAdapter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAclAdapter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAclAdapter {
	mock := &MockAclAdapter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
