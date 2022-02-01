---
title: "[源码分析] Flask蓝图实现原理分析"
date: 2019-07-01T11:19:00+08:00
draft: false
tags: ["Python", "Flask"]
---

> 看这篇文章之前，建议看一下我之前写的：[源码分析]Flask中路由匹配是如何实现的

BluePrint（蓝图）的概念说白了就是路由组，所有注册到该蓝图上的路由都使用同一个前缀。这样方便了管理，不同的功能可以放在一个模块（比如admin模块）中实现，更加解耦。

首先来看看蓝图是如何使用的：

```python
# 定义一个蓝图
simple_page = Blueprint('simple_page', __name__,
                        template_folder='templates')

# 绑定视图函数
@simple_page.route('/', defaults={'page': 'index'})
@simple_page.route('/<page>')
def show(page):
    try:
        return render_template('pages/%s.html' % page)
    except TemplateNotFound:
        abort(404)
        

        # 在主模块中注册路由
app = Flask(__name__)
app.register_blueprint(simple_page)
```

看上面的例子，首先定义了一个蓝图simple_page，然后经由这个蓝图来定义路由以及绑定到视图函数上，最后在主模块中，注册这个蓝图即可。

看起来跟常见的定义视图函数的方式一样，只不过在添加路由的时候，需要以蓝图开头。

来看看源码中是如何实现的。

蓝图的功能是在flask 0.7版本中被加入的，app在调用 **register_blueprint** 方法的时候会调用 **Blueprint** 类中的 **register** 方法来注册该蓝图中添加的所有路由。

```python
def register_blueprint(self, blueprint, **options):
   	...	
	blueprint.register(self, options, first_registration)
```

我们看一下register方法：

```python
# blueprints.py
def register(self, app, options, first_registration=False):
	
    ...
    state = self.make_setup_state(app, options, first_registration)
    
    ...
    for deferred in self.deferred_functions:
        deferred(state)
```

额，make_setup_state是个啥，deferred_functions又是个啥。我们跳到make_setup_state来看看它里面有什么：

```python
def make_setup_state(self, app, options, first_registration=False):
    return BlueprintSetupState(self, app, options, first_registration)
```

返回了一个类。先不管。来看看deferred_functions是什么，从名字上可以看出是延迟函数之类的。

来梳理一下流程，**app.register_blueprint** 注册蓝图之后，会激活Buleprint类中的register方法，在register方法中循环调用 **deferred_functions** 中的函数来执行，我们大概能猜出来这段代码的功能就是将蓝图中定义的路由都添加到路由组中。

以上面的蓝图例子，

```python
@simple_page.route('/', defaults={'page': 'index'})
```

蓝图的route方法是这样的：

```python
def route(self, rule, **options):
    def decorator(f):
        self.add_url_rule(rule, f.__name__, f, **options)
        return f
    return decorator
```

route方法是个装饰器，实际上调用了 **add_url_rule** 方法：

```python
def add_url_rule(self, rule, endpoint=None, view_func=None, **options):
    self.record(lambda s: s.add_url_rule(rule, endpoint, view_func, **options))
        
def record(self, func):
	....
    self.deferred_functions.append(func)
```

在record方法中，将func添加到了deferred_functions列表中，而add_url_rule中调用了record方法，那么一切就都可以解释了：

**register** 方法中的这段代码，

```python
state = self.make_setup_state(app, options, first_registration)    
...
for deferred in self.deferred_functions:
    deferred(state)
```

循环 **deferred_functions**，**deferred_functions** 里面是啥？是lambda，具体来说，就是蓝图中定义的路由和视图函数，我们通过

```python
@simple_page.route('/<page>')
```

定义路由之后，实际上就是在 **deferred_functions** 里面添加了一个lambda，为什么说它是defer，因为只有在register注册的时候才会真正添加到app的url_map中。

上面代码中的state是一个 **BlueprintSetupState** 示例，这个类里面有一个add_url_rule方法，会在全局app的 **url_map** 中添加路由和视图函数。

```python
def add_url_rule(self, rule, endpoint=None, view_func=None, **options):
    self.app.add_url_rule(rule, '%s.%s' % (self.blueprint.name, endpoint),view_func, defaults=defaults, **options)
```

来梳理一下：

```python
# state 是 BlueprintSetupState 实例
BlueprintSetupState -> state

# deferred_functions 里面是蓝图路由的lambda
lambda s: s.add_url_rule -> deferred_functions

for deferred in self.deferred_functions:
    deferred(state)
    
意思就是 lambda 中的 s 被赋值为 state ，然后state.add_url_rule,
这样就执行了app.add_url_rule
```

这个延迟执行设计的太巧妙了，蓝图中添加的路由规则只有在register方法中才真正的被添加到全局的路由map中。