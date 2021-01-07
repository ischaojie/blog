package main

import (
	"github.com/shiniao/blog"
	flag "github.com/spf13/pflag"
)

var owner *string = flag.String("owner", "", "github owner")
var repo *string = flag.String("repo", "", "backup repo")

func main() {
	flag.Parse()
	backup := blog.NewBackup("github")
	backup.BackupToGithubCon(*owner, *repo, "../content/posts/")
}
