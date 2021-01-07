package main

import (
	"github.com/shiniao/blog"
	"path/filepath"
)


func main() {
	backup := blog.NewBackup("github")
	dir, _ := filepath.Abs("content/posts")
	backup.BackupToGithubCon("blog", dir)
}
