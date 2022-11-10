// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/openshift/occ/cmd/run (interfaces: Copier)

// Package mocks is a generated GoMock package.
package mocks

import (
	io "io"
	reflect "reflect"

	copier "github.com/containers/buildah/copier"
	gomock "github.com/golang/mock/gomock"
)

// MockCopier is a mock of Copier interface.
type MockCopier struct {
	ctrl     *gomock.Controller
	recorder *MockCopierMockRecorder
}

// MockCopierMockRecorder is the mock recorder for MockCopier.
type MockCopierMockRecorder struct {
	mock *MockCopier
}

// NewMockCopier creates a new mock instance.
func NewMockCopier(ctrl *gomock.Controller) *MockCopier {
	mock := &MockCopier{ctrl: ctrl}
	mock.recorder = &MockCopierMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCopier) EXPECT() *MockCopierMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockCopier) Get(arg0, arg1 string, arg2 copier.GetOptions, arg3 []string, arg4 io.Writer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockCopierMockRecorder) Get(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockCopier)(nil).Get), arg0, arg1, arg2, arg3, arg4)
}