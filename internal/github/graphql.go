package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// batchSize is the number of files to query per GraphQL request.
	batchSize = 100
)

// GetFileCommitDates fetches the last commit date for multiple files.
func (c *Client) GetFileCommitDates(ctx context.Context, repo Repository, paths []string) ([]FileCommitInfo, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	results := make([]FileCommitInfo, 0, len(paths))

	// Process files in batches to stay within GraphQL API limits.
	for i := 0; i < len(paths); i += batchSize {
		end := min(i+batchSize, len(paths))
		batch := paths[i:end]

		query := buildFileHistoryQuery(repo.Owner, repo.Name, repo.Ref, batch)

		var response struct {
			Repository struct {
				Ref struct {
					Target map[string]struct {
						Nodes []struct {
							CommittedDate time.Time `json:"committedDate"`
						} `json:"nodes"`
					} `json:"target"`
				} `json:"ref"`
			} `json:"repository"`
		}

		err := c.graphql.DoWithContext(ctx, query, nil, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch file commit dates: %w", err)
		}

		// Extract commit dates from the response
		for j, path := range batch {
			alias := "file" + strconv.Itoa(j)
			history, ok := response.Repository.Ref.Target[alias]
			if !ok || len(history.Nodes) == 0 {
				continue // File doesn't exist or no commit history
			}

			results = append(results, FileCommitInfo{
				Path:          path,
				CommittedDate: history.Nodes[0].CommittedDate,
			})
		}
	}

	return results, nil
}

// buildFileHistoryQuery builds a compact GraphQL query with aliases for each file.
// Query structure (shown formatted for readability, actual query is compact):
//
//	{
//	  repository(owner: "owner", name: "repo") {
//	    ref(qualifiedName: "ref") {
//	      target {
//	        ... on Commit {
//	          file0: history(first: 1, path: "path0") {
//	            nodes { committedDate }
//	          }
//	          file1: history(first: 1, path: "path1") {
//	            nodes { committedDate }
//	          }
//	        }
//	      }
//	    }
//	  }
//	}
func buildFileHistoryQuery(owner, repo, ref string, paths []string) string {
	var buf strings.Builder
	buf.Grow(200 + len(paths)*80) // estimate: 200 bytes base overhead + ~80 bytes per path

	fmt.Fprintf(&buf, "{repository(owner:%q,name:%q){ref(qualifiedName:%q){target{...on Commit{", owner, repo, ref)

	for i, path := range paths {
		escapedPath, _ := json.Marshal(path)
		fmt.Fprintf(&buf, "%s:history(first:1,path:%s){nodes{committedDate}}", "file"+strconv.Itoa(i), escapedPath)
	}

	fmt.Fprintf(&buf, "}}}}}")

	return buf.String()
}
