package main

import (
	"github.com/timoth-y/fabnctl/cmd/fabnctl"
	_ "github.com/timoth-y/fabnctl/pkg/core"
)

func main() {
	fabnctl.Execute()
}
