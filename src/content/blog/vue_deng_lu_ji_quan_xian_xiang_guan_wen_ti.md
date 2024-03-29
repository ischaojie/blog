---
title: "Vue 登录及权限相关问题"
pubDate: "2019-04-08T15:17:05+08:00"
tags: ["前端", "Vue"]
---

最近在做一个小应用，需要用到 vue 实现登录，以及给不同路由设置权限。在网上看了很多文章，讲的是乱七八糟。感叹国内技术类文章实在是差劲，抄来抄去。这篇文章就说说我最后是如何实现的。

前后端分离项目中，后端提供 api 接口给前端，使用 jwt 发放权限。

首先前端提供用户名和密码请求登录接口，后端验证之后返回给前端一个 token，之后前端在请求需要权限的接口时携带这个 token 就可以了。

### 两个问题

现在面临两个问题，

首先 vue 中不同的路由有不同的权限，比如我要访问后台 **/admin**, 就需要先登录才行，而有的页面不需要登录。

第二个问题是，vue 组件中使用 axios 请求后台服务时，不同的接口有不同的权限。

![Vue 权限认证](https://shiniao.fun/images/vue_auth.png)

先来解决第二个问题。vue 不同组件都要用到 axios，我们在全局为 axios 添加 request 和 response 的拦截器。

也就是，在发起请求之前，先检测 header 是否携带 token 信息。在接收响应之前，先查看后端返回状态码，如果说需要 token 验证就跳转到登录界面。

在 main.js 添加如下，或者新增一个 http.js 文件：

```javascript
// * http request 拦截器
axios.interceptors.request.use(
    config => {
        // * 判断是否存在 token，如果存在的话，则每个 http header 都加上 token
        // * token 会在登录之后存储在本地
        if (localStorage.token) {
            config.headers["Authorization"]  = `Bearer ${localStorage.token}`;
        }
        return config;
    },
    err => {
        return Promise.reject(err);
    });

// * http response 拦截器
axios.interceptors.response.use(
    response => {
        let data = response.data;
        // * 正常返回数据
        if (data.code === 0) {
            // * 返回 data
            return data
        }
        // * 如果 code 是 20103 表示 token 未认证 (后端定义的错误码)
        // * 跳转到 login
        if (data.code === 20103) {
            router.replace('/login')
        }
        return  Promise.reject(data);
    },
    error => {
        return Promise.reject(error);
    });

Vue.prototype.$http = axios;
```

现在发起的任何请求之前都会检查是否携带 token，如果没有就跳到 login 界面。

在 login 中，携带用户名和密码获取 token 之后，存放到本地。

login.vue:

```javascript
axios.post('/api/login', {
                    email: this.email,
                    password: this.password
                }) .then((res) => {
                        // * 存储 token
                        localStorage.setItem('token', res.data.token);
                        console.info("login successful");
                        // * 跳转回登录前页面
                        this.$router.push({path: this.$route.query.redirect || '/admin',})

                    }).catch((error) => {
                    console.error(error)
                });
```

现在，访问后端接口的权限问题解决了。但是在 vue 中我不同的页面有不同的访问权限该如何处理？vue-router 官方文档给出了例子：

在需要权限的路由添加 meta 信息，表明该路由需要登录才能访问，然后在所有路由跳转之前添加处理函数，如果没有 auth，跳转到登录：

```javascript
path: '/admin',
name: 'admin',
component: () => import('../views/admin/Admin.vue'),
// * 需要登录才能访问
meta: {requiresAuth: true},
```

```javascript
// * 全局钩子
router.beforeEach((to, from, next) => {
    if (to.matched.some(record => record.meta.requiresAuth)) {
        // * 对于需要 auth 的路径
        // * 没有 token 信息，redirect to login
        if (!localStorage.token) {
            next({
                path: '/login',
                query: {redirect: to.fullPath}
            })
        } else {
            next()
        }
    } else {
        next() // 确保一定要调用 next()
    }
})
```

登出的话，清除 token 信息即可。

```javascript
localStorage.removeItem("token")
```

以上，希望有帮助。