+++
title = "[源码阅读] Pydantic 是如何实现的"
date = 2021-12-25T22:12:17+08:00
draft = true
+++

Pydantic 是一个使用类型注解进行数据解析和校验的 lib。有点像 marshmallow，但特点是使用了 type hints。有名的 fastapi 就使用 pydantic 进行 api 数据的校验。

之前有个通过 json schema 生成表单的需求，本来想自己实现 model 到 json schema 的转换的，没想到 pydantic 已经有这功能了。

来看看 pydantic 的源码：

一个基本的 Model

```python
from pydantic import BaseModel

class User(BaseModel):
    name: str
    age: int
    normal: bool = True
```

todo......
