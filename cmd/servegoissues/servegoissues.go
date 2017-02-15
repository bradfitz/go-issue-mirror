// servegoissues is a program that serves Go issues over HTTP, so they can be viewed in a browser.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"

	goissues "github.com/bradfitz/go-issue-mirror/issues"
	"github.com/bradfitz/issuemirror"
	"github.com/google/go-github/github"
	"github.com/shurcooL/issues"
	"github.com/shurcooL/issuesapp"
	"github.com/shurcooL/users"
)

var httpFlag = flag.String("http", ":8080", "Listen for HTTP connections on this address.")

func main() {
	flag.Parse()

	root, err := goissues.Open()
	if err != nil {
		log.Fatalln(err)
	}

	// We provide a simple read-only implementation of issues.Service on top of root,
	// and a nil users service since this runs locally and doesn't need user authentication.
	issuesApp := issuesapp.New(issuesService{root: root}, nil, issuesapp.Options{
		HeadPre: `<style type="text/css">
	body {
		margin: 20px;
		font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
		font-size: 14px;
		line-height: initial;
		color: #373a3c;
	}
	a {
		color: #0275d8;
		text-decoration: none;
	}
	a:focus, a:hover {
		color: #014c8c;
		text-decoration: underline;
	}
	.btn {
		font-size: 11px;
		line-height: 11px;
		border-radius: 4px;
		border: solid #d2d2d2 1px;
		background-color: #fff;
		box-shadow: 0 1px 1px rgba(0, 0, 0, .05);
	}
</style>`,
		BodyPre: `
{{/* Override new comment component to link to original issue for leaving comments. */}}
{{define "new-comment"}}<div class="event" style="margin-top: 20px; margin-bottom: 100px;">
	View <a href="https://github.com/golang/go/issues/{{.Issue.ID}}#new_comment_field">original issue</a> to comment.
</div>{{end}}`,
	})

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		req = req.WithContext(context.WithValue(req.Context(), issuesapp.RepoSpecContextKey, issues.RepoSpec{URI: "github.com/golang/go"}))
		req = req.WithContext(context.WithValue(req.Context(), issuesapp.BaseURIContextKey, "."))
		issuesApp.ServeHTTP(w, req)
	})
	http.HandleFunc("/login/github", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "Sorry, this is a read-only instance and it doesn't support signing in.")
	})

	printServingAt(*httpFlag)
	err = http.ListenAndServe(*httpFlag, nil)
	if err != nil {
		log.Fatalln("http.ListenAndServe:", err)
	}
}

func printServingAt(addr string) {
	hostPort := addr
	if strings.HasPrefix(hostPort, ":") {
		hostPort = "localhost" + hostPort
	}
	fmt.Printf("serving at http://%s/\n", hostPort)
}

// issuesService implements issues.Service using issuemirror.Root.
// It's a view-only implementation at this time, so create and edit endpoints
// are not implemented.
type issuesService struct {
	root issuemirror.Root
}

// List issues.
func (s issuesService) List(ctx context.Context, repo issues.RepoSpec, opt issues.IssueListOptions) ([]issues.Issue, error) {
	var is []issues.Issue

	// TODO: Pagination support would be nice... But meh, it reads from local disk,
	//       and the browser seems to render 14000+ issues on a single page pretty well.
	//       So this isn't as critical or blocking as I expected. Besides, it's nice
	//       to be able to do Cmd+F to find stuff when it's all on one page.

	err := s.root.ForeachIssue(func(issue *github.Issue) error {
		switch {
		case opt.State == issues.StateFilter(issues.OpenState) && issues.State(*issue.State) != issues.OpenState:
			return nil
		case opt.State == issues.StateFilter(issues.ClosedState) && issues.State(*issue.State) != issues.ClosedState:
			return nil
		}

		var labels []issues.Label
		for _, l := range issue.Labels {
			color, err := ghColor(l.Color)
			if err != nil {
				// issuemirror doesn't seem to provide label colors now, so fall back to a default light gray.
				color = issues.RGB{R: 0xed, G: 0xed, B: 0xed}
			}
			labels = append(labels, issues.Label{
				Name:  *l.Name,
				Color: color,
			})
		}
		is = append(is, issues.Issue{
			ID:     uint64(*issue.Number),
			State:  issues.State(*issue.State),
			Title:  *issue.Title,
			Labels: labels,
			Comment: issues.Comment{
				User:      ghUser(issue.User),
				CreatedAt: *issue.CreatedAt,
			},
			Replies: *issue.Comments,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(byID(is)))

	return is, nil
}

// byID implements sort.Interface.
type byID []issues.Issue

func (s byID) Len() int           { return len(s) }
func (s byID) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s byID) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Count issues.
func (s issuesService) Count(ctx context.Context, repo issues.RepoSpec, opt issues.IssueListOptions) (uint64, error) {
	var count uint64

	err := s.root.ForeachIssue(func(issue *github.Issue) error {
		switch {
		case opt.State == issues.StateFilter(issues.OpenState) && issues.State(*issue.State) != issues.OpenState:
			return nil
		case opt.State == issues.StateFilter(issues.ClosedState) && issues.State(*issue.State) != issues.ClosedState:
			return nil
		}

		count++

		return nil
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Get an issue.
func (s issuesService) Get(ctx context.Context, repo issues.RepoSpec, id uint64) (issues.Issue, error) {
	issue, err := s.root.Issue(int(id))
	if err != nil {
		return issues.Issue{}, err
	}

	return issues.Issue{
		ID:    uint64(*issue.Number),
		State: issues.State(*issue.State),
		Title: *issue.Title,
		Comment: issues.Comment{
			User:      ghUser(issue.User),
			CreatedAt: *issue.CreatedAt,
			Editable:  false,
		},
	}, nil
}

// ListComments lists comments for specified issue id.
func (s issuesService) ListComments(ctx context.Context, repo issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Comment, error) {
	// TODO: Pagination. Respect opt.Start and opt.Length, if given.

	var comments []issues.Comment

	issue, err := s.root.Issue(int(id))
	if err != nil {
		return comments, err
	}
	comments = append(comments, issues.Comment{
		ID:        0, // We use 0 as a special ID for the comment that is the issue description.
		User:      ghUser(issue.User),
		CreatedAt: *issue.CreatedAt,
		Body:      *issue.Body,
		Reactions: nil, // Doesn't seem to be exposed by issuemirror.
		Editable:  false,
	})

	err = s.root.ForeachIssueComment(int(id), func(comment *github.IssueComment) error {
		comments = append(comments, issues.Comment{
			ID:        uint64(*comment.ID),
			User:      ghUser(comment.User),
			CreatedAt: *comment.CreatedAt,
			Body:      *comment.Body,
			Reactions: nil, // Doesn't seem to be exposed by issuemirror.
			Editable:  false,
		})

		return nil
	})
	return comments, err
}

// ListEvents lists events for specified issue id.
func (issuesService) ListEvents(ctx context.Context, repo issues.RepoSpec, id uint64, opt *issues.ListOptions) ([]issues.Event, error) {
	// For now, no events.
	// Not sure if issuemirror exposes them. They're pretty optional, so look into this later.
	return nil, nil
}

// Create a new issue.
func (issuesService) Create(ctx context.Context, repo issues.RepoSpec, issue issues.Issue) (issues.Issue, error) {
	return issues.Issue{}, fmt.Errorf("Create: not implemented")
}

// CreateComment creates a new comment for specified issue id.
func (issuesService) CreateComment(ctx context.Context, repo issues.RepoSpec, id uint64, comment issues.Comment) (issues.Comment, error) {
	return issues.Comment{}, fmt.Errorf("CreateComment: not implemented")
}

// Edit the specified issue id.
func (issuesService) Edit(ctx context.Context, repo issues.RepoSpec, id uint64, ir issues.IssueRequest) (issues.Issue, []issues.Event, error) {
	return issues.Issue{}, nil, fmt.Errorf("Edit: not implemented")
}

// EditComment edits comment of specified issue id.
func (issuesService) EditComment(ctx context.Context, repo issues.RepoSpec, id uint64, cr issues.CommentRequest) (issues.Comment, error) {
	return issues.Comment{}, fmt.Errorf("EditComment: not implemented")
}

// ghUser converts a GitHub user into a users.User.
func ghUser(user *github.User) users.User {
	return users.User{
		UserSpec: users.UserSpec{
			ID:     uint64(*user.ID),
			Domain: "github.com",
		},
		Login:     *user.Login,
		AvatarURL: template.URL(fmt.Sprintf("https://avatars.githubusercontent.com/u/%v?v=3", *user.ID)),
		HTMLURL:   template.URL(fmt.Sprintf("https://github.com/%v", *user.Login)),
	}
}

// ghColor converts a GitHub color hex string like "ff0000"
// into an issues.RGB value.
func ghColor(hex *string) (issues.RGB, error) {
	if hex == nil {
		return issues.RGB{}, errors.New("color value is missing")
	}
	var c issues.RGB
	_, err := fmt.Sscanf(*hex, "%02x%02x%02x", &c.R, &c.G, &c.B)
	if err != nil {
		return issues.RGB{}, fmt.Errorf("color value %q has unexpected format, error parsing: %v", *hex, err)
	}
	return c, nil
}
