---
title: "分布式缓存中一致性 hash 算法原理解析与实现"
pubDate: "2019-11-22T15:34:42+08:00"
tags: ["分布式", "cache", "Go", "算法"]
---

分布式缓存服务中，提供缓存服务的节点可能有很多个。在单机缓存服务中，数据被缓存的流程是这样的：

第一次查询数据时首先从源数据查询（比如数据库），找到之后，同时放入缓存服务器中，下次查询同样的数据时会直接从缓存服务器上查找。

但是缓存服务器一般不太可能是单机的，往往有多个节点。转换为分布式之后，会出现一些问题。

## 问题一数据冗余

考虑一下，单机服务的时候，利用 LRU 算法实现缓存的存取，一个 key 对应一个数据 value。分布式条件下，如果只是单纯的增加节点，这次查找 key 对应的数据在 A 节点上，下次查找的时候却在 B 服务器上。同一个 key 有多个缓存，完全没必要嘛，这样就是数据冗余了。

怎么解决？

利用哈希。首先对 key 值 hash，然后利用节点数取余。

```
h = hash(key) %len(node)
```

这样同一个 key 的数据只会被一个节点缓存。很 awesome 有没有。

But，我不可能一直是这几个节点呀，万一有的节点挂了呢，或者我要添加节点呢？

## 问题二容错性和扩展性

如果有节点挂了或者新增节点，都会导致**len(node)** 的变化，那么利用 hash 计算出来的值跟之前的就不一样。这样导致新增或者删除一个节点，之前的所有缓存都失效了！我的天哪！！！

这种问题就是 **缓存雪崩** 。

怎么办呢？利用一致性 hash 算法。

> 一致性哈希算法（Consistent Hashing）最早在论文《[Consistent Hashing and Random Trees: Distributed Caching Protocols for Relieving Hot Spots on the World Wide Web](http://www.akamai.com/dl/technical_publications/ConsistenHashingandRandomTreesDistributedCachingprotocolsforrelievingHotSpotsontheworldwideweb.pdf)》中被提出。

它的原理是，把所有的 hash 值空间（就是上面公式计算出来的 h）看成是一个环，取值范围从 0 到 2^32-1。将每个服务器节点通过 hash 映射到环上，同时将数据 key 通过 hash 函数也映射到环上，按顺时针方向，数据 key 跟哪个节点近就属于哪个节点。

举个例子，现在有三个缓存服务器节点 2，4，6，假设这个 hash 算法就是原样输出，我们将节点和数据（1，3，7，9）经过 hash 之后到环上：

![一致性 hash](https://tva1.sinaimg.cn/large/0082zybply1gc53lsij1jj30b3079glv.jpg)

按顺时针方向，数据 1 属于 node2，数据 3 属于 node4，数据 7、9 输入 node6。

貌似看起来不错，但是还有个问题。在节点较少的情况上，会发生 **数据倾斜** 的问题。比如上图所示，数据可能大量的堆积在 node6 和 node2 之间。

解决办法是添加虚拟节点，利用虚拟节点负载均衡每个数据。虚拟节点的做法是，对一个真实节点计算多个 hash，放到环上，所有这些虚拟节点的数据都属于真实节点。

![一致性 hash2](https://tva1.sinaimg.cn/large/0082zybply1gc54o5rm70j30ax06xwf4.jpg)

这样所有的数据都均匀的分布在环上了。

## 算法实现

了解了原理，来动手实现一下一致性 hash 算法。整个算法模仿 go 语言的分布式缓存服务[groupcache](https://github.com/golang/groupcache) 实现，groupcache 可以说是**memcached** 的 go 语言实现。

首先定义一致性 hash 环结构体：

```go
type Hash func(data []byte) uint32

// ConHash 一致性 hash
type ConHash struct {
	hash     Hash           // hash 算法
	replicas int            // 虚拟节点数
	nodes    []int          // hash 环节点数
	hashMap  map[int]string // 虚拟节点 - 真实节点
}
```

可以看到，类型 Hash 就是个回调函数，用户可以自定义 hash 算法。

然后需要往 hash 环上添加节点，根据指定的虚拟节点数用 hash 算法放到环上。

```go
// Add 添加节点到 hash 环上
func (m *ConHash) Add(nodes ...string) {
	for _, node := range nodes {
		// 将节点值根据指定的虚拟节点数利用 hash 算法放置到环中
		for i := 0; i < m.replicas; i++ {
			h := int(m.hash([]byte(strconv.Itoa(i) + node)))
			m.nodes = append(m.nodes, h)
			// 映射虚拟节点到真实节点
			m.hashMap[h] = node
		}
	}
	sort.Ints(m.nodes)
}
```

同样还需要根据 key 值从环上获取对应的节点，获取到节点之后从该节点查找数据。

```go
// Get 从 hash 环上获取 key 对应的节点
func (m *ConHash) Get(key string) string {
	if len(m.nodes) == 0 {
		return ""
	}
	// 计算 key 的 hash 值
	h := int(m.hash([]byte(key)))
	// 顺时针找到第一个匹配的虚拟节点
	idx := sort.Search(len(m.nodes), func(i int) bool {
		return m.nodes[i] >= h
	})

	// 从 hash 环查找
	// 返回 hash 映射的真实节点
	return m.hashMap[m.nodes[idx%len(m.nodes)]]

}
```

有的人说不对啊，为啥添加的都是服务器节点，数据不是也放在环上吗？

其实是因为 groupcache 将数据划分出一个 group 的概念，数据在内部存储上利用 hash+ 双向链表实现，缓存的数据被放在链表中。

整个流程是这样的，查找 key 值对应的数据时，根据 url 链接中的 group 和 key 值确定节点，如何确定的？上面的代码已经解释了，计算 key 值的 hash，看它属于哪个节点。

然后从该节点的双向链表中查找。如果节点不存在这个 key，从用户定义的数据源查找（比如数据库），找到之后将数据存入该 group 中。

以上，希望有帮助。