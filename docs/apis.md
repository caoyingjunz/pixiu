# Pixiu API Specification

- Pixiu API 是对 Pixiu 资源的调用接口，通常接口有 list, show details, create, update 和 delete, 以及对 resource 的具体动作, 如: 启动 cicd 资源的 job.

- API 应当遵循（以 cicd 资源的 job 为例):

## List
- method: GET
- URL: /cicd/jobs
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403)
- 请求参数:
- 响应结果:

## Show Details
- method: GET
- URL: /cicd/jobs/{job_id}
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数:
- 响应结果:

## Create
- method: POST
- URL: /cicd/jobs
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403)
- 请求参数:
- 响应结果:

## Update
- method: PUT
- URL: /cicd/jobs/{image_id}
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数:
- 响应结果:

## Delete
- method: DELETE
- URL: /cicd/jobs/{job_id}
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404)
- 请求参数:
- 响应结果:

## Run
- method: POST
- URL: /cicd/jobs/{job_id}/run
- 正常返回码: 200
- 异常返回码: badRequest(400), unauthorized(401), forbidden(403), itemNotFound(404), NotImplemented(501)
- 请求参数:
- 响应结果:
