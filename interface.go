package resource

import (
	"fmt"

	"github.com/Drelf2018/TypeGo/Chan"
)

type Explorer interface {
	Parent() Explorer
	Path(file ...string) string

	CD(path string) Explorer
	Find(name string) *File
	Touch(name string, size int64) (*File, bool)
	JumpTo(path string) Explorer
	MakeTo(path string) Explorer
	List() Chan.Chan[fmt.Stringer]
	Replace(path string) string

	MkdirAll()
	RemoveAll()
}
