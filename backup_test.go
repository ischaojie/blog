package blog

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v33/github"
	"github.com/shurcooL/githubv4"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func createTempArticle(t *testing.T, fileName string) string {
	// create temp md file
	tempDir := t.TempDir()
	err := ioutil.WriteFile(filepath.Join(tempDir, fileName), []byte("test"), 0644)
	if err != nil {
		log.Fatal(err)
	}

	articlePath := filepath.Join(tempDir, "test.md")
	return articlePath
}

func TestArticlePath(t *testing.T) {

	dir, err := filepath.Abs(filepath.Dir("./content/posts/"))
	if err != nil {
		log.Fatal(err)
	}
	token := os.Getenv("GITHUB_AUTH_TOKEN")
	t.Log(token)
	t.Logf("dir: %s", dir)
}

func TestNewBackup(t *testing.T) {
	backup := NewBackup("github")
	client := backup.client.(*github.Client)
	user, _, _ := client.Users.Get(context.Background(), "")
	if *user.Name != "shiniao" {
		t.Fatalf("github client error")
	}
}

func TestParseArticle(t *testing.T) {
	// create temp md file
	tempDir := t.TempDir()
	err := ioutil.WriteFile(filepath.Join(tempDir, "test.md"), []byte("test"), 0644)
	if err != nil {
		log.Fatal(err)
	}

	articlePath := filepath.Join(tempDir, "test.md")

	cases := []struct {
		in   string
		want []string
	}{
		{articlePath, []string{"test", "test"}},
	}

	for _, c := range cases {

		article, _ := ParseArticle(c.in)
		if c.want[0] != article.Title {
			t.Errorf("result: %q, want: %q", article.Title, c.want[0])
		}
		if c.want[1] != article.Content {
			t.Errorf("result: %q, want: %q", article.Content, c.want[1])
		}
	}
}

func TestBackup_QueryBlogRepoID(t *testing.T) {
	backup := NewBackup("github")
	actualID := "MDEwOlJlcG9zaXRvcnkyODk4MDA1NTY="
	id, _ := backup.QueryBlogRepoID("shiniao", "blog")
	if id != actualID {
		t.Errorf("query blog repo error")
	}
}

func TestBackup_QueryRepoIssues(t *testing.T) {
	backup := NewBackup("github")
	_, err := backup.QueryRepoIssues("shiniao", "blog")
	if err != nil {
		t.Fatal(err)
	}
}

func TestBackup_CreateAndDeleteIssue(t *testing.T) {

	backup := NewBackup("github")
	client := backup.client.(*githubv4.Client)
	var q struct {
		Repository struct {
			Issues struct {
				TotalCount githubv4.Int
			}
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String("shiniao"),
		"repositoryName":  githubv4.String("blog"),
	}
	err := client.Query(context.Background(), &q, variables)
	if err != nil {
		t.Fatal(err)
	}
	beforeIssuesCount := q.Repository.Issues.TotalCount

	path := createTempArticle(t, "test.md")
	article, _ := ParseArticle(path)

	id, err := backup.CreateIssue(article, "shiniao", "blog")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("issue id: %s", id)

	issuesCount, err := backup.DeleteIssue(id)
	if err != nil {
		t.Fatal(err)
	}
	if issuesCount != beforeIssuesCount {
		t.Fatal("delete issue error")
	}

}

func TestConcurrentDeleteIssues(t *testing.T) {
	backup := NewBackup("github")
	issuesID, err := backup.QueryRepoIssues("shiniao", "blog")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan githubv4.Int, len(issuesID))
	errch := make(chan error, len(issuesID))
	for _, id := range issuesID {

		go func(id githubv4.ID) {
			count, err := backup.DeleteIssue(id)
			if err != nil {
				done <- -1
				errch <- err
			}
			done <- count
			errch <- nil
		}(id)
	}
	var errStr string
	var result []githubv4.Int
	for i := 0; i < len(issuesID); i++ {
		result = append(result, <-done)
		if err := <-errch; err != nil {
			errStr = errStr + " " + err.Error()
		}
	}

	if errStr != "" {
		t.Fatal(errors.New(errStr))
	}

	for _, r := range result {
		t.Log(r)
	}

}

func TestBackup_BackupToGithub(t *testing.T) {

	backup := NewBackup("github")
	tempDir := t.TempDir()
	for i := 0; i < 3; i++ {
		// create temp md file
		articleName := fmt.Sprintf("test%d.md", i+1)
		err := ioutil.WriteFile(filepath.Join(tempDir, articleName), []byte("test"), 0644)
		if err != nil {
			log.Fatal(err)
		}

	}

	result, err := backup.BackupToGithub("shiniao", "blog", tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("result: %s", result)
	}
}
