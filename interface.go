package resource

type Explorer interface {
	Path(file ...string) string

	Find(name string) *File
	Touch(name string, size int64) (*File, bool)
	MakeTo(path string) Explorer

	MkdirAll()
	RemoveAll()
}
