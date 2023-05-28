// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/adykaaa/k8s-netpol-ctrl/watcher (interfaces: EventHandler)

// Package watcher is a generated GoMock package.
package watcher

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockEventHandler is a mock of EventHandler interface.
type MockEventHandler struct {
	ctrl     *gomock.Controller
	recorder *MockEventHandlerMockRecorder
}

// MockEventHandlerMockRecorder is the mock recorder for MockEventHandler.
type MockEventHandlerMockRecorder struct {
	mock *MockEventHandler
}

// NewMockEventHandler creates a new mock instance.
func NewMockEventHandler(ctrl *gomock.Controller) *MockEventHandler {
	mock := &MockEventHandler{ctrl: ctrl}
	mock.recorder = &MockEventHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventHandler) EXPECT() *MockEventHandlerMockRecorder {
	return m.recorder
}

// HandleAdd mocks base method.
func (m *MockEventHandler) HandleAdd(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleAdd", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleAdd indicates an expected call of HandleAdd.
func (mr *MockEventHandlerMockRecorder) HandleAdd(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleAdd", reflect.TypeOf((*MockEventHandler)(nil).HandleAdd), arg0)
}

// HandleDelete mocks base method.
func (m *MockEventHandler) HandleDelete(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleDelete", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleDelete indicates an expected call of HandleDelete.
func (mr *MockEventHandlerMockRecorder) HandleDelete(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleDelete", reflect.TypeOf((*MockEventHandler)(nil).HandleDelete), arg0)
}

// HandleUpdate mocks base method.
func (m *MockEventHandler) HandleUpdate(arg0, arg1 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleUpdate", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleUpdate indicates an expected call of HandleUpdate.
func (mr *MockEventHandlerMockRecorder) HandleUpdate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleUpdate", reflect.TypeOf((*MockEventHandler)(nil).HandleUpdate), arg0, arg1)
}