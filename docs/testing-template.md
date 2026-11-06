# Go 单元测试标准模板

本模板用于 pixiu 仓库的 Go 单元测试编写与验证，统一表驱动、失败信息、运行方式和清理约定，便于后续功能复用。

## 1. 约定

- 测试文件命名 `*_test.go`，包名与源码保持一致（可测试未导出函数）。
- 优先测试**纯函数**与**状态决策**；不依赖网络、MySQL、CRI 等外部环境。
- 使用标准库 `testing`，不引入第三方断言库。
- 采用表驱动 + `t.Run` 子测试；失败信息必须包含输入、实际值、期望值。
- 测试通过后，按当前仓库约定移除 `_test.go` 文件，写法沉淀到本模板；验证命令与结果写入任务报告。
- 数据库迁移、容器重建、健康检查等集成行为不在单元测试中伪造，应在真实环境单独验证并记录命令与结果。

## 2. 模板一：纯函数 / 表驱动

```go
package foo

import "testing"

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "空字符串", in: "", want: ""},
		{name: "带 v 前缀", in: "v2.0.3", want: "2.0.3"},
		{name: "带预发布后缀", in: "v2.0.3-beta.1", want: "2.0.3-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeVersion(tt.in); got != tt.want {
				t.Fatalf("normalizeVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

适用：版本比较、字符串归一化、镜像 tag 解析、参数校验等无副作用函数。

## 3. 模板二：状态机 / 多分支决策

```go
package foo

import "testing"

func TestDecideStatus(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		available bool
		want      int
	}{
		{name: "未升级发现新版本", current: 0, available: true, want: 0},
		{name: "已是最新", current: 0, available: false, want: 3},
		{name: "同版本失败保持失败态", current: 4, available: true, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideStatus(tt.current, tt.available); got != tt.want {
				t.Fatalf("decideStatus(%d, %v) = %d, want %d",
					tt.current, tt.available, got, tt.want)
			}
		})
	}
}
```

要求：每个状态迁移分支至少一个用例；边界值（缺失记录、空值、失败态、完成态）单独成例。

## 4. 模板三：对象构造与测试辅助

```go
package foo

import "testing"

func TestMatchTarget(t *testing.T) {
	target := &Record{Version: "v2.0.3", Image: "registry.example.com/pixiu:v2.0.3"}

	tests := []struct {
		name    string
		current string
		record  *Record
		want    bool
	}{
		{name: "镜像 tag 匹配", current: "registry.example.com/pixiu:v2.0.3", record: target, want: true},
		{name: "记录为空", current: "registry.example.com/pixiu:v2.0.3", record: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchTarget(tt.current, tt.record); got != tt.want {
				t.Fatalf("matchTarget(%q, %+v) = %v, want %v", tt.current, tt.record, got, tt.want)
			}
		})
	}
}
```

- 复杂对象在表外构造一次，避免每个用例重复。
- 抽取断言辅助函数时加 `t.Helper()`，让失败行号指向调用处。

## 5. 断言规范

- 默认用 `t.Fatalf`：失败即停止当前子测试；不要只 `t.Log`。
- 失败信息格式：`函数名(输入) = 实际值, want 期望值`。
- 布尔/整型/字符串直接比较；结构体优先逐字段比较，避免 `reflect.DeepEqual` 掩盖差异。
- 不写 `time.Sleep`、随机数、真实网络请求；时间相关逻辑通过参数/接口注入。
- 子测试名称使用中文短语，便于失败定位。

## 6. 运行与验证

```bash
# 当前包
go test ./pkg/updater/... -count=1

# 指定用例
go test ./pkg/updater/... -run TestDecideStatus -v

# 全仓编译与静态检查
go build ./...
go vet ./...
gofmt -l <改动文件>
```

验证完成后检查：

```bash
find . -name '*_test.go' -not -path './.git/*' | wc -l   # 按约定应为 0
```

## 7. 复跑已移除的测试

测试文件按约定移除后，可从历史提交用临时 worktree 复跑，不污染当前分支：

```bash
git worktree add --detach /tmp/pixiu-test-check <commit>
(cd /tmp/pixiu-test-check && go test ./pkg/updater/... -v)
git worktree remove --force /tmp/pixiu-test-check
```

不要把临时 worktree 留在仓库目录内；验证结束后必须清理。

## 8. 本次参考实现

自动升级状态机改造的完整测试示例见提交 `2822167`：

- `pkg/updater/version_test.go`：版本比较、镜像 tag 解析
- `pkg/updater/status_test.go`：状态决策、目标匹配、失败原因保留

查看方式：

```bash
git show 2822167:pkg/updater/status_test.go
git show 2822167:pkg/updater/version_test.go
```

## 9. 检查清单

- [ ] 每个纯函数/状态分支有对应用例
- [ ] 失败信息包含输入、实际值、期望值
- [ ] `gofmt -l` 无输出
- [ ] `go build ./...`、`go vet ./...`、`go test` 全部通过
- [ ] 验证完成后移除 `_test.go`，仓库内测试文件数为 0
- [ ] 在任务报告与 `.gitcommits` 中记录测试命令、结果与未覆盖项

## 10. 风险提示

当前约定不在仓库中保留 `_test.go`，回归保护依赖任务执行时的临时测试与历史提交。关键状态机、版本比较等逻辑建议在 CI 或独立测试仓中保留等价测试，避免后续改动失去自动回归能力。
