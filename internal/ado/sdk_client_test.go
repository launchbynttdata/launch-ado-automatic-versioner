package ado

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

func TestConvertGitRefs(t *testing.T) {
	t.Parallel()

	t.Run("annotated tag preserves ref and peeled object ids", func(t *testing.T) {
		t.Parallel()
		name := "refs/tags/v1"
		tagObject := "tag-object"
		commit := "commit-object"
		refs := convertGitRefs([]git.GitRef{{
			Name:           &name,
			ObjectId:       &tagObject,
			PeeledObjectId: &commit,
		}})
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d", len(refs))
		}
		if refs[0].Name != name || refs[0].ObjectID != tagObject || refs[0].PeeledObjectID != commit {
			t.Fatalf("unexpected ref conversion: %+v", refs[0])
		}
	})

	t.Run("lightweight tag leaves peeled object id empty", func(t *testing.T) {
		t.Parallel()
		name := "refs/tags/v1-lw"
		commit := "commit-only"
		refs := convertGitRefs([]git.GitRef{{
			Name:     &name,
			ObjectId: &commit,
		}})
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d", len(refs))
		}
		if refs[0].Name != name || refs[0].ObjectID != commit || refs[0].PeeledObjectID != "" {
			t.Fatalf("unexpected ref conversion: %+v", refs[0])
		}
	})
}

func TestErrIfRefDeleteUpdateRejected(t *testing.T) {
	t.Parallel()
	const ref = "refs/tags/floating"

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name    string
		results *[]git.GitRefUpdateResult
		wantErr bool
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: true,
		},
		{
			name:    "empty slice",
			results: &[]git.GitRefUpdateResult{},
			wantErr: true,
		},
		{
			name: "two results",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(true)},
				{Success: boolPtr(true)},
			},
			wantErr: true,
		},
		{
			name: "success false stale old object id",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(false)},
			},
			wantErr: true,
		},
		{
			name: "success nil",
			results: &[]git.GitRefUpdateResult{
				{Success: nil},
			},
			wantErr: true,
		},
		{
			name: "single success true",
			results: &[]git.GitRefUpdateResult{
				{Success: boolPtr(true)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := errIfRefDeleteUpdateRejected(tt.results, ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				const wantSub = "deleting ref refs/tags/floating rejected"
				if err.Error() != wantSub {
					t.Fatalf("error %q, want %q", err.Error(), wantSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPullRequestTitleFromGitPR(t *testing.T) {
	t.Parallel()

	title := "feat: add login"
	blank := "   "

	cases := []struct {
		name    string
		pr      *git.GitPullRequest
		want    string
		wantErr bool
	}{
		{
			name: "valid title",
			pr:   &git.GitPullRequest{Title: &title},
			want: title,
		},
		{
			name:    "nil pull request",
			pr:      nil,
			wantErr: true,
		},
		{
			name:    "nil title",
			pr:      &git.GitPullRequest{},
			wantErr: true,
		},
		{
			name:    "blank title",
			pr:      &git.GitPullRequest{Title: &blank},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pullRequestTitleFromGitPR(tc.pr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCanonicalizeOrganizationURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		allowedHosts []string
		want         string
		wantErr      bool
	}{
		{
			name:  "azure devops services canonical url",
			input: "https://dev.azure.com/my-org",
			want:  "https://dev.azure.com/my-org",
		},
		{
			name:  "azure devops services with trailing slash and uppercase host",
			input: "HTTPS://DEV.AZURE.COM/my-org/",
			want:  "https://dev.azure.com/my-org",
		},
		{
			name:  "explicit default port 443 is normalized",
			input: "https://dev.azure.com:443/my-org",
			want:  "https://dev.azure.com/my-org",
		},
		{
			name:  "visualstudio.com host",
			input: "https://my-org.visualstudio.com/",
			want:  "https://my-org.visualstudio.com",
		},
		{
			name:         "rejects custom host when custom allowlist is not configured",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection/",
			allowedHosts: nil,
			wantErr:      true,
		},
		{
			name:         "accepts custom host when explicitly allowlisted",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection/",
			allowedHosts: []string{"ado.corp.example.com"},
			want:         "https://ado.corp.example.com/tfs/DefaultCollection",
		},
		{
			name:         "custom allowlist normalizes case and trailing dot",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{" ADO.CORP.EXAMPLE.COM. "},
			want:         "https://ado.corp.example.com/tfs/DefaultCollection",
		},
		{
			name:         "custom allowlist removes default dev.azure.com host",
			input:        "https://dev.azure.com/my-org",
			allowedHosts: []string{"ado.corp.example.com"},
			wantErr:      true,
		},
		{
			name:         "custom allowlist removes default visualstudio.com host",
			input:        "https://my-org.visualstudio.com",
			allowedHosts: []string{"ado.corp.example.com"},
			wantErr:      true,
		},
		{
			name:         "custom allowlist rejects unlisted custom host",
			input:        "https://other.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{"ado.corp.example.com"},
			wantErr:      true,
		},
		{
			name:    "rejects bare visualstudio.com without org subdomain",
			input:   "https://visualstudio.com/my-org",
			wantErr: true,
		},
		{
			name:    "rejects nested visualstudio.com subdomain",
			input:   "https://sub.my-org.visualstudio.com/",
			wantErr: true,
		},
		{
			name:    "rejects lookalike suffix domain",
			input:   "https://evildev.azure.com/my-org",
			wantErr: true,
		},
		{
			name:    "rejects subdomain of dev.azure.com",
			input:   "https://sub.dev.azure.com/my-org",
			wantErr: true,
		},
		{
			name:         "rejects wildcard in custom allowlist",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{"*.corp.example.com"},
			wantErr:      true,
		},
		{
			name:         "rejects scheme or path in custom allowlist entry",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{"https://ado.corp.example.com"},
			wantErr:      true,
		},
		{
			name:         "rejects port in custom allowlist entry",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{"ado.corp.example.com:8443"},
			wantErr:      true,
		},
		{
			name:         "rejects ip literal in custom allowlist entry",
			input:        "https://ado.corp.example.com/tfs/DefaultCollection",
			allowedHosts: []string{"10.0.0.5"},
			wantErr:      true,
		},
		{
			name:    "rejects empty url",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "rejects http scheme",
			input:   "http://dev.azure.com/my-org",
			wantErr: true,
		},
		{
			name:    "rejects relative url",
			input:   "/my-org",
			wantErr: true,
		},
		{
			name:    "rejects userinfo",
			input:   "https://user:pass@dev.azure.com/my-org",
			wantErr: true,
		},
		{
			name:    "rejects query string",
			input:   "https://dev.azure.com/my-org?foo=bar",
			wantErr: true,
		},
		{
			name:    "rejects force query question mark",
			input:   "https://dev.azure.com/my-org?",
			wantErr: true,
		},
		{
			name:    "rejects fragment",
			input:   "https://dev.azure.com/my-org#frag",
			wantErr: true,
		},
		{
			name:    "rejects non-443 port",
			input:   "https://dev.azure.com:8443/my-org",
			wantErr: true,
		},
		{
			name:    "rejects localhost",
			input:   "https://localhost/my-org",
			wantErr: true,
		},
		{
			name:    "rejects single-label hostname",
			input:   "https://intranet/my-org",
			wantErr: true,
		},
		{
			name:    "rejects ipv4 literal",
			input:   "https://127.0.0.1/my-org",
			wantErr: true,
		},
		{
			name:    "rejects ipv6 literal",
			input:   "https://[::1]/my-org",
			wantErr: true,
		},
		{
			name:    "rejects relative dot segments in path",
			input:   "https://dev.azure.com/my-org/../other",
			wantErr: true,
		},
		{
			name:    "rejects whitespace or backslash",
			input:   "https://dev.azure.com/my\\org",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalizeOrganizationURL(tc.input, tc.allowedHosts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q (allowedHosts=%v), got %q", tc.input, tc.allowedHosts, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q (allowedHosts=%v): %v", tc.input, tc.allowedHosts, err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewClientRejectsUntrustedHostBeforeNetworkIO(t *testing.T) {
	t.Parallel()

	var hitCount atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := NewClient(context.Background(), Config{
		OrganizationURL: "https://evil.example.com/my-org",
		Project:         "proj",
		Repository:      "repo",
		Token:           "secret-pat-canary",
	})
	if err == nil {
		t.Fatal("expected NewClient to reject untrusted organization host, got nil")
	}
	if !strings.Contains(err.Error(), "not in the trusted host allowlist") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = NewClient(context.Background(), Config{
		OrganizationURL: "https://dev.azure.com/my-org",
		AllowedHosts:    []string{"ado.corp.example.com"},
		Project:         "proj",
		Repository:      "repo",
		Token:           "secret-pat-canary",
	})
	if err == nil {
		t.Fatal("expected NewClient to reject dev.azure.com when custom AllowedHosts is set, got nil")
	}
	if !strings.Contains(err.Error(), "not in the trusted host allowlist") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = NewClient(context.Background(), Config{
		OrganizationURL: srv.URL,
		Project:         "proj",
		Repository:      "repo",
		Token:           "secret-pat-canary",
	})
	if err == nil {
		t.Fatal("expected NewClient to reject httptest IP/port URL, got nil")
	}
	if hitCount.Load() != 0 {
		t.Fatalf("expected 0 outbound HTTP requests, got %d", hitCount.Load())
	}
}
