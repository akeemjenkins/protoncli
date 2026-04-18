package imaputil

import (
	"testing"
)

func TestDetectAllMail(t *testing.T) {
	tests := []struct {
		name      string
		mailboxes  []MailboxInfo
		wantName  string
		wantOK    bool
	}{
		{
			name:     "empty",
			mailboxes: nil,
			wantOK:   false,
		},
		{
			name: "attr All",
			mailboxes: []MailboxInfo{
				{Name: "All Mail", Attrs: []string{"\\All"}},
			},
			wantName: "All Mail",
			wantOK:   true,
		},
		{
			name: "fallback name",
			mailboxes: []MailboxInfo{
				{Name: "All Mail", Attrs: nil},
			},
			wantName: "All Mail",
			wantOK:   true,
		},
		{
			name: "inbox only",
			mailboxes: []MailboxInfo{
				{Name: "INBOX", Attrs: nil},
			},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotOK := DetectAllMail(tt.mailboxes)
			if gotOK != tt.wantOK || gotName != tt.wantName {
				t.Errorf("DetectAllMail() = %q, %v; want %q, %v", gotName, gotOK, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestResolveMailboxName(t *testing.T) {
	mailboxes := []MailboxInfo{
		{Name: "Folders/Accounts", Delim: '/'},
		{Name: "INBOX", Delim: '/'},
	}
	tests := []struct {
		name    string
		request string
		want    string
		wantErr bool
	}{
		{"exact", "Folders/Accounts", "Folders/Accounts", false},
		{"case fold", "folders/accounts", "Folders/Accounts", false},
		{"missing", "Folders/Nope", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveMailboxName(tt.request, mailboxes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveMailboxName(%q) err=%v; wantErr=%v", tt.request, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ResolveMailboxName(%q) = %q; want %q", tt.request, got, tt.want)
			}
		})
	}
}

func TestDetectLabelsRoot(t *testing.T) {
	tests := []struct {
		name     string
		mailboxes []MailboxInfo
		wantName string
		wantOK   bool
	}{
		{
			name:     "empty",
			mailboxes: nil,
			wantOK:   false,
		},
		{
			name: "Labels folder",
			mailboxes: []MailboxInfo{
				{Name: "Labels", Delim: '/'},
			},
			wantName: "Labels",
			wantOK:   true,
		},
		{
			name: "nested Labels",
			mailboxes: []MailboxInfo{
				{Name: "Foo/Labels", Delim: '/'},
				{Name: "Labels", Delim: '/'},
			},
			wantName: "Labels", // shortest wins
			wantOK:   true,
		},
		{
			name: "no Labels",
			mailboxes: []MailboxInfo{
				{Name: "INBOX", Delim: '/'},
			},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotOK := DetectLabelsRoot(tt.mailboxes)
			if gotOK != tt.wantOK || gotName != tt.wantName {
				t.Errorf("DetectLabelsRoot() = %q, %v; want %q, %v", gotName, gotOK, tt.wantName, tt.wantOK)
			}
		})
	}
}
