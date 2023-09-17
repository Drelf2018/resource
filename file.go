package resource

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Drelf2018/TypeGo/Chan"
	"github.com/Drelf2018/asyncio"
	"github.com/Drelf2018/cmps"
	"github.com/Drelf2020/utils"
)

type File struct {
	Name  string  `cmps:"1" json:"name"`
	Back  *Folder `json:"-"`
	Size  Size    `json:"size"`
	cache string
}

func (f *File) Path(file ...string) string {
	if f.cache == "" {
		if b := f.Back; b != nil {
			f.cache = b.Path(f.Name)
		} else {
			f.cache = filepath.Join(f.Name)
		}
	}
	if len(file) == 0 {
		return f.cache
	}
	return filepath.Join(slices.Insert(file, 0, f.cache)...)
}

func (f *File) Parent() Explorer {
	return f.Back
}

func (f *File) Load() ([]byte, error) {
	return os.ReadFile(f.Path())
}

func (f *File) MustLoad() []byte {
	b, _ := f.Load()
	return b
}

func (f *File) Read() (string, error) {
	b, err := f.Load()
	return string(b), err
}

func (f *File) MustRead() string {
	s, _ := f.Read()
	return s
}

func (f *File) Store(data []byte) error {
	return os.WriteFile(f.Path(), data, os.ModePerm)
}

func (f *File) MustStore(data []byte) {
	f.Store(data)
}

func (f *File) Write(data string) error {
	return f.Store([]byte(data))
}

func (f *File) MustWrite(data string) {
	f.Write(data)
}

func (f *File) Remove() error {
	return os.Remove(f.Path())
}

func (f *File) MustRemove() {
	f.Remove()
}

func (f *File) String() string {
	return fmt.Sprintf("File(%v, %v)", f.Name, f.Size)
}

type Folder struct {
	File    `cmps:"1"`
	Files   cmps.SafeSlice[*File]   `json:"files"`
	Folders cmps.SafeSlice[*Folder] `json:"folders"`
}

func (f *Folder) String() string {
	return fmt.Sprintf("Folder(%v, %v)", f.Name, f.Size)
}

func (f *Folder) Find(name string) *File {
	return f.Files.Search(&File{Name: name})
}

func (f *Folder) touch(name string) *File {
	return &File{Name: name, Back: f}
}

func (f *Folder) Touch(name string, size int64) (*File, bool) {
	if c := f.Find(name); c != nil {
		return c, false
	}
	file := f.touch(name)
	file.Size.Set(size)
	f.Files.Insert(file)
	return file, true
}

// Warning: if you want to check the returning Explorer whether nil.
//
// You need to use:
//
//	f.CD("xxx") == (*Folder)(nil)
//
// Instead of:
//
//	f.CD("xxx") == nil
func (f *Folder) CD(path string) Explorer {
	return f.Folders.Search(&Folder{File: File{Name: path}})
}

func (f *Folder) JumpTo(path string) Explorer {
	var anchor Explorer = f
	for _, d := range Split(f.Replace(path)) {
		if d == "" {
			continue
		}
		anchor = anchor.CD(d)
		if anchor == (*Folder)(nil) {
			return anchor
		}
	}
	return anchor
}

func (f *Folder) mkdir(path string) *Folder {
	return &Folder{
		File:    *f.touch(path),
		Files:   cmps.SafeSlice[*File]{I: make([]*File, 0)},
		Folders: cmps.SafeSlice[*Folder]{I: make([]*Folder, 0)},
	}
}

func (f *Folder) Mkdir(path string) (*Folder, bool) {
	if c := f.CD(path).(*Folder); c != nil {
		return c, false
	}
	folder := f.mkdir(path)
	f.Folders.Insert(folder)
	return folder, true
}

func (f *Folder) Child(path string) *Folder {
	c, _ := f.Mkdir(path)
	return c
}

func (f *Folder) MakeTo(path string) Explorer {
	for _, d := range Split(f.Replace(path)) {
		if d == "" {
			continue
		}
		f, _ = f.Mkdir(path)
	}
	return f
}

func (f *Folder) MkdirAll() {
	os.MkdirAll(f.Path(), os.ModePerm)
}

func (f *Folder) RemoveAll() {
	os.RemoveAll(f.Path())
}

func (f *Folder) Delete(c interface{ Byte() int64 }) {
	if c == nil {
		return
	}
	switch c := c.(type) {
	case *File:
		f.Files.Delete(c)
	case *Folder:
		f.Folders.Delete(c)
	}
	var size int64 = -c.Byte()
	for f != nil {
		f.Size.Add(size)
		f = f.Back
	}
}

func (f *Folder) List() Chan.Chan[fmt.Stringer] {
	return Chan.Auto[fmt.Stringer](func(c chan fmt.Stringer) {
		for _, folders := range f.Folders.I {
			c <- folders
		}
		for _, file := range f.Files.I {
			c <- file
		}
	})
}

func (f *Folder) create(fi os.FileInfo) int64 {
	if fi.IsDir() {
		if dir, ok := f.Mkdir(fi.Name()); ok {
			return dir.Walk()
		}
	} else {
		if _, ok := f.Touch(fi.Name(), fi.Size()); ok {
			return fi.Size()
		}
	}
	return 0
}

// Use Walk() function when detect a new folder create or initialize *Folder.
func (f *Folder) Walk() int64 {
	files, err := os.ReadDir(f.Path())
	if err != nil {
		return 0
	}

	var size int64 = 0
	asyncio.ForEach(files, func(de fs.DirEntry) {
		fi, _ := de.Info()
		size += f.create(fi)
	})
	f.Size.Set(size)
	return size
}

// The dirs is used to describe the subdirectory under f.
func (f *Folder) Create(dirs []string) (size int64, isDir bool) {
	if len(dirs) != 1 {
		size, isDir = f.Child(dirs[0]).Create(dirs[1:])
		f.Size.Add(size)
		return
	}
	fi, _ := os.Stat(f.Path(dirs[0]))
	return f.create(fi), fi.IsDir()
}

type Anchor struct {
	root *Folder
	dir  *Folder
	file *File
	name string
	info os.FileInfo
}

func (a *Anchor) IsDir(f func(root, dir *Folder, name string, info os.FileInfo)) *Anchor {
	if a == nil {
		return nil
	}
	if a.dir != nil {
		f(a.root, a.dir, a.name, a.info)
	}
	return a
}

func (a *Anchor) IsFile(f func(root *Folder, file *File, name string, info os.FileInfo)) *Anchor {
	if a == nil {
		return nil
	}
	if a.file != nil {
		f(a.root, a.file, a.name, a.info)
	}
	return a
}

const SEP = string(os.PathSeparator)

func Split(path string) []string {
	path = utils.Cut(path, SEP, SEP, 0)
	return strings.Split(path, SEP)
}

func (f *Folder) Replace(path string) string {
	abs, _ := filepath.Abs(f.Path())
	return strings.Replace(path, abs, "", 1)
}

func (f *Folder) Transport(path string, names ...string) *Anchor {
	var name string
	if len(names) != 0 {
		name = strings.Join(names, " ")
	} else {
		path, name = filepath.Split(path)
	}
	f = f.JumpTo(path).(*Folder)
	if f == nil {
		return nil
	}
	info, _ := os.Stat(f.Path(name))
	return &Anchor{
		root: f,
		dir:  f.CD(name).(*Folder),
		file: f.Find(name),
		name: name,
		info: info,
	}
}
