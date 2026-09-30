/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package runtime

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNormalizeImageRef(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "短名补全 docker.io/library 与 latest", in: "nginx", want: "docker.io/library/nginx:latest"},
		{name: "短名带 tag 补全 registry 与 library", in: "nginx:1.25", want: "docker.io/library/nginx:1.25"},
		{name: "含 registry 的完整引用原样保留", in: "registry.example.com:5000/foo/bar:v1", want: "registry.example.com:5000/foo/bar:v1"},
		{name: "digest 引用保留 digest", in: "docker.io/library/nginx@" + digest, want: "docker.io/library/nginx@" + digest},
		{name: "空串报错", in: "", wantErr: true},
		{name: "含空格报错", in: "nginx foo", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeImageRef(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeImageRef(%q) 期望报错，实际得到 %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeImageRef(%q) 报错: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeImageRef(%q) = %q，期望 %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseBinds(t *testing.T) {
	mounts, err := parseBinds([]string{"/etc/pixiu/plan/12:/configs", "/data:/data"})
	if err != nil {
		t.Fatalf("parseBinds 报错: %v", err)
	}
	if len(mounts) != 2 {
		t.Fatalf("mounts 数量 = %d，期望 2", len(mounts))
	}
	if mounts[0].Source != "/etc/pixiu/plan/12" || mounts[0].Destination != "/configs" {
		t.Fatalf("mounts[0] = %+v，期望 host=/etc/pixiu/plan/12 container=/configs", mounts[0])
	}
	if mounts[0].Type != "bind" {
		t.Fatalf("mounts[0].Type = %q，期望 bind", mounts[0].Type)
	}
	if len(mounts[0].Options) == 0 || mounts[0].Options[0] != "rbind" {
		t.Fatalf("mounts[0].Options = %v，期望包含 rbind", mounts[0].Options)
	}

	for _, bad := range []string{"no-colon", ":/configs", "/host:", ""} {
		if _, err := parseBinds([]string{bad}); err == nil {
			t.Fatalf("parseBinds(%q) 期望报错", bad)
		}
	}
}

func TestContainerNameRe(t *testing.T) {
	valid := []string{"deploy-12", "apply-1", "a.b_c-9", "A1"}
	for _, name := range valid {
		if !containerNameRe.MatchString(name) {
			t.Fatalf("容器名 %q 应合法", name)
		}
	}

	invalid := []string{"_deploy", "-deploy", ".hidden", "deploy/1", "deploy 1", ""}
	for _, name := range invalid {
		if containerNameRe.MatchString(name) {
			t.Fatalf("容器名 %q 应非法", name)
		}
	}
}

func TestFileFollower(t *testing.T) {
	dir := t.TempDir()

	t.Run("跟随增长的文件并靠 done 收尾", func(t *testing.T) {
		path := filepath.Join(dir, "deploy-1.log")
		if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
			t.Fatalf("写入初始日志失败: %v", err)
		}

		done := make(chan struct{})
		l := newFileFollower(context.Background(), path, done, nil, nil)
		defer l.Close()

		// 初始内容必须能读到（文件已存在，无需等待轮询）
		buf := make([]byte, 6)
		if _, err := io.ReadFull(l, buf); err != nil {
			t.Fatalf("读取初始内容失败: %v", err)
		}
		if string(buf) != "hello\n" {
			t.Fatalf("初始内容 = %q，期望 %q", buf, "hello\n")
		}

		// 追加内容后关闭 done，模拟 RunContainer 返回
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("打开日志追加失败: %v", err)
		}
		if _, err = f.WriteString("world\n"); err != nil {
			t.Fatalf("追加日志失败: %v", err)
		}
		_ = f.Close()
		close(done)

		rest, err := io.ReadAll(l)
		if err != nil {
			t.Fatalf("读取追加内容失败: %v", err)
		}
		if string(rest) != "world\n" {
			t.Fatalf("追加内容 = %q，期望 %q", rest, "world\n")
		}
	})

	t.Run("文件后出现仍能跟随", func(t *testing.T) {
		path := filepath.Join(dir, "late.log")
		// 启动时文件尚不存在，覆盖 pump 的惰性打开（轮询等待文件出现）分支；done 为 nil
		l := newFileFollower(context.Background(), path, nil, nil, nil)
		defer l.Close()

		if err := os.WriteFile(path, []byte("late\n"), 0o644); err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}

		buf := make([]byte, 5)
		readErr := make(chan error, 1)
		go func() {
			_, err := io.ReadFull(l, buf)
			readErr <- err
		}()
		select {
		case err := <-readErr:
			if err != nil {
				t.Fatalf("读取后出现文件的内容失败: %v", err)
			}
			if string(buf) != "late\n" {
				t.Fatalf("内容 = %q，期望 %q", buf, "late\n")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("文件出现后 5s 内未读到内容")
		}
	})

	t.Run("Close 先于文件出现", func(t *testing.T) {
		// 同上使用 done 为 nil 的未登记配置，Close 覆盖 pump 尚未打开文件的阶段
		l := newFileFollower(context.Background(), filepath.Join(dir, "never.log"), nil, nil, nil)
		if err := l.Close(); err != nil {
			t.Fatalf("Close 报错: %v", err)
		}

		readErr := make(chan error, 1)
		go func() {
			_, err := l.Read(make([]byte, 8))
			readErr <- err
		}()
		select {
		case err := <-readErr:
			if err == nil {
				t.Fatal("Close 后 Read 应返回错误而非继续等待")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Close 后 Read 悬挂未返回")
		}
	})

	t.Run("Close 后 Read 立即结束不悬挂", func(t *testing.T) {
		l := newFileFollower(context.Background(), filepath.Join(dir, "missing.log"), make(chan struct{}), nil, nil)
		if err := l.Close(); err != nil {
			t.Fatalf("Close 报错: %v", err)
		}

		readErr := make(chan error, 1)
		go func() {
			_, err := l.Read(make([]byte, 8))
			readErr <- err
		}()
		select {
		case err := <-readErr:
			if err == nil {
				t.Fatal("Close 后 Read 应返回错误而非继续等待")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Close 后 Read 悬挂未返回")
		}
	})

	t.Run("Close 与跟随中的 pump 并发安全", func(t *testing.T) {
		path := filepath.Join(dir, "race.log")
		if err := os.WriteFile(path, []byte("0123456789\n"), 0o644); err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}

		l := newFileFollower(context.Background(), path, nil, nil, nil)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 只消费一部分数据就 Close，尽量让 Close 与 pump 的读写并发
			_, _ = io.ReadFull(l, make([]byte, 4))
		}()
		_ = l.Close()
		wg.Wait()

		if _, err := l.Read(make([]byte, 4)); err == nil {
			t.Fatal("Close 后 Read 应返回错误")
		}
	})
}
