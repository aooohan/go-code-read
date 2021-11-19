/*
Copyright 2012 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package singleflight provides a duplicate function call suppression
// mechanism.
// singleflight 提供了防止并发时函数重复调用的机制
package singleflight

import "sync"

// call is an in-flight or completed Do call
// 表示一个正在进行的do调用或者是已经完成的do调用
type call struct {
	wg  sync.WaitGroup // 让其他相同的do调用等待
	val interface{}    // 存放结果
	err error          // 存放错误信息
}

// Group represents a class of work and forms a namespace in which
// units of work can be executed with duplicate suppression.
type Group struct {
	mu sync.Mutex // protects m
	// 存放调用信息
	m map[string]*call // lazily initialized
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
// 对于给定的key和function，确保同一时间，只能有一个fn正在执行的
// 其他重复调用者，需要等待第一个执行的完成，获取第一个执行的结果
// ps:
// 同一时间可能会有许多个相同key+fn的do，哪这里就是在保证，只有一个do调用fn
// 其余的do,等待第一个do的完成，并获取它的结果
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 先获取锁
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		// 有其他的call正在执行，则释放锁，等待其完成
		// 这里释放掉锁，原因在于mu是保护m的，你这时候有调用记录,
		// 并且当前goroutine不需要对m进行写操作，所以可以释放掉锁
		g.mu.Unlock()
		// 等待结果
		c.wg.Wait()
		// 获取结果, 返回
		return c.val, c.err
	}
	// 到这说明没有相同的fn在执行,则创建一个call
	// 那就在没有释放锁前，先让wg+1并设置调用信息，等释放锁后，好让其他相同的call，进行等待
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock() // 对于m的写操作完成了，释放锁

	// 执行fn,保存结果,并通知其他等待的goroutine
	c.val, c.err = fn()
	c.wg.Done()

	// 最后删掉，调用信息
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
