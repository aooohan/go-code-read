/*
Copyright 2013 Google Inc.

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

// Package lru implements an LRU cache.
package lru

import "container/list"

// Cache is an LRU cache. It is not safe for concurrent access.
// LRU cache 并发访问是不安全的
// 因此groupcache在cache struct中保证了并发访问安全
type Cache struct {
	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	// 能够缓存的entry的最大数量,如果数量大于MaxEntries，则会进行淘汰
	// 0代表无限制
	MaxEntries int

	// OnEvicted optionally specifies a callback function to be
	// executed when an entry is purged from the cache.
	// 可选，当有entry被淘汰时，执行这个callback
	OnEvicted func(key Key, value interface{})

	// 双向链表
	ll *list.List
	// map
	cache map[interface{}]*list.Element
}

// A Key may be any value that is comparable. See http://golang.org/ref/spec#Comparison_operators
type Key interface{}

// 也记录了key, 主要是方便后期淘汰清理map
type entry struct {
	key   Key
	value interface{}
}

// New creates a new Cache.
// If maxEntries is zero, the cache has no limit and it's assumed
// that eviction is done by the caller.
// 创建LRU 缓存，如果maxEntries为0，表示缓存没有限制，不会进行淘汰
func New(maxEntries int) *Cache {
	return &Cache{
		MaxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[interface{}]*list.Element),
	}
}

// Add adds a value to the cache.
func (c *Cache) Add(key Key, value interface{}) {
	// 懒初始化,省内存
	if c.cache == nil {
		c.cache = make(map[interface{}]*list.Element)
		c.ll = list.New()
	}
	if ee, ok := c.cache[key]; ok {
		// cache hit 就将当前entry移动到队头
		c.ll.MoveToFront(ee)
		// 重新赋值
		ee.Value.(*entry).value = value
		return
	}
	// 未命中，说明不存在，就创建一个entry放入cache中
	// 根据lru算法，所以新增也需要放到队头
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
	// 如果有限制，并且当前容量大于了maxEntries，需要将最近最近未使用的淘汰掉,也就是队尾的元素
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries {
		c.RemoveOldest()
	}
}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key Key) (value interface{}, ok bool) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		// cache hit
		// 根据LRU算法，将ele移动到队头
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key Key) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		// hit,移除ele
		c.removeElement(ele)
	}
}

// RemoveOldest removes the oldest item from the cache.
// 清除队尾item
func (c *Cache) RemoveOldest() {
	if c.cache == nil {
		return
	}
	// 获取队尾Element
	ele := c.ll.Back()
	if ele != nil {
		// 执行清除
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(e *list.Element) {
	// 将Element从双向链表中移除
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	// 清除map对应信息
	delete(c.cache, kv.key)
	if c.OnEvicted != nil {
		// 触发callback
		c.OnEvicted(kv.key, kv.value)
	}
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	if c.cache == nil {
		return 0
	}
	return c.ll.Len()
}

// Clear purges all stored items from the cache.
// 清除缓存中的所有item
func (c *Cache) Clear() {
	if c.OnEvicted != nil {
		// 触发callback
		for _, e := range c.cache {
			kv := e.Value.(*entry)
			c.OnEvicted(kv.key, kv.value)
		}
	}
	// 直接设置nil就可以，交给gc回收
	c.ll = nil
	c.cache = nil
}
