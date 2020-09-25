---
title: "Lock-Free 是什么"
date: 2020-09-24T16:15:04+08:00
draft: false
tags: ["并发", "算法"]
---

什么是Lock-Free？

在并发访问某个资源的实现中，经常利用锁机制来保证对资源的正确访问。但是锁机制的问题在于会出先死锁、活锁或者线程调度优先级被抢占等问题，同时锁的增加和释放都会消耗时间，导致性能问题。

Lock-Free指的是不通过锁机制来保证资源的并发访问。也就是说线程间不会相互阻塞了。

![](https://shiniao.fun/images/lockfree.png)

实现Lock-Free常见的解决办法是利用CAS操作，CAS是啥？

CAS（Compare and Swap）是一种原子操作，原子很好理解，不可分割（比如原子事务），原子操作意味着CPU在操作内存时（读写）要么一次完成，要么失败，不会出现只完成一部分的现象。现代CPU对原子的读写操作都有相应的支持，比如X86/64架构就通过CAS的方式来实现，而ARM通过LL/SC（Load-Link/Store-Conditional）来实现。

在Go语言中，可通过 atomic 包中的 CompareAndSwap** 方法来编程实现CAS：

```go
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)
```

使用CAS的过程中有一个问题，考虑如下状况：

如果线程1读取共享内存地址得到A，这时候线程2抢占线程1，将A的值修改为B，然后又改回A，线程1再次读取得到A，虽然结果相同，但是A已经被修改过了，这个就是**ABA问题**。

一种办法是通过类似版本号的方式来解决，每次更新的时候 **counter+1**，比如对于上面的问题，在线程2修改的时候，因为增加了版本号，导致修改前后的A值并不相同：

```bash
1A--2B--3A
```

在论文[《 Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms》](https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf) 中，描述了一种Lock-Free 队列的实现，通过 counter 机制解决了CAS中的ABA问题，并且给出了详细的伪代码实现，可查看论文中的详细介绍。

Lock-Free常用来实现底层的数据结构，比如队列、栈等，本文比较了使用锁机制的队列实现和参考上述论文的Lock-Free队列实现，两种实现的性能测试结果如下图所示：

![性能比较](https://shiniao.fun/images/benchmark.png)

可以看到，队列的Lock-Free算法稳定在200ns/op，性能更佳，而使用锁的算法要高出一倍。

> 代码实现参考：
>
> https://github.com/rilkee/distributed/queue/



参考文献：

1. http://preshing.com/20120612/an-introduction-to-lock-free-programming/
2. Michael, M. M., & Scott, M. L. (1996). Simple, fast, and practical non-blocking and blocking concurrent queue algorithms. Proceedings of the Annual ACM Symposium on Principles of Distributed Computing, 267–275. https://doi.org/10.1145/248052.248106