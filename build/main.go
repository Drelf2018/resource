package main

import (
	"os"

	"github.com/Drelf2018/resource"
)

func main() {
	dir, _ := os.Getwd()
	new(resource.Resource).Init(dir).Shell(resource.ReadLine)
}
