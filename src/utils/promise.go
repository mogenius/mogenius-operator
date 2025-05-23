package utils

import (
	"sync"
)

type Promise interface {
	Wait() error
	RunSingle(execFunc func())
	RunArray(execFunc func())
}

type promise[T any] struct {
	wg       sync.WaitGroup
	resultCh chan T
}

func NewPromise[T any]() *promise[T] {
	self := &promise[T]{}
	self.resultCh = make(chan T)
	return self
}

func (self *promise[T]) Wait() []T {
	var result []T = []T{}
	go func() {
		self.wg.Wait()
		close(self.resultCh)
	}()

	for res := range self.resultCh {
		result = append(result, res)
	}
	return result
}

func (self *promise[T]) Run(execFunc func() *T) {
	self.wg.Add(1)
	go func() {
		defer self.wg.Done()
		result := execFunc()
		if result != nil {
			self.resultCh <- *result
		}
	}()
}

func (self *promise[T]) RunArray(execFunc func() *[]T) {
	self.wg.Add(1)
	go func() {
		defer self.wg.Done()
		result := execFunc()
		if result != nil {
			for _, res := range *result {
				self.resultCh <- res
			}
		}
	}()
}
