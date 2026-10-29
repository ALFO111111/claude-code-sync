package engine

import (
	"strings"
	"testing"
)

func TestMergeText3(t *testing.T) {
	base := "one\ntwo\nthree\nfour\n"
	tests := []struct {
		name          string
		local, remote string
		want          string
		conflict      bool
	}{
		{name: "different lines", local: "ONE\ntwo\nthree\nfour\n", remote: "one\ntwo\nthree\nFOUR\n", want: "ONE\ntwo\nthree\nFOUR\n"},
		{name: "same edit", local: "one\nTWO\nthree\nfour\n", remote: "one\nTWO\nthree\nfour\n", want: "one\nTWO\nthree\nfour\n"},
		{name: "same line conflict", local: "one\nLOCAL\nthree\nfour\n", remote: "one\nREMOTE\nthree\nfour\n", conflict: true},
		{name: "different insert positions", local: "one\nlocal\ntwo\nthree\nfour\n", remote: "one\ntwo\nthree\nremote\nfour\n", want: "one\nlocal\ntwo\nthree\nremote\nfour\n"},
		{name: "same insert different", local: "one\nlocal\ntwo\nthree\nfour\n", remote: "one\nremote\ntwo\nthree\nfour\n", conflict: true},
		{name: "delete versus unchanged", local: "one\nthree\nfour\n", remote: base, want: "one\nthree\nfour\n"},
		{name: "delete versus edit", local: "one\nthree\nfour\n", remote: "one\nTWO\nthree\nfour\n", conflict: true},
		{name: "adjacent edits", local: "ONE\ntwo\nthree\nfour\n", remote: "one\nTWO\nthree\nfour\n", want: "ONE\nTWO\nthree\nfour\n"},
		{name: "remote only", local: base, remote: "remote\n", want: "remote\n"},
		{name: "local only", local: "local\n", remote: base, want: "local\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conflict := mergeText3([]byte(base), []byte(tt.local), []byte(tt.remote))
			if conflict != tt.conflict {
				t.Fatalf("conflict=%v want %v", conflict, tt.conflict)
			}
			if !conflict && string(got) != tt.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestMergeText3FinalNewlineAndBinary(t *testing.T) {
	got, conflict := mergeText3([]byte("a\nb"), []byte("A\nb"), []byte("a\nB"))
	if conflict || string(got) != "A\nB" {
		t.Fatalf("got %q conflict=%v", got, conflict)
	}
	if _, conflict := mergeText3([]byte("a"), []byte{0}, []byte("b")); !conflict {
		t.Fatal("binary should conflict")
	}
}

func TestMergeText3TransitiveConflict(t *testing.T) {
	base := "1\n2\n3\n4\n5\n6\n"
	local := "1\nL\n3\nL\n5\n6\n"
	remote := "1\nR\nR\nR\n5\n6\n"
	if _, conflict := mergeText3([]byte(base), []byte(local), []byte(remote)); !conflict {
		t.Fatal("overlapping edit chain should conflict")
	}
}

func TestMergeJSON3Objects(t *testing.T) {
	base := []byte(`{"model":"local","a":1,"remove":true}`)
	local := []byte(`{"model":"local","a":2,"remove":true}`)
	remote := []byte(`{"model":"local","a":1,"remote":3}`)
	got, conflict, err := mergeJSON3(base, local, remote, false)
	if err != nil || conflict {
		t.Fatalf("conflict=%v err=%v", conflict, err)
	}
	want := "{\n  \"a\": 2,\n  \"model\": \"local\",\n  \"remote\": 3\n}\n"
	if string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestMergeJSON3ConflictAndDeletion(t *testing.T) {
	base := []byte(`{"a":1}`)
	if _, conflict, err := mergeJSON3(base, []byte(`{"a":2}`), []byte(`{"a":3}`), false); err != nil || !conflict {
		t.Fatalf("same field should conflict: %v %v", conflict, err)
	}
	got, conflict, err := mergeJSON3([]byte(`{"a":1,"b":2}`), []byte(`{"b":2}`), []byte(`{"a":1,"b":2}`), false)
	if err != nil || conflict || strings.Contains(string(got), `"a"`) {
		t.Fatalf("field deletion failed: %s conflict=%v err=%v", got, conflict, err)
	}
	if _, conflict, _ := mergeJSON3([]byte(`{"a":1,"b":2}`), []byte(`{"b":2}`), []byte(`{"a":2,"b":2}`), false); !conflict {
		t.Fatal("delete versus edit should conflict")
	}
}

func TestMergeJSON3KeyedArrays(t *testing.T) {
	base := []byte(`[{"id":"1","status":"todo","text":"a"}]`)
	local := []byte(`[{"id":"1","status":"done","text":"a"},{"id":"2","status":"todo","text":"local"}]`)
	remote := []byte(`[{"id":"1","status":"todo","text":"remote"}]`)
	got, conflict, err := mergeJSON3(base, local, remote, true)
	if err != nil || conflict {
		t.Fatalf("conflict=%v err=%v", conflict, err)
	}
	text := string(got)
	if !strings.Contains(text, `"status": "done"`) || !strings.Contains(text, `"text": "remote"`) || !strings.Contains(text, `"id": "2"`) {
		t.Fatalf("bad keyed merge: %s", text)
	}
	if _, conflict, _ := mergeJSON3(base, local, remote, false); !conflict {
		t.Fatal("plain arrays should not be guessed")
	}
}

func TestMergeJSON3RejectsInvalid(t *testing.T) {
	if _, _, err := mergeJSON3([]byte(`{}`), []byte(`{}`), []byte(`{} {}`), false); err == nil {
		t.Fatal("trailing JSON should fail")
	}
	if _, _, err := mergeJSON3([]byte(`{`), []byte(`{}`), []byte(`{}`), false); err == nil {
		t.Fatal("invalid JSON should fail")
	}
}
