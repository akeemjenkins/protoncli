package imapthread

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC) }

func TestBuild_Empty(t *testing.T) {
	if got := Build(nil); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

func TestBuild_ReferencesChain(t *testing.T) {
	envs := []Envelope{
		{UID: 1, MessageID: "<a@x>", Subject: "Hello", From: "alice@x.com", Date: day(1)},
		{UID: 2, MessageID: "<b@x>", InReplyTo: "<a@x>", Subject: "Re: Hello", From: "bob@x.com", Date: day(2)},
		{UID: 3, MessageID: "<c@x>", InReplyTo: "<b@x>", References: []string{"<a@x>", "<b@x>"},
			Subject: "Re: Hello", From: "alice@x.com", Date: day(3)},
	}
	threads := Build(envs)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	tr := threads[0]
	if tr.Count != 3 {
		t.Errorf("count=%d, want 3", tr.Count)
	}
	if tr.Root == nil || tr.Root.UID != 1 {
		t.Errorf("root UID=%v, want 1", tr.Root)
	}
	if !reflect.DeepEqual(tr.UIDs, []uint32{1, 2, 3}) {
		t.Errorf("UIDs=%v", tr.UIDs)
	}
	if tr.Subject != "Hello" {
		t.Errorf("subject=%q", tr.Subject)
	}
	if len(tr.Participants) != 2 {
		t.Errorf("participants=%v", tr.Participants)
	}
	// The root should have one child b, which in turn has c.
	if len(tr.Root.Children) != 1 || tr.Root.Children[0].UID != 2 {
		t.Errorf("unexpected child graph: %+v", tr.Root.Children)
	}
	if len(tr.Root.Children[0].Children) != 1 || tr.Root.Children[0].Children[0].UID != 3 {
		t.Errorf("unexpected grand-children: %+v", tr.Root.Children[0].Children)
	}
}

func TestBuild_SubjectFallback(t *testing.T) {
	// No references → subject-normalized fallback should still merge.
	envs := []Envelope{
		{UID: 1, MessageID: "<a@x>", Subject: "Lunch?", From: "a@x.com", Date: day(1)},
		{UID: 2, MessageID: "<b@x>", Subject: "Re: Lunch?", From: "b@x.com", Date: day(2)},
		{UID: 3, MessageID: "<c@x>", Subject: "FWD: RE: Lunch?", From: "c@x.com", Date: day(3)},
	}
	threads := Build(envs)
	if len(threads) != 1 {
		t.Fatalf("want 1 thread, got %d", len(threads))
	}
	if threads[0].Count != 3 {
		t.Errorf("want 3 messages, got %d", threads[0].Count)
	}
}

func TestBuild_SeparateThreads(t *testing.T) {
	envs := []Envelope{
		{UID: 10, MessageID: "<t1@x>", Subject: "Alpha", From: "a@x.com", Date: day(1)},
		{UID: 11, MessageID: "<t1r@x>", InReplyTo: "<t1@x>", Subject: "Re: Alpha", From: "b@x.com", Date: day(2)},
		{UID: 20, MessageID: "<t2@x>", Subject: "Beta", From: "c@x.com", Date: day(5)},
	}
	threads := Build(envs)
	if len(threads) != 2 {
		t.Fatalf("want 2 threads, got %d", len(threads))
	}
	// Sorted by FirstDate ascending.
	if threads[0].Subject != "Alpha" || threads[1].Subject != "Beta" {
		t.Errorf("order: %q / %q", threads[0].Subject, threads[1].Subject)
	}
}

func TestBuild_NoSubjectNoReferencesStaysSeparate(t *testing.T) {
	envs := []Envelope{
		{UID: 1, MessageID: "<a@x>", Subject: "", From: "a@x.com", Date: day(1)},
		{UID: 2, MessageID: "<b@x>", Subject: "", From: "b@x.com", Date: day(2)},
	}
	threads := Build(envs)
	if len(threads) != 2 {
		t.Fatalf("empty subjects should not merge; got %d threads", len(threads))
	}
}

func TestBuild_MissingParentRefAlone(t *testing.T) {
	// Orphan reply whose InReplyTo points to a message not present: should
	// appear as its own single-node thread.
	envs := []Envelope{
		{UID: 1, MessageID: "<orphan@x>", InReplyTo: "<missing@x>",
			Subject: "Re: Something", From: "a@x.com", Date: day(1)},
	}
	threads := Build(envs)
	if len(threads) != 1 || threads[0].Count != 1 {
		t.Fatalf("unexpected: %+v", threads)
	}
	if threads[0].Root.UID != 1 {
		t.Errorf("root not the orphan")
	}
}

func TestNormalizeSubject(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello", "hello"},
		{"Re: Hello", "hello"},
		{"RE: hello", "hello"},
		{"Re[2]: Hello", "hello"},
		{"Fwd: Re: Hello", "hello"},
		{"FW: Hello", "hello"},
		{"", ""},
		{"  spaced   out  ", "spaced out"},
		{"Re:   Hello  world", "hello world"},
	}
	for _, c := range cases {
		got := NormalizeSubject(c.in)
		if got != c.want {
			t.Errorf("NormalizeSubject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseReferences(t *testing.T) {
	refs := ParseReferences("<a@x> <b@x>\n <c@x> <a@x>")
	if !reflect.DeepEqual(refs, []string{"<a@x>", "<b@x>", "<c@x>"}) {
		t.Errorf("unexpected: %v", refs)
	}
	if got := ParseReferences(""); got != nil {
		t.Errorf("empty should be nil: %v", got)
	}
	if got := ParseReferences("garbage no brackets"); len(got) != 0 {
		t.Errorf("no refs should be empty: %v", got)
	}
}

func TestBuild_DuplicateMessageIDs(t *testing.T) {
	// Two messages with the same Message-ID (e.g. cross-folder copies) should
	// land in the same thread but not crash.
	envs := []Envelope{
		{UID: 1, MessageID: "<dup@x>", Subject: "Hi", From: "a@x.com", Date: day(1)},
		{UID: 2, MessageID: "<dup@x>", Subject: "Hi", From: "a@x.com", Date: day(1)},
		{UID: 3, MessageID: "<reply@x>", InReplyTo: "<dup@x>", Subject: "Re: Hi", From: "b@x.com", Date: day(2)},
	}
	threads := Build(envs)
	if len(threads) != 1 {
		t.Fatalf("want 1 thread, got %d", len(threads))
	}
	if threads[0].Count != 3 {
		t.Errorf("want 3, got %d", threads[0].Count)
	}
	// UIDs sorted.
	if !sort.SliceIsSorted(threads[0].UIDs, func(i, j int) bool { return threads[0].UIDs[i] < threads[0].UIDs[j] }) {
		t.Errorf("UIDs not sorted: %v", threads[0].UIDs)
	}
}

func TestFormatDateEmpty(t *testing.T) {
	if formatDate(time.Time{}) != "" {
		t.Error("zero time should format to empty string")
	}
	got := formatDate(day(1))
	if got == "" || got[:4] != "2026" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestUnionFind(t *testing.T) {
	u := newUnionFind(5)
	u.union(0, 1)
	u.union(1, 2)
	u.union(3, 4)
	if u.find(0) != u.find(2) {
		t.Error("0 and 2 should be in same set")
	}
	if u.find(0) == u.find(3) {
		t.Error("0 and 3 should NOT be in same set")
	}
	// Already-unioned pair.
	u.union(0, 2)
	// Cross-rank branches.
	u.union(0, 4)
	if u.find(1) != u.find(4) {
		t.Error("all should be merged now")
	}
}
