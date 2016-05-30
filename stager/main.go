package main

import "github.com/cf-furnace/k8s-stager/cmd"

var version = ""

func main() {
	cmd.Execute(version)
}
