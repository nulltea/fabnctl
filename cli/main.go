package main

import (
	"github.com/timoth-y/chainmetric-network/cli/cmd"
	"github.com/timoth-y/chainmetric-network/cli/shared"
)

func init() {
	shared.InitCore()
}

func main() {
	cmd.Execute()
}
