---
title: "对Python并发编程的思考"
date: 2018-01-11T22:02:53+08:00
draft: false
tags: ["Python"]
---

为了提高系统密集型运算的效率，我们常常会使用到多个进程或者是多个线程，python中的`Threading`包实现了线程，`multiprocessing` 包则实现了多进程。而在3.2版本的python中，将进程与线程进一步封装成`concurrent.futures` 这个包，使用起来更加方便。我们以请求网络服务为例，来实际测试一下加入多线程之后的效果。

首先来看看不使用多线程花费的时间：

```python
import time
import requests

NUMBERS = range(12)
URL = 'http://httpbin.org/get?a={}'

# 获取网络请求结果
def fetch(a):
    r = requests.get(URL.format(a))
    return r.json()['args']['a']

# 开始时间
start = time.time()

for num in NUMBERS:
    result = fetch(num)
    print('fetch({}) = {}'.format(num, result))
# 计算花费的时间
print('cost time: {}'.format(time.time() - start))

```

执行结果如下：

```bash
fetch(0) = 0
fetch(1) = 1
fetch(2) = 2
fetch(3) = 3
fetch(4) = 4
fetch(5) = 5
fetch(6) = 6
fetch(7) = 7
fetch(8) = 8
fetch(9) = 9
fetch(10) = 10
fetch(11) = 11
cost time: 6.952988862991333
```

再来看看加入多线程之后的效果：

```python
import time
import requests
from concurrent.futures import ThreadPoolExecutor

NUMBERS = range(12)
URL = 'http://httpbin.org/get?a={}'

def fetch(a):
    r = requests.get(URL.format(a))
    return r.json()['args']['a']

start = time.time()
# 使用线程池（使用5个线程）
with ThreadPoolExecutor(max_workers=5) as executor:
  # 此处的map操作与原生的map函数功能一样
    for num, result in zip(NUMBERS, executor.map(fetch, NUMBERS)):
        print('fetch({}) = {}'.format(num, result))
print('cost time: {}'.format(time.time() - start))
```

执行结果如下：

```bash
fetch(0) = 0
fetch(1) = 1
fetch(2) = 2
fetch(3) = 3
fetch(4) = 4
fetch(5) = 5
fetch(6) = 6
fetch(7) = 7
fetch(8) = 8
fetch(9) = 9
fetch(10) = 10
fetch(11) = 11
cost time: 1.9467740058898926
```

只用了近2秒的时间，如果再多加几个线程时间会更短，而不加入多线程需要接近7秒的时间。

不是说python中由于全局解释锁的存在，每次只能执行一个线程吗，为什么上面使用多线程还快一些？

确实，由于python的解释器（只有cpython解释器中存在这个问题）本身不是线程安全的，所以存在着全局解释锁，也就是我们经常听到的GIL，导致一次只能使用一个线程来执行Python的字节码。但是对于上面的I/O操作来说，一个线程在等待网络响应时，执行I/O操作的函数会释放GIL，然后再运行一个线程。

所以，执行I/O密集型操作时，多线程是有用的，对于CPU密集型操作，则每次只能使用一个线程。那这样说来，想执行CPU密集型操作怎么办？

答案是使用多进程，使用concurrent.futures包中的`ProcessPoolExecutor` 。这个模块实现的是真正的并行计算，因为它使用ProcessPoolExecutor 类把工作分配给多个 Python 进程处理。因此，如果需要做 CPU密集型处理，使用这个模块能绕开 GIL，利用所有可用的 CPU 核心。

说到这里，对于I/O密集型，可以使用多线程或者多进程来提高效率。我们上面的并发请求数只有5个，但是如果同时有1万个并发操作，像淘宝这类的网站同时并发请求数可以达到千万级以上，服务器每次为一个请求开一个线程，还要进行上下文切换，这样的开销会很大，服务器压根承受不住。一个解决办法是采用分布式，大公司有钱有力，能买很多的服务器，小公司呢。

我们知道系统开进程的个数是有限的，线程的出现就是为了解决这个问题，于是在进程之下又分出多个线程。所以有人就提出了能不能用**同一线程来同时处理若干连接**，再往下分一级。于是**协程**就出现了。

> 协程在实现上试图用一组少量的线程来实现多个任务，一旦某个任务阻塞，则可能用同一线程继续运行其他任务，避免大量上下文的切换，而且，各个协程之间的切换，往往是用户通过代码来显式指定的，不需要系统参与，可以很方便的实现异步。

协程本质上是异步非阻塞技术，它是将事件回调进行了包装，让程序员看不到里面的事件循环。说到这里，什么是异步非阻塞？同步异步，阻塞，非阻塞有什么区别？

借用知乎上的一个例子，假如你打电话问书店老板有没有《分布式系统》这本书，如果是同步通信机制，书店老板会说，你稍等，”我查一下”，然后开始查啊查，等查好了（可能是5秒，也可能是一天）告诉你结果（返回结果）。而异步通信机制，书店老板直接告诉你我查一下啊，查好了打电话给你，然后直接挂电话了（不返回结果）。然后查好了，他会主动打电话给你。在这里老板通过“回电”这种方式来回调。

而阻塞与非阻塞则是你打电话问书店老板有没有《分布式系统》这本书，你如果是阻塞式调用，你会一直把自己“挂起”，直到得到这本书有没有的结果，如果是非阻塞式调用，你不管老板有没有告诉你，你自己先一边去玩了， 当然你也要偶尔过几分钟check一下老板有没有返回结果。在这里阻塞与非阻塞与是否同步异步无关。跟老板通过什么方式回答你结果无关。

总之一句话，阻塞和非阻塞，描述的是一种状态，而同步与非同步描述的是行为方式。

回到协程上。

类似于`Threading` 包是对线程的实现一样，python3.4之后加入的`asyncio` 包则是对协程的实现。我们用asyncio改写文章开头的代码，看看使用协程之后能花费多少时间。

```python
import asyncio
import aiohttp
import time

NUMBERS = range(12)
URL = 'http://httpbin.org/get?a={}'
# 这里的代码不理解没关系
# 主要是为了证明协程的强大
async def fetch_async(a):
    async with aiohttp.request('GET', URL.format(a)) as r:
        data = await r.json()
    return data['args']['a']

start = time.time()
loop = asyncio.get_event_loop()
tasks = [fetch_async(num) for num in NUMBERS]
results = loop.run_until_complete(asyncio.gather(*tasks))

for num, results in zip(NUMBERS, results):
    print('fetch({}) = ()'.format(num, results))

print('cost time: {}'.format(time.time() - start))
```

执行结果：

```bash
fetch(0) = ()
fetch(1) = ()
fetch(2) = ()
fetch(3) = ()
fetch(4) = ()
fetch(5) = ()
fetch(6) = ()
fetch(7) = ()
fetch(8) = ()
fetch(9) = ()
fetch(10) = ()
fetch(11) = ()
cost time: 0.8582110404968262
```

不到一秒！感受到协程的威力了吧。

asyncio的知识说实在的有点难懂，因为它是用异步的方式在编写代码。上面给出的asyncio示例不理解也没有关系，之后的文章会详细的介绍一些asyncio相关的概念。