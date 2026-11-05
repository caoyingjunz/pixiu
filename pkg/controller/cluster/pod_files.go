/*
Copyright 2024 The Pixiu Authors.

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

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	podFileExecTimeout     = 60 * time.Second
	podFileDownloadTimeout = 120 * time.Second
)

// ListPodFiles lists entries under path inside the pod container via non-TTY exec.
func (c *cluster) ListPodFiles(ctx context.Context, cluster, namespace, pod, container, filePath string) (*types.PodFileListResult, error) {
	if container == "" {
		return nil, fmt.Errorf("container is required")
	}
	cleaned, err := sanitizePodFilePath(filePath)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := c.execPodCommand(ctx, cluster, namespace, pod, container, []string{
		"sh", "-c", podFileListScript, "--", cleaned,
	})
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		klog.Errorf("list pod files failed cluster=%s ns=%s pod=%s container=%s path=%s: %v stderr=%s",
			cluster, namespace, pod, container, cleaned, err, msg)
		return nil, fmt.Errorf("list files failed: %s", shortenExecMessage(msg, err))
	}

	items := parsePodFileListOutput(stdout)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			if items[i].Type == "dir" {
				return true
			}
			if items[j].Type == "dir" {
				return false
			}
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return &types.PodFileListResult{
		Path:  cleaned,
		Items: items,
	}, nil
}

// DownloadPodFile streams a file or directory (as tar) from the pod container to w.
func (c *cluster) DownloadPodFile(ctx context.Context, cluster, namespace, pod, container, filePath string, w http.ResponseWriter) error {
	if container == "" {
		return fmt.Errorf("container is required")
	}

	cleaned, err := sanitizePodFilePath(filePath)
	if err != nil {
		return err
	}
	// Root filesystem archive is intentionally blocked (too large / risky).
	if cleaned == "/" {
		return fmt.Errorf("cannot download root directory")
	}

	stdout, stderr, err := c.execPodStream(ctx, podFileDownloadTimeout, cluster, namespace, pod, container, []string{
		"sh", "-c", podFileDownloadTimeoutScript, "--", podFileDownloadScript, cleaned, strconv.FormatInt(types.PodFileMaxBytes, 10),
	}, nil, types.PodFileMaxBytes)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		klog.Errorf("download pod file failed cluster=%s ns=%s pod=%s container=%s path=%s: %v stderr=%s",
			cluster, namespace, pod, container, cleaned, err, msg)
		return fmt.Errorf("download failed: %s", shortenExecMessage(msg, err))
	}

	filename := path.Base(cleaned)
	if filename == "" || filename == "." || filename == "/" {
		filename = "download"
	}
	if parsePodDownloadKind(stderr) == "dir" {
		filename = filename + ".tar"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeDownloadFilename(filename)))
	w.Header().Set("Content-Length", strconv.Itoa(len(stdout)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(stdout)
	return err
}

// UploadPodFile writes a local file into the pod container at dirPath/filename via exec stdin.
// Existing files are overwritten. size is the declared content length (must be >= 0).
func (c *cluster) UploadPodFile(ctx context.Context, cluster, namespace, pod, container, dirPath, filename string, r io.Reader, size int64) error {
	if container == "" {
		return fmt.Errorf("container is required")
	}
	if r == nil {
		return fmt.Errorf("file content is required")
	}
	if size < 0 {
		return fmt.Errorf("invalid file size")
	}
	if size > types.PodFileMaxBytes {
		return fmt.Errorf("file exceeds size limit (%d bytes)", types.PodFileMaxBytes)
	}

	dir, err := sanitizePodFilePath(dirPath)
	if err != nil {
		return err
	}
	name, err := sanitizeUploadFilename(filename)
	if err != nil {
		return err
	}
	target := path.Join(dir, name)
	if target == "/" {
		return fmt.Errorf("invalid upload path")
	}

	limited := io.LimitReader(r, types.PodFileMaxBytes+1)
	stdout, stderr, err := c.execPodCommandWithStdin(ctx, podFileDownloadTimeout, cluster, namespace, pod, container,
		[]string{
			"sh", "-c", podFileUploadScript, "--", target,
		}, limited)
	_ = stdout
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		klog.Errorf("upload pod file failed cluster=%s ns=%s pod=%s container=%s path=%s: %v stderr=%s",
			cluster, namespace, pod, container, target, err, msg)
		return fmt.Errorf("upload failed: %s", shortenExecMessage(msg, err))
	}
	return nil
}

func (c *cluster) execPodCommand(ctx context.Context, cluster, namespace, pod, container string, command []string) ([]byte, string, error) {
	return c.execPodCommandWithTimeout(ctx, podFileExecTimeout, cluster, namespace, pod, container, command)
}

func (c *cluster) execPodCommandWithTimeout(ctx context.Context, timeout time.Duration, cluster, namespace, pod, container string, command []string) ([]byte, string, error) {
	return c.execPodStream(ctx, timeout, cluster, namespace, pod, container, command, nil, 0)
}

func (c *cluster) execPodCommandWithStdin(ctx context.Context, timeout time.Duration, cluster, namespace, pod, container string, command []string, stdin io.Reader) ([]byte, string, error) {
	return c.execPodStream(ctx, timeout, cluster, namespace, pod, container, command, stdin, 0)
}

type podFileOutputBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int64
	exceeded bool
	sealed   bool
	cancel   context.CancelFunc
}

func (b *podFileOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sealed {
		return len(p), nil
	}
	if b.limit > 0 && int64(b.buffer.Len())+int64(len(p)) > b.limit {
		b.exceeded = true
		if b.cancel != nil {
			b.cancel()
		}
		return 0, fmt.Errorf("pod file output exceeds size limit")
	}
	return b.buffer.Write(p)
}

func (b *podFileOutputBuffer) result() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sealed = true
	if b.exceeded {
		return nil, true
	}
	return b.buffer.Bytes(), false
}

func (c *cluster) execPodStream(ctx context.Context, timeout time.Duration, cluster, namespace, pod, container string, command []string, stdin io.Reader, outputLimit int64) ([]byte, string, error) {
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, "", err
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := cs.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := client.NewSPDYExecutor(cs.Config, "POST", req.URL())
	if err != nil {
		return nil, "", err
	}

	stdout := &podFileOutputBuffer{limit: outputLimit, cancel: cancel}
	stderr := &podFileOutputBuffer{}
	err = executor.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})
	output, exceeded := stdout.result()
	errorOutput, _ := stderr.result()
	if exceeded {
		return nil, fmt.Sprintf("ETOOLARGE:%d", outputLimit), fmt.Errorf("pod file output exceeds size limit")
	}
	return output, string(errorOutput), err
}

func sanitizePodFilePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "/"
	}
	if strings.Contains(p, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	cleaned := path.Clean(p)
	if cleaned == "." {
		cleaned = "/"
	}
	if !path.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid path")
	}
	return cleaned, nil
}

func sanitizeDownloadFilename(name string) string {
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' || r == '/' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "download"
	}
	return name
}

func sanitizeUploadFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid filename")
	}
	name = path.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid filename")
	}
	return name, nil
}

func shortenExecMessage(stderr string, err error) string {
	lines := strings.Split(stderr, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "PIXIU_KIND:") {
			filtered = append(filtered, line)
		}
	}
	msg := strings.Join(filtered, "\n")
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, "E") && strings.Contains(line, ":") {
			msg = line
			break
		}
	}
	switch {
	case strings.HasPrefix(msg, "ENOTDIR:"):
		return "path is not a directory"
	case strings.HasPrefix(msg, "ENOENT:"):
		return "path not found"
	case strings.HasPrefix(msg, "EISDIR:"):
		return "path is a directory"
	case strings.HasPrefix(msg, "EUNSUPPORTED:"):
		return "unsupported file type or symbolic link"
	case strings.HasPrefix(msg, "ENOSTAT:"):
		return "container does not support required file metadata operations"
	case strings.HasPrefix(msg, "ENOTIMEOUT:"):
		return "container does not support safe file download (timeout unavailable)"
	case strings.HasPrefix(msg, "ETIMEDOUT:"):
		return "file download timed out"
	case strings.HasPrefix(msg, "ENOTAR:"):
		return "container does not support directory download (tar unavailable)"
	case strings.HasPrefix(msg, "ETOOLARGE:"):
		return fmt.Sprintf("file or directory exceeds size limit (%d bytes)", types.PodFileMaxBytes)
	case strings.Contains(msg, "Permission denied"), strings.Contains(msg, "permission denied"):
		return "permission denied writing file"
	case strings.Contains(msg, "Read-only file system"), strings.Contains(msg, "read-only file system"):
		return "filesystem is read-only"
	case strings.Contains(msg, "executable file not found"), strings.Contains(msg, "no such file or directory"):
		return "container does not support file browse (shell unavailable)"
	case msg != "":
		if len(msg) > 300 {
			return msg[:300]
		}
		return msg
	case err != nil:
		return err.Error()
	default:
		return "unknown error"
	}
}

// parsePodFileListOutput parses lines: type\tsize\tmtime\tmode\tuid\tgid\tname
func parsePodFileListOutput(out []byte) []types.PodFileEntry {
	lines := strings.Split(string(out), "\n")
	items := make([]types.PodFileEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 7)
		// backward compatible: type size mtime name
		if len(parts) == 4 {
			typ := parts[0]
			switch typ {
			case "dir", "file", "link", "other":
			default:
				continue
			}
			size, sizeErr := strconv.ParseInt(parts[1], 10, 64)
			if sizeErr != nil {
				size = -1
			}
			mtime, _ := strconv.ParseInt(parts[2], 10, 64)
			name := parts[3]
			if name == "" || name == "." || name == ".." {
				continue
			}
			entry := types.PodFileEntry{Name: name, Type: typ, Size: size}
			if mtime > 0 {
				entry.ModTime = time.Unix(mtime, 0).UTC().Format(time.RFC3339)
			}
			items = append(items, entry)
			continue
		}
		if len(parts) < 7 {
			continue
		}
		typ := parts[0]
		switch typ {
		case "dir", "file", "link", "other":
		default:
			continue
		}
		size, sizeErr := strconv.ParseInt(parts[1], 10, 64)
		if sizeErr != nil {
			size = -1
		}
		mtime, _ := strconv.ParseInt(parts[2], 10, 64)
		mode := parts[3]
		uid := parts[4]
		gid := parts[5]
		name := parts[6]
		if name == "" || name == "." || name == ".." {
			continue
		}
		entry := types.PodFileEntry{
			Name: name,
			Type: typ,
			Size: size,
			Mode: mode,
			Uid:  uid,
			Gid:  gid,
		}
		if mtime > 0 {
			entry.ModTime = time.Unix(mtime, 0).UTC().Format(time.RFC3339)
		}
		items = append(items, entry)
	}
	return items
}

// podFileListScript lists directory entries as: type\tsize\tmtime\tmode\tuid\tgid\tname
const podFileListScript = `
set -e
P="$1"
if [ ! -e "$P" ]; then
  echo "ENOENT:$P" >&2
  exit 2
fi
if [ ! -d "$P" ]; then
  echo "ENOTDIR:$P" >&2
  exit 2
fi
cd "$P" || exit 2
ls -A 2>/dev/null | while IFS= read -r f; do
  [ -z "$f" ] && continue
  [ "$f" = "." ] && continue
  [ "$f" = ".." ] && continue
  if [ -L "$f" ]; then
    t="link"
  elif [ -d "$f" ]; then
    t="dir"
  elif [ -f "$f" ]; then
    t="file"
  else
    t="other"
  fi
  s=$(stat -c %s "$f" 2>/dev/null || stat -f %z "$f" 2>/dev/null || true)
  case "$s" in
    ''|*[!0-9]*) s='-' ;;
  esac
  m=$(date -r "$f" +%s 2>/dev/null || stat -c %Y "$f" 2>/dev/null || echo 0)
  mode=$(ls -ld "$f" 2>/dev/null | awk '{print $1}' || echo '-')
  uid=$(stat -c %u "$f" 2>/dev/null || stat -f %u "$f" 2>/dev/null || echo '0')
  gid=$(stat -c %g "$f" 2>/dev/null || stat -f %g "$f" 2>/dev/null || echo '0')
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$t" "$s" "$m" "$mode" "$uid" "$gid" "$f"
done
`

func parsePodDownloadKind(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PIXIU_KIND:") {
			return strings.TrimPrefix(line, "PIXIU_KIND:")
		}
	}
	return "file"
}

// podFileDownloadTimeoutScript runs downloads with a container-side deadline.
const podFileDownloadTimeoutScript = `
if ! command -v timeout >/dev/null 2>&1 || ! timeout -k 1 2 true >/dev/null 2>&1; then
  echo "ENOTIMEOUT:timeout not available" >&2
  exit 5
fi
timeout -k 5 110 sh -c "$1" -- "$2" "$3"
status=$?
if [ "$status" -eq 124 ] || [ "$status" -eq 137 ]; then
  echo "ETIMEDOUT:download timed out" >&2
fi
exit "$status"
`

// podFileDownloadScript downloads a regular file or directory (tar).
// Args: path maxBytes. On success stderr may contain PIXIU_KIND:file|dir.
const podFileDownloadScript = `
set -e
P="$1"
MAX="$2"
if [ ! -e "$P" ]; then
  echo "ENOENT:$P" >&2
  exit 2
fi
if [ -L "$P" ]; then
  echo "EUNSUPPORTED:$P" >&2
  exit 2
fi
if [ -d "$P" ]; then
  echo "PIXIU_KIND:dir" >&2
  if ! command -v tar >/dev/null 2>&1; then
    echo "ENOTAR:tar not found" >&2
    exit 5
  fi
  kb=$(du -sk "$P" 2>/dev/null | awk '{print $1}')
  case "$kb" in
    ''|*[!0-9]*) kb=0 ;;
  esac
  s=$((kb * 1024))
  if [ "$s" -gt "$MAX" ]; then
    echo "ETOOLARGE:$s" >&2
    exit 4
  fi
  parent=$(dirname "$P")
  base=$(basename "$P")
  tar cf - -C "$parent" "$base"
  exit 0
fi
if [ ! -f "$P" ]; then
  echo "EUNSUPPORTED:$P" >&2
  exit 2
fi
echo "PIXIU_KIND:file" >&2
s=$(stat -c %s "$P" 2>/dev/null || stat -f %z "$P" 2>/dev/null || true)
case "$s" in
  ''|*[!0-9]*) echo "ENOSTAT:$P" >&2; exit 5 ;;
esac
if [ "$s" -gt "$MAX" ]; then
  echo "ETOOLARGE:$s" >&2
  exit 4
fi
head -c "$((MAX + 1))" -- "$P"
`

// podFileUploadScript writes stdin to the target path. Args: absolute file path.
const podFileUploadScript = `
set -e
P="$1"
parent=$(dirname "$P")
if [ ! -d "$parent" ]; then
  echo "ENOTDIR:$parent" >&2
  exit 2
fi
if [ -L "$P" ]; then
  echo "EUNSUPPORTED:$P" >&2
  exit 3
fi
if [ -d "$P" ]; then
  echo "EISDIR:$P" >&2
  exit 3
fi
if [ -e "$P" ] && [ ! -f "$P" ]; then
  echo "EUNSUPPORTED:$P" >&2
  exit 3
fi
cat > "$P"
`
