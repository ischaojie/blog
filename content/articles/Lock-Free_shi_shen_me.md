+++
title = "Lock Free是什么"
date = 2020-08-28T15:17:05+08:00
draft = false
[taxonomies]
tags = ["分布式", "算法"]
+++

什么是Lock-Free？

在并发访问某个资源的实现中，经常利用锁机制来保证对资源的正确访问。但是锁机制的问题在于会出先死锁、活锁或者线程调度优先级被抢占等问题，同时锁的增加和释放都会消耗时间，导致性能问题。

Lock-Free指的是不通过锁机制来保证资源的并发访问。也就是说线程间不会相互阻塞了。

![lock-free](https://shiniao.fun/images/lockfree.png))

实现Lock-Free常见的解决办法是利用CAS操作，CAS是啥？

CAS（Compare and Swap）是一种原子操作，原子很好理解，不可分割（比如原子事务），原子操作意味着CPU在操作内存时（读写）要么一次完成，要么失败，不会出现只完成一部分的现象。现代CPU对原子的读写操作都有相应的支持，比如X86/64架构就通过CAS的方式来实现，而ARM通过LL/SC（Load-Link/Store-Conditional）来实现。

在Go语言中，可通过 atomic 包中的 CompareAndSwap** 方法来编程实现CAS：

```go
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)
```

使用CAS的过程中有一个问题，考虑如下状况：

如果线程1读取共享内存地址得到A，这时候线程2抢占线程1，将A的值修改为B，然后又改回A，线程1再次读取得到A，虽然结果相同，但是A已经被修改过了，这个就是**ABA问题**。

一种办法是通过类似版本号的方式来解决，每次更新的时候 counter+1，比如对于上面的问题，在线程2修改的时候，因为增加了版本号，导致修改前后的A值并不相同：

```
1A--2B--3A
```

在论文[《 Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms》](https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf) 中，描述了一种利用CAS的Lock-Free 队列的实现，通过 **counter 机制**解决了CAS中的ABA问题，并且给出了详细的伪代码实现，可查看论文中的详细介绍。

```
structure pointer_t {ptr: pointer to node_t, count: unsigned integer}
 structure node_t {value: data type, next: pointer_t}
 structure queue_t {Head: pointer_t, Tail: pointer_t}
 
 initialize(Q: pointer to queue_t)
    node = new_node()		// Allocate a free node
    node->next.ptr = NULL	// Make it the only node in the linked list
    Q->Head.ptr = Q->Tail.ptr = node	// Both Head and Tail point to it
 
 enqueue(Q: pointer to queue_t, value: data type)
  E1:   node = new_node()	// Allocate a new node from the free list
  E2:   node->value = value	// Copy enqueued value into node
  E3:   node->next.ptr = NULL	// Set next pointer of node to NULL
  E4:   loop			// Keep trying until Enqueue is done
  E5:      tail = Q->Tail	// Read Tail.ptr and Tail.count together
  E6:      next = tail.ptr->next	// Read next ptr and count fields together
  E7:      if tail == Q->Tail	// Are tail and next consistent?
              // Was Tail pointing to the last node?
  E8:         if next.ptr == NULL
                 // Try to link node at the end of the linked list
  E9:            if CAS(&tail.ptr->next, next, <node, next.count+1>)
 E10:               break	// Enqueue is done.  Exit loop
 E11:            endif
 E12:         else		// Tail was not pointing to the last node
                 // Try to swing Tail to the next node
 E13:            CAS(&Q->Tail, tail, <next.ptr, tail.count+1>)
 E14:         endif
 E15:      endif
 E16:   endloop
        // Enqueue is done.  Try to swing Tail to the inserted node
 E17:   CAS(&Q->Tail, tail, <node, tail.count+1>)
 
 dequeue(Q: pointer to queue_t, pvalue: pointer to data type): boolean
  D1:   loop			     // Keep trying until Dequeue is done
  D2:      head = Q->Head	     // Read Head
  D3:      tail = Q->Tail	     // Read Tail
  D4:      next = head.ptr->next    // Read Head.ptr->next
  D5:      if head == Q->Head	     // Are head, tail, and next consistent?
  D6:         if head.ptr == tail.ptr // Is queue empty or Tail falling behind?
  D7:            if next.ptr == NULL  // Is queue empty?
  D8:               return FALSE      // Queue is empty, couldn't dequeue
  D9:            endif
                 // Tail is falling behind.  Try to advance it
 D10:            CAS(&Q->Tail, tail, <next.ptr, tail.count+1>)
 D11:         else		     // No need to deal with Tail
                 // Read value before CAS
                 // Otherwise, another dequeue might free the next node
 D12:            *pvalue = next.ptr->value
                 // Try to swing Head to the next node
 D13:            if CAS(&Q->Head, head, <next.ptr, head.count+1>)
 D14:               break             // Dequeue is done.  Exit loop
 D15:            endif
 D16:         endif
 D17:      endif
 D18:   endloop
 D19:   free(head.ptr)		     // It is safe now to free the old node
 D20:   return TRUE                   // Queue was not empty, dequeue succeeded
```

除此之外，该论文还给出了一种two-lock的并发队列实现，通过在Head和Tail分别添加锁，来保证入队和出队的完全并发操作。

Lock-Free常用来实现底层的数据结构，比如队列、栈等，本文比较了使用单锁机制的队列实现和参考上述论文的Lock-Free队列实现，在 1<<12 个节点的出队入队中，两种算法实现的性能测试结果如下图所示：

![性能测试](https://shiniao.fun/images/benchmark.png)

可以看到，随着处理器个数的增加，队列的Lock-Free算法一直稳定在200ns/op，性能更佳，而使用锁的算法耗时要高出一倍。

> 代码实现参考：
>
> https://github.com/rilkee/distributed/queue/



参考文献：

1. http://preshing.com/20120612/an-introduction-to-lock-free-programming/
2. Michael, M. M., & Scott, M. L. (1996). Simple, fast, and practical non-blocking and blocking concurrent queue algorithms. Proceedings of the Annual ACM Symposium on Principles of Distributed Computing, 267–275. https://doi.org/10.1145/248052.248106