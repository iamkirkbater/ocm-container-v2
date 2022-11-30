// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/openshift/occ/cmd/run (interfaces: FileSystemRead)

// Package mocks is a generated GoMock package.
package mocks

import (
	fs "io/fs"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockFileSystemRead is a mock of FileSystemRead interface.
type MockFileSystemRead struct {
	ctrl     *gomock.Controller
	recorder *MockFileSystemReadMockRecorder
}

// MockFileSystemReadMockRecorder is the mock recorder for MockFileSystemRead.
type MockFileSystemReadMockRecorder struct {
	mock *MockFileSystemRead
}

// NewMockFileSystemRead creates a new mock instance.
func NewMockFileSystemRead(ctrl *gomock.Controller) *MockFileSystemRead {
	mock := &MockFileSystemRead{ctrl: ctrl}
	mock.recorder = &MockFileSystemReadMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFileSystemRead) EXPECT() *MockFileSystemReadMockRecorder {
	return m.recorder
}

// ReadDir mocks base method.
func (m *MockFileSystemRead) ReadDir(arg0 string) ([]fs.DirEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDir", arg0)
	ret0, _ := ret[0].([]fs.DirEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDir indicates an expected call of ReadDir.
func (mr *MockFileSystemReadMockRecorder) ReadDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDir", reflect.TypeOf((*MockFileSystemRead)(nil).ReadDir), arg0)
}

// Stat mocks base method.
func (m *MockFileSystemRead) Stat(arg0 string) (fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat", arg0)
	ret0, _ := ret[0].(fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat.
func (mr *MockFileSystemReadMockRecorder) Stat(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFileSystemRead)(nil).Stat), arg0)
}