package main

import (
	"fmt"
	"github.com/shiniao/blog"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
)

var (
	repo    string
	token   string
	version bool
)

func init() {

	pflag.StringVarP(&repo, "repo", "r", "blog", "which github repo to backup")
	pflag.StringVarP(&token, "token", "t", "", "github access token")
	pflag.BoolVarP(&version, "version", "v", false, "display version information")
}

func main() {
	pflag.Parse()
	if version {
		fmt.Printf("blog backup version %s", "1.0.0")
		os.Exit(0)
	}
	if len(pflag.Args()) < 1 {
		pflag.Usage()
		os.Exit(1)
	}

	if token == "" {
		token = os.Getenv("GITHUB_AUTH_TOKEN")
	} else {
		_ = os.Setenv("GITHUB_AUTH_TOKEN", token)
	}

	target := pflag.Arg(0)
	if target == "github" {
		backup := blog.NewBackup("github")
		dir, _ := filepath.Abs("content/posts")
		backup.BackupToGithubCon(repo, dir)
	} else {
		fmt.Printf("This backup target not yet support")
		os.Exit(0)
	}

}
