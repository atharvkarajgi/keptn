// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package nats_mock

import (
	"github.com/keptn/keptn/shipyard-controller/models"
	"sync"
)

// IKeptnNatsMessageHandlerMock is a mock implementation of nats.IKeptnNatsMessageHandler.
//
// 	func TestSomethingThatUsesIKeptnNatsMessageHandler(t *testing.T) {
//
// 		// make and configure a mocked nats.IKeptnNatsMessageHandler
// 		mockedIKeptnNatsMessageHandler := &IKeptnNatsMessageHandlerMock{
// 			ProcessFunc: func(event models.Event, sync bool) error {
// 				panic("mock out the Process method")
// 			},
// 		}
//
// 		// use mockedIKeptnNatsMessageHandler in code that requires nats.IKeptnNatsMessageHandler
// 		// and then make assertions.
//
// 	}
type IKeptnNatsMessageHandlerMock struct {
	// ProcessFunc mocks the Process method.
	ProcessFunc func(event models.Event, sync bool) error

	// calls tracks calls to the methods.
	calls struct {
		// Process holds details about calls to the Process method.
		Process []struct {
			// Event is the event argument value.
			Event models.Event
			// Sync is the sync argument value.
			Sync bool
		}
	}
	lockProcess sync.RWMutex
}

// Process calls ProcessFunc.
func (mock *IKeptnNatsMessageHandlerMock) Process(event models.Event, sync bool) error {
	if mock.ProcessFunc == nil {
		panic("IKeptnNatsMessageHandlerMock.ProcessFunc: method is nil but IKeptnNatsMessageHandler.Process was just called")
	}
	callInfo := struct {
		Event models.Event
		Sync  bool
	}{
		Event: event,
		Sync:  sync,
	}
	mock.lockProcess.Lock()
	mock.calls.Process = append(mock.calls.Process, callInfo)
	mock.lockProcess.Unlock()
	return mock.ProcessFunc(event, sync)
}

// ProcessCalls gets all the calls that were made to Process.
// Check the length with:
//     len(mockedIKeptnNatsMessageHandler.ProcessCalls())
func (mock *IKeptnNatsMessageHandlerMock) ProcessCalls() []struct {
	Event models.Event
	Sync  bool
} {
	var calls []struct {
		Event models.Event
		Sync  bool
	}
	mock.lockProcess.RLock()
	calls = mock.calls.Process
	mock.lockProcess.RUnlock()
	return calls
}