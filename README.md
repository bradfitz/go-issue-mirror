# Go Issue Tracker Mirror

This repo is a mirror of the
[Go Issue Tracker](https://github.com/golang/go/issues).

The goal is to permit offline access and more powerful searches. It
also doesn't take any quota to query this way.

To use it:

```go
package main

import (
	"log"

	"github.com/bradfitz/go-issue-mirror/issues"
	"github.com/google/go-github/github"
)

func main() {
	root, err := issues.Open()
	if err != nil {
		log.Fatal(err)
	}
	is, err := root.Issue(1234)
	log.Printf("Issue: %#v, %v", is, err)

	c, err := root.IssueComment(1234, 66053086)
	log.Printf("Comment: %#v, %v", c, err)

	root.ForeachIssue(func(is *github.Issue) error {
		log.Printf("Issue %d: %v", *is.Number, *is.Title)
		root.ForeachIssueComment(*is.Number, func(c *github.IssueComment) error {
			log.Printf("  comment from %v at %v", *c.User.Login, *c.CreatedAt)
			return nil
		})
		return nil
	})
}
```
