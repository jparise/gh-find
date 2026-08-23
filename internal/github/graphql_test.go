package github

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildFileHistoryQuery(t *testing.T) {
	tests := []struct {
		name     string
		owner    string
		repo     string
		ref      string
		paths    []string
		contains []string
	}{
		{
			name:  "files",
			owner: "cli",
			repo:  "cli",
			ref:   "trunk",
			paths: []string{"README.md", "LICENSE", "go.mod"},
			contains: []string{
				"ref(qualifiedName:\"trunk\")",
				"file0:history(first:1,path:\"README.md\")",
				"file1:history(first:1,path:\"LICENSE\")",
				"file2:history(first:1,path:\"go.mod\")",
			},
		},
		{
			name:  "file with quotes",
			owner: "cli",
			repo:  "cli",
			ref:   "trunk",
			paths: []string{"path/to/\"file\".txt"},
			contains: []string{
				"history(first:1,path:\"path/to/\\\"file\\\".txt\")",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildFileHistoryQuery(tt.owner, tt.repo, tt.ref, tt.paths)

			for _, substr := range tt.contains {
				if !strings.Contains(query, substr) {
					t.Errorf("query missing expected substring %q:\n%s", substr, query)
				}
			}
		})
	}
}

func TestGetFileCommitDates(t *testing.T) {
	testDate := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		paths      []string
		mockStatus int
		mockBody   string
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "empty paths",
			paths:     nil,
			wantCount: 0,
		},
		{
			name:       "single file",
			paths:      []string{"README.md"},
			mockStatus: 200,
			mockBody:   `{"data":{"repository":{"ref":{"target":{"file0":{"nodes":[{"committedDate":"2024-01-15T10:30:00Z"}]}}}}}}`,
			wantCount:  1,
		},
		{
			name:       "multiple files",
			paths:      []string{"README.md", "LICENSE", "go.mod"},
			mockStatus: 200,
			mockBody:   `{"data":{"repository":{"ref":{"target":{"file0":{"nodes":[{"committedDate":"2024-01-15T10:30:00Z"}]},"file1":{"nodes":[{"committedDate":"2024-01-15T10:30:00Z"}]},"file2":{"nodes":[{"committedDate":"2024-01-15T10:30:00Z"}]}}}}}}`,
			wantCount:  3,
		},
		{
			name:       "files with empty history excluded",
			paths:      []string{"README.md", "missing.txt"},
			mockStatus: 200,
			mockBody:   `{"data":{"repository":{"ref":{"target":{"file0":{"nodes":[{"committedDate":"2024-01-15T10:30:00Z"}]},"file1":{"nodes":[]}}}}}}`,
			wantCount:  1,
		},
		{
			name:       "error response",
			paths:      []string{"README.md"},
			mockStatus: 500,
			mockBody:   `{"message": "Internal Server Error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var responses []mockResponse
			if tt.mockStatus != 0 {
				query := buildFileHistoryQuery("cli", "cli", "main", tt.paths)
				responses = append(responses, graphqlResponse(query, tt.mockStatus, tt.mockBody))
			}

			client := testClient(t, responses...)
			repo := Repository{Owner: "cli", Name: "cli", Ref: "main"}
			got, err := client.GetFileCommitDates(context.Background(), repo, tt.paths)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetFileCommitDates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(got) != tt.wantCount {
				t.Errorf("GetFileCommitDates() returned %d results, want %d", len(got), tt.wantCount)
			}

			// Verify all returned results have valid paths and dates
			for i, result := range got {
				if result.Path == "" {
					t.Errorf("result[%d] has empty path", i)
				}
				if result.CommittedDate.IsZero() {
					t.Errorf("result[%d] has zero commit date", i)
				}
				if !tt.wantErr && i < len(tt.paths) && result.Path != tt.paths[i] {
					t.Errorf("result[%d].Path = %q, want %q", i, result.Path, tt.paths[i])
				}
			}

			// Verify first result in detail
			if !tt.wantErr && len(got) > 0 && got[0].Path == "README.md" {
				if !got[0].CommittedDate.Equal(testDate) {
					t.Errorf("first result CommittedDate = %v, want %v", got[0].CommittedDate, testDate)
				}
			}
		})
	}
}

func TestGetFileCommitDates_MultipleBatches(t *testing.T) {
	// Create 150 files to trigger 2 batches (100 + 50)
	paths := make([]string, 150)
	for i := range paths {
		paths[i] = fmt.Sprintf("file%d.go", i)
	}

	// Mock both batches
	responses := make([]mockResponse, 0, 2)
	for batchNum := range 2 {
		start := batchNum * 100
		end := min(start+100, len(paths))
		batch := paths[start:end]
		query := buildFileHistoryQuery("cli", "cli", "main", batch)
		response := buildBatchResponse(len(batch), "2024-01-15T10:00:00Z")
		responses = append(responses, graphqlResponse(query, 200, response))
	}

	client := testClient(t, responses...)
	repo := Repository{Owner: "cli", Name: "cli", Ref: "main"}

	got, err := client.GetFileCommitDates(context.Background(), repo, paths)
	if err != nil {
		t.Fatalf("GetFileCommitDates() error = %v", err)
	}

	if len(got) != 150 {
		t.Errorf("got %d results, want 150", len(got))
	}
}

func TestGetFileCommitDates_ContextCanceled(t *testing.T) {
	client := testClient(t)
	repo := Repository{Owner: "cli", Name: "cli", Ref: "main"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetFileCommitDates(ctx, repo, []string{"README.md"})
	if err == nil {
		t.Error("expected error with canceled context")
	}
}

// buildBatchResponse creates a GraphQL response with N files, all with the same commit date.
func buildBatchResponse(count int, commitDate string) string {
	var sb strings.Builder
	sb.WriteString(`{"data":{"repository":{"ref":{"target":{`)

	for i := range count {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"file%d":{"nodes":[{"committedDate":%q}]}`, i, commitDate)
	}

	sb.WriteString(`}}}}}`)
	return sb.String()
}
