# Users API Specification

## List
- method: GET
- URL: /users
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403)
- 请求参数:
- 响应结果:

## Show Details
- method: GET
- URL: /users/:id
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数:
- 响应结果:

## Create
- method: POST
- URL: /users
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403)
- 请求参数: name, password ...
- 响应结果:

## Update
- method: PUT
- URL: /users/:id
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数: role, email, description
- 响应结果:

## Delete
- method: DELETE
- URL: /users/:id
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数:
- 响应结果:

## Login
- method: POST
- URL: /users/login
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数: name, password
- 响应结果:

## Logout
- method: POST
- URL: /users/:id/login
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数: name, password
- 响应结果:

## ChangePassword
- method: POST
- URL: /users/:id
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数: current_password, new_password, re_new_password
- 响应结果:

## ResetPassword
- method: POST
- URL: /users/:id
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数: new_password, re_new_password
- 响应结果: