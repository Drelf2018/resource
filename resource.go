package resource

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Drelf2018/TypeGo/Queue"
	"github.com/Drelf2018/event"
	"github.com/Drelf2020/utils"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type Resource struct {
	event.AsyncEvent[fsnotify.Op]

	Root    *Folder
	Watcher *fsnotify.Watcher

	rename map[string]string
	mu     sync.Mutex
}

func (r *Resource) OnEqual(cmd fsnotify.Op, handle func(name string)) func() {
	return r.AsyncEvent.OnCommand(cmd, event.OnlyData(handle))
}

func (r *Resource) Init(root string) {
	r.AsyncEvent = event.New[fsnotify.Op](114514)
	r.Watcher, _ = fsnotify.NewWatcher()
	r.rename = make(map[string]string)
	r.Root = &Folder{File: File{Name: root}}
	r.Root.Walk()

	q := Queue.New(r.Root)
	for f := range q.Chan() {
		r.Watcher.Add(f.Path())
		q.Next(f.Folders.I...)
	}

	r.OnEqual(fsnotify.Write, func(e string) {
		r.Root.Transport(e).IsFile(
			func(root *Folder, file *File, name string, info os.FileInfo) {
				if info.Size() == 0 {
					return
				}
				var size int64 = info.Size() - file.Size.Byte()
				file.Size.Add(size)
				for root != nil {
					root.Size.Add(size)
					root = root.Back
				}
			},
		)
	})

	r.OnEqual(fsnotify.Remove, func(e string) {
		r.Root.Transport(e).IsFile(
			func(root *Folder, file *File, name string, info os.FileInfo) {
				root.Delete(file)
			},
		).IsDir(
			func(root, dir *Folder, name string, info os.FileInfo) {
				root.Delete(dir)
			},
		)
	})

	r.OnEqual(fsnotify.Create, func(e string) {
		r.mu.Lock()
		l := len(r.rename)
		if l == 0 {
			r.mu.Unlock()
		} else {
			dir, file1 := filepath.Split(e)
			if file0, ok := r.rename[dir]; ok {
				delete(r.rename, dir)
				r.mu.Unlock()
				go r.AsyncEvent.Dispatch(fsnotify.Create|fsnotify.Rename, [3]string{dir, file0, file1})
				return
			}
			r.mu.Unlock()
		}

		sep := string(os.PathSeparator)
		name := strings.Replace(e, r.Root.Path(), "", 1)
		name = utils.Cut(name, sep, sep, 0)
		_, isDir := r.Root.Create(strings.Split(name, sep))
		if isDir {
			r.Watcher.Add(e)
		}
	})

	r.OnCommand(fsnotify.Rename|fsnotify.Create, event.OnlyData(func(data [3]string) {
		r.Root.Transport(data[0], data[1]).IsFile(
			func(root *Folder, file *File, name string, info os.FileInfo) {
				file.Name = data[2]
				file.cache = ""
				root.Files.Sort()
			},
		).IsDir(
			func(root, dir *Folder, name string, info os.FileInfo) {
				dir.Name = data[2]
				dir.cache = ""
				root.Folders.Sort()
			},
		)
	}))

	go func() {
		for event := range r.Watcher.Events {
			if event.Has(fsnotify.Rename) {
				r.mu.Lock()
				dir, file := filepath.Split(event.Name)
				r.rename[dir] = file
				r.mu.Unlock()
				continue
			}
			go r.AsyncEvent.Dispatch(event.Op, event.Name)
		}
	}()
}

func readLine() (r []string) {
	var cmd string
	for {
		n, err := fmt.Scanf("%s", &cmd)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			panic(err)
		}
		r = append(r, cmd)
	}
	return r
}

func decodeSimplifiedChinese(b []byte) string {
	b, _ = simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	return string(b)
}

func (r *Resource) Shell(name string) {
	r.Init(name)
	defer r.Watcher.Close()
	anchor := r.Root

outer:
	for {
		print("Go ", anchor.Path(), "> ")
		cmds := readLine()
		notFound := false
		switch len(cmds) {
		case 1:
			switch cmds[0] {
			case "exit":
				break outer
			case "self":
				println("  ", anchor.String())
			case "ls":
				for f := range anchor.List() {
					fmt.Printf("  %v\n", f)
				}
			default:
				notFound = true
			}
		case 2:
		inner:
			switch cmds[0] {
			case "cd":
				var next *Folder = anchor
				for _, p := range strings.Split(cmds[1], "/") {
					if p == ".." {
						next = next.Back
					} else {
						next = next.CD(p)
					}
					if next == nil {
						println("   Folder", p, "not found.")
						break inner
					}
				}
				anchor = next
			case "mkdir":
				anchor.Mkdir(cmds[1])
			case "find":
				arg := cmds[1]
				file := anchor.Find(arg)
				if file == nil {
					println("   File", arg, "not found.")
				} else {
					println("  ", file.String())
				}
			case "touch":
				anchor.Touch(cmds[1], 0)
			default:
				notFound = true
			}
		}

		if notFound {
			cmd := exec.Command("cmd", slices.Insert(cmds, 0, "/C")...)
			cmd.Dir = anchor.Path()
			stdout, _ := cmd.StdoutPipe()
			cmd.Start()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				println(decodeSimplifiedChinese(scanner.Bytes()))
			}
			cmd.Wait()
		}
		println()
	}
}
