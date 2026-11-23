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

package sqlbackup

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	DownloadRoute = "/sql-backups/:id/download"
	DownloadPath  = "/pixiu/extension" + DownloadRoute
	cookieName    = "pixiu-sql-backup"
	fileTTL       = 10 * time.Minute
)

type artifact struct {
	file     *os.File
	unlinked bool
	secret   string
	filename string
	size     int64
	expires  time.Time
	active   bool
}

func (a *artifact) close() {
	_ = a.file.Close()
	if !a.unlinked {
		_ = os.Remove(a.file.Name())
	}
	<-slots
}

var (
	slots      = make(chan struct{}, 8)
	generators = make(chan struct{}, 2)
	filesMu    sync.Mutex
	files      = make(map[string]*artifact)
)

type downloadResult struct {
	*types.SQLBackupSummary
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Serve 生成 SQL 备份文件，失败时写入统一错误响应，供各引擎备份路由复用。
func Serve(c *gin.Context, filename string, generate func(context.Context, io.Writer) (*types.SQLBackupSummary, error)) {
	if err := Stage(c, filename, generate); err != nil {
		httputils.SetFailed(c, httputils.NewResponse(), err)
	}
}

func Stage(c *gin.Context, filename string, generate func(context.Context, io.Writer) (*types.SQLBackupSummary, error)) error {
	select {
	case generators <- struct{}{}:
		defer func() { <-generators }()
	default:
		return apierrors.NewError(fmt.Errorf("too many SQL backups are being generated"), http.StatusTooManyRequests)
	}
	select {
	case slots <- struct{}{}:
	default:
		return apierrors.NewError(fmt.Errorf("too many SQL backups awaiting download"), http.StatusTooManyRequests)
	}
	file, err := os.CreateTemp("", "pixiu-sql-backup-*")
	if err != nil {
		<-slots
		return fmt.Errorf("could not create temporary SQL backup file: %w", err)
	}
	a := &artifact{file: file}
	a.unlinked = os.Remove(file.Name()) == nil
	ready := false
	defer func() {
		if !ready {
			a.close()
		}
	}()
	buffer := bufio.NewWriterSize(file, 64<<10)
	started := time.Now()
	summary, err := generate(c, buffer)
	generation := time.Since(started)
	if err != nil {
		return err
	}
	if err := buffer.Flush(); err != nil {
		return fmt.Errorf("could not finish temporary SQL backup file: %w", err)
	}
	if err := c.Err(); err != nil {
		return err
	}
	if summary == nil || summary.SizeBytes <= 0 {
		return fmt.Errorf("empty SQL backup")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not verify temporary SQL backup file: %w", err)
	}
	if info.Size() != summary.SizeBytes {
		return fmt.Errorf("temporary SQL backup file size mismatch")
	}

	random := make([]byte, 48)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	id, secret := hex.EncodeToString(random[:16]), hex.EncodeToString(random[16:])
	url := strings.Replace(DownloadPath, ":id", id, 1)
	a.secret = secret
	a.filename = cleanFilename(filename)
	a.size = summary.SizeBytes
	a.expires = time.Now().Add(fileTTL)
	filesMu.Lock()
	files[id] = a
	time.AfterFunc(fileTTL, func() {
		filesMu.Lock()
		expired := files[id]
		if expired == nil || expired.active {
			filesMu.Unlock()
			return
		}
		delete(files, id)
		filesMu.Unlock()
		expired.close()
	})
	filesMu.Unlock()
	ready = true
	klog.V(3).InfoS("SQL backup prepared", "generation", generation,
		"afterGeneration", time.Since(started)-generation, "tables", summary.TableNum,
		"bytes", summary.SizeBytes, "truncated", summary.Truncated)

	c.Header("Cache-Control", "no-store")
	http.SetCookie(c.Writer, &http.Cookie{
		Name: cookieName, Value: secret, Path: url, MaxAge: int(fileTTL / time.Second),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secureRequest(c),
	})
	response := httputils.NewResponse()
	response.Result = downloadResult{SQLBackupSummary: summary, DownloadURL: url, ExpiresAt: a.expires}
	httputils.SetSuccess(c, response)
	return nil
}

func secureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func cleanFilename(filename string) string {
	if len(filename) > 240 {
		return "backup.sql"
	}
	filename = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, filename))
	if filename == "" || filename == ".sql" {
		return "backup.sql"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".sql") {
		filename += ".sql"
	}
	return filename
}

func Download(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id := c.Param("id")
	secret, err := c.Cookie(cookieName)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	filesMu.Lock()
	a := files[id]
	if a == nil || !time.Now().Before(a.expires) || subtle.ConstantTimeCompare([]byte(secret), []byte(a.secret)) != 1 {
		filesMu.Unlock()
		c.Status(http.StatusNotFound)
		return
	}
	if a.active {
		filesMu.Unlock()
		c.Status(http.StatusConflict)
		return
	}
	a.active = true
	filesMu.Unlock()
	defer func() {
		filesMu.Lock()
		if files[id] == a {
			delete(files, id)
		}
		filesMu.Unlock()
		a.close()
	}()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.filename}))
	c.Header("Content-Length", fmt.Sprint(a.size))
	c.Header("X-Content-Type-Options", "nosniff")
	started := time.Now()
	written, err := io.CopyN(c.Writer, io.NewSectionReader(a.file, 0, a.size), a.size)
	klog.V(3).InfoS("SQL backup download", "duration", time.Since(started),
		"bytes", written, "complete", err == nil)
	if err != nil {
		_ = c.Error(err)
	}
}
