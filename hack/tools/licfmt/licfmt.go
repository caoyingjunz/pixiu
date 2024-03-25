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

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/sync/errgroup"
)

const tpml = `/*
Copyright %s The Pixiu Authors.

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
`

var (
	ignorePatterns = stringSlice{"vendor/**", "docs/**"}
	year           = flag.String("y", fmt.Sprint(time.Now().Year()), "copyright year(s)")
	verbose        = flag.Bool("v", false, "verbose mode: print the name of the files that are modified or were skipped")
)

func init() {
	flag.Var(&ignorePatterns, "i", "file patterns to ignore, for example: -i vendor/**")
}

type stringSlice []string

func (i *stringSlice) String() string {
	return fmt.Sprint(*i)
}

func (i *stringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type file struct {
	path string
	mode os.FileMode
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	for _, p := range ignorePatterns {
		if !doublestar.ValidatePattern(p) {
			log.Fatalf("ignore pattern %q is invalid", p)
		}
	}

	headerText := fmt.Sprintf(tpml, *year)

	// process at most CPU number files in parallel
	ch := make(chan *file, runtime.NumCPU())
	done := make(chan struct{})
	go func() {
		var wg errgroup.Group
		for f := range ch {
			f := f
			wg.Go(func() error {
				updated, err := addLicenseHeader(f.path, headerText, f.mode)
				if err != nil {
					log.Printf("%s: %v", f.path, err)
					return err
				}
				if *verbose && updated {
					log.Printf("%s updated", f.path)
				}
				return nil
			})
		}
		err := wg.Wait()
		close(done)
		if err != nil {
			os.Exit(1)
		}
	}()

	for _, d := range flag.Args() {
		if err := walk(ch, d); err != nil {
			log.Fatal(err)
		}
	}
	close(ch)
	<-done
}

// walk walks the file tree from root, sends go file to task queue.
func walk(ch chan<- *file, root string) error {
	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Printf("%s error: %v", path, err)
		}
		if fi.IsDir() {
			return nil
		}
		if match(path, ignorePatterns) || filepath.Ext(path) != ".go" {
			if *verbose {
				log.Printf("skipping: %s", path)
			}
			return nil
		}
		ch <- &file{path, fi.Mode()}
		return nil
	})
}

// match returns if path matches one of the provided file patterns.
func match(path string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := doublestar.Match(p, path); ok {
			return true
		}
	}
	return false
}

// hasLicense returns if there is a license in the file header already.
func hasLicense(header []byte) bool {
	return bytes.Contains(header, []byte("Copyright")) &&
		bytes.Contains(header, []byte("Apache License"))
}

// addLicenseHeader adds a license header to the file if it does not exist.
func addLicenseHeader(path, license string, fmode os.FileMode) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR, fmode)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := bytes.Buffer{}
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Bytes()
		buf.Write(line)
		buf.WriteString("\n")
		if bytes.HasPrefix(line, []byte("package ")) {
			// reading until the line contains `package p`
			break
		}
	}

	if hasLicense(buf.Bytes()) {
		return false, nil
	}

	// add license header at the beginning
	new := bytes.NewBufferString(license + "\n")
	// reuse the buffer
	if _, err := new.Write(buf.Bytes()); err != nil {
		return false, err
	}
	// read the content after `package p`
	if _, err := f.Seek(int64(buf.Len()), io.SeekStart); err != nil {
		return false, err
	}
	if _, err := new.ReadFrom(f); err != nil {
		return false, err
	}
	// rewrite the whole file
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	if _, err := new.WriteTo(f); err != nil {
		return false, err
	}
	err = f.Sync()
	return err == nil, err
}
