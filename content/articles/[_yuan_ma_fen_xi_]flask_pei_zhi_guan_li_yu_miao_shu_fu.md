+++
title = "[源码分析] Flask配置管理与描述符析"
date = 2019-07-03T11:19:00+08:00
draft = false
[taxonomies]
tags = ["Python", "Flask"]
+++

在Flask中可以通过 **app.config['NAME'] = what** 的形式指定一些配置，比如设置 **debug = True** ：

```python
app.debug = True
# 或者
app.config['DEBUG'] = True
```

有些配置比如设置ENV和TESTING还可以直接利用Flask对象来设置，像这样：

```python
app.testing = True
```

除了在程序中指定配置，也可以将配置写在单独的文件中，比如：

```python
app = Flask(__name__)
app.config.from_object('yourapplication.default_settings')
app.config.from_envvar('YOURAPPLICATION_SETTINGS')
```

应用首先从 **yourapplication.default_settings** 模块载入配置，然后根据 `YOURAPPLICATION_SETTINGS` 环境变量所指向的文件的内容重载配置的值。

除了从配置文件加载，也可以定义类类指定配置，具体用法去看看官方文档就知道了。

知道了在Flask中如何使用配置，我们来看看它是如何实现的。

首先 **config** 肯定是个变量，在Flask这个类中被定义为：

```python
self.config = self.make_config(instance_relative_config)
```

然后 **make_config** 的代码是这样的：

```python
def make_config(self, instance_relative=False):

    root_path = self.root_path
    if instance_relative:
        root_path = self.instance_path
    # 默认配置
    defaults = dict(self.default_config)
    defaults['ENV'] = get_env()
    defaults['DEBUG'] = get_debug_flag()
    return self.config_class(root_path, defaults)
```

make_config方法获取flask中默认的配置，以及ENV和DEBUG这两个配置，之后返回了 **self.config_class** 对象，它在类中是这样定义的：

```python
config_class = Config

class Config(dict):
    def __init__(self, root_path, defaults=None):
        # dict.__init__让Config实例拥有字典行为config['ENV']
        dict.__init__(self, defaults or {})
        self.root_path = root_path
```

**config_class** 本质上是Config类，注意看Config类的初始化方法，

```python
dict.__init__(self, defaults or {})
```

这一行代码使得Config类可以像字典一样使用，比如 `app.config['TESTING']=True`。 当然你也可以使用 `__getitem__` 和 `__setitem__` 内置方法使得类具有字典的行为。

那么像 **app.testing = True** 这样的配置是如何实现的？

在Flask类中可以看到，这些类变量都是 **ConfigAttribute** 对象。

```python
testing = ConfigAttribute('TESTING')
secret_key = ConfigAttribute('SECRET_KEY')
```

**ConfigAttribute**类如下：

```python
class ConfigAttribute(object):
    def __init__(self, name, get_converter=None):
        self.__name__ = name
        self.get_converter = get_converter
    
    # obj是被托管类实例
    def __get__(self, obj, type=None):
        # 如果被托管实例不存在，返回描述符自身
        if obj is None:
            return self
        # 返回Flask实例的config[name]
        rv = obj.config[self.__name__]
        if self.get_converter is not None:
            rv = self.get_converter(rv)
        return rv

    def __set__(self, obj, value):
        obj.config[self.__name__] = value
```

**ConfigAttribute** 是一个描述符类，描述符是什么？

> 描述符是对多个属性运用相同存取逻辑的一种方式。——《流畅的python》

描述符实现了特定的内置方法，`__get__` ， `__set__` 和 `__delete__` ，常见的比如Django中ORM中的实现就是用的描述符：

```python
class Person(models.Model):
    # models.CharField就是一个描述符
    first_name = models.CharField(max_length=30)
    last_name = models.CharField(max_length=30)
```

还有python内置的 **@property** **@classmethod** **staticmethod** 装饰器就是用描述符实现的。

说了这么多，来看看描述符到底怎么用。

首先来看**ConfigAttribute**类中的 `__get__` 方法：

```python
# obj是被托管类实例
def __get__(self, obj, type=None):
    # 如果被托管实例不存在，返回描述符自身
    if obj is None:
        return self
    # 返回Flask实例的config[name]
    rv = obj.config[self.__name__]
    if self.get_converter is not None:
        rv = self.get_converter(rv)
    return rv
```

`__get__` 方法的参数obj是被托管类的实例，在这里就是Flask类，方法中首先判断被托管类是否存在，不存在就返回描述符本身。之后返回Flask类实例中的 **config[name]**， 看到没有env类变量实际上就是 **config[name]** 中指定的值。

来看 `__set__` 方法：

```python
def __set__(self, obj, value):
    obj.config[self.__name__] = value
```

`__set__`方法中的obj同样是被托管类的实例，然后value被存储在被托管类的config变量中。所以，我们才可以用 **app.testing = True** 来指定配置。

说到底，描述符有什么用？我们来看，在Flask类中我们要指定类变量 **testing, env, secret_key, session_cookie_name** 等等，都需要从config变量中接收（保证配置的一致性）。我们为这些变量都写一个存取方法是不是很麻烦，使用描述符就可以简化流程，对外封装了具体的存取细节，并且减少代码量。

当然我们也可以使用一个函数，通过构建特性工厂的方式来实现，比如：

```python
def config_attribute(name):
    def getter(instance):
        return instance.config[name]

    def setter(instance, value):
        instance.config[name] = value

    # property实际上就是@property装饰器
    return property(getter, setter)

# 在Flask中调用方式一样
testing = config_attribute('TESTING')
```

关于描述符的更多细节可以查看 **《流畅的python》** 这本书中第20章的内容，有详细的介绍。