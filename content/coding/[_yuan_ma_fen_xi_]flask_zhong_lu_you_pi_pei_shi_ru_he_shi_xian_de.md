---
title: "[源码分析] Flask中路由匹配是如何实现的"
date: 2019-07-04T11:19:00+08:00
draft: false
tags: ["Python", "Flask"]
---

首先让我们来了解下WSGI规范是啥？

简单来说，WSGI是服务器和应用之间的接口，前端过来的请求传到服务器之后比如gunicorn，之后服务器会将请求转发给应用。因为有很多个服务器，如果我们为我们的应用根据不同的服务写不同的代码，会很麻烦，所以就出现了WSGI。

WSGI规定了application应该实现一个可调用的对象（函数，类，方法或者带`__call__`的实例），这个对象应该接受两个位置参数：

1. 环境变量（比如header信息，状态码等）
2. 回调函数（WSGI服务器负责），用来发送http状态和header等

同时，该对象需要返回可迭代的响应文本。

更具体的解释可以去google搜索相关知识。

一个最简单的实现：

```python
def app(environ, start_response):
    response_body = b"Hello, World!"
    status = "200 OK"
    # 将响应状态和header交给WSGI服务器比如gunicorn
    start_response(status, headers=[])
    return iter([response_body])
```

我们可以直接使用gunicorn之类的服务启动这个app。

有了WSGI规定，框架中就要实现规范中所要求的部分。我们来看看Flask是如何实现的。

Flask0.1版本的实现中只有一个文件，一共600多行代码。根据官方文档，一个最简单的web服务像这样：

```python
from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello_world():
    return 'Hello World!'

if __name__ == '__main__':
    app.run()
```

调用 **Flask()** 之后发生了什么？

首先在 `__init__` 内置方法中有这么几个变量：

```python
class Flask(object):
    def __init__(self, package_name):
        # view_functions存储视图函数名称和视图函数
        self.view_functions = {}
        # 路由字典
        self.url_map = Map()
```

根据名字可以猜测，**view_functions** 用来存放视图函数，**url_map** 用来存放路由字典。暂时跳过，来看看 `__call__`内置方法：

```python
def __call__(self, environ, start_response):
        return self.wsgi_app(environ, start_response)
```

environ，start_response，是不是在哪里见过？WSGI规范中要求实现的对不对。它返回了 **wsgi_app** 方法：

```python
def wsgi_app(self, environ, start_response):
        with self.request_context(environ):
            rv = self.preprocess_request()
            if rv is None:
                rv = self.dispatch_request()
            response = self.make_response(rv)
            response = self.process_response(response)
            return response(environ, start_response)
```

看到了没，跟我们上面实现的那个简单的app是不是很像。

首先预处理请求，然后分发请求到不同的视图函数，最后响应。

我们先来看 **dispatch_request** 是如何实现的：

```python
def dispatch_request(self):
    	# 精简了下代码
        try:
            endpoint, values = self.match_request()
            return self.view_functions[endpoint](**values)
        except HTTPException, e:
            ......
            
def match_request(self):
        rv = _request_ctx_stack.top.url_adapter.match()
        request.endpoint, request.view_args = rv
        return rv
```

**dispatch_request** 首先获取endpoint和一些变量，然后在视图函数字典里找到对应的视图函数返回。endpoint和values就是我们在定义路由的处理函数时，比如：

```python
url_for('profile', username='John Doe')
```

其中profile就是endpoint，也就是对应视图函数的名称，username就是变量。

**match_request** 中这个 **_request_ctx_stack** 又是个啥。看起来它像是用来匹配路由的。

**_request_ctx_stack** 是请求上下文栈，用一个栈把当前请求相关的数据压入栈中，然后进行路由分发和后续处理，处理完成后退出。

具体来说，我们回过头看 **wsgi_app**方法中有个with语句，控制请求上下文的进入和退出。

```python
with self.request_context(environ):
```

这个 **request_context**是这样的：

```python
class _RequestContext(object):
    def __init__(self, app, environ):
		...
        self.url_adapter = app.url_map.bind_to_environ(environ)
        ...

    def __enter__(self):
        _request_ctx_stack.push(self)

    def __exit__(self, exc_type, exc_value, tb):
        if tb is None or not self.app.debug:
            _request_ctx_stack.pop()
```

其中的 **url_adapter** 获取了路由字典，然后连同其他变量一起被压入栈中，这样在上面的 **match_request** 方法中，从栈中获取 **url_adapter** ， 然后匹配路由找到对应的endpoint和参数，然后根据endpoint和参数从**view_functions** 中查找对应的视图函数。

```python
self.url_adapter = app.url_map.bind_to_environ(environ)
rv = _request_ctx_stack.top.url_adapter.match()
```

其中，**bind_to_environ** 将 url绑定到目前的环境返回一个适配器，然后适配器去匹配请求。这两个方法都来自Flask的底层调用 **werkzeug** 。

梳理一下流程，首先适配器在 url_map中查找当前路由对应的endpoint和values，然后dispatch_request根据endpoint找到对应的视图函数，然后返回。

那么，url_map中的路由和endpoint对应关系是从哪里来的？

我们在使用Flask是不是要用装饰器给视图函数加上路由和方法对吧，像这样：

```python
@app.route('/')
def hello_world():
    return 'Hello World!'
```

这个route装饰器长这样，可以看到它调用了add_url_rule方法。

```python
def route(self, rule, **options):
    def decorator(f):
            self.add_url_rule(rule, f.__name__, **options)
            self.view_functions[f.__name__] = f
            return f
        return decorator
    
def add_url_rule(self, rule, endpoint, **options):
    options['endpoint'] = endpoint
    options.setdefault('methods', ('GET',))
    self.url_map.add(Rule(rule, **options))
```

add_url_rule添加了路由和 endpoint 到 url_map 中，这样一个请求的路由过来后，**url_adapter.match()** 就能匹配到对应的 endpoint ，然后根据 endpoint 从 view_functions 里面查找视图函数。

url_map 是werkzeug中的Map对象，然后添加的是Rule对象。它看起来像这样：

```python
self.url_map = Map([
    Rule('/', endpoint='home'),
    Rule('/book/<id>', endpoint='book')
])
```

除了利用装饰器，我们也可以这样使用：

```python
def index():
    pass
app.add_url_rule('index', '/')
app.view_functions['index'] = index
```

现在，一切都解释清楚了，定义好视图函数后，**app.run**运行即可。

可以看到，Flask中路由匹配是利用字典实现的，还有一种利用前缀树来实现路由的，比如go语言中的gin框架，关于如何用前缀树实现路由可以看我的另一篇文章：

>  [前缀树算法实现路由匹配原理解析](https://shiniao.fun/posts/前缀树算法实现路由匹配原理解析)

