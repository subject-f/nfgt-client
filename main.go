package main

import (
	"nfgt-client/pkg/api"
	"nfgt-client/pkg/client"
	"nfgt-client/pkg/common"
	"nfgt-client/pkg/git"
)

func main() {
	config := common.ParseConfig("./config.yaml")
	git := git.NewGitProvider(&config)
	cli := client.NewClient(&config, git)
	api := api.NewRouterController(&config, cli)

	api.Run()
}
