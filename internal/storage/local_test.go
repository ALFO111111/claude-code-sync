package storage

import (
	"context"
	"testing"
)

// Put / Get / List 返回的 etag 必须一致（state 里存的是 Put 的返回值，之后和 List 比）。
func TestLocalStorageETagConsistent(t *testing.T) {
	s := &localStorage{root: t.TempDir()}
	ctx := context.Background()

	e1, err := s.Put(ctx, "sub/b.txt", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	_, e2, err := s.Get(ctx, "sub/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if e1 != e2 {
		t.Errorf("Put 和 Get 的 etag 不一致: %q vs %q", e1, e2)
	}
	objs, err := s.List(ctx, "sub/")
	if err != nil || len(objs) != 1 {
		t.Fatalf("List: %v %v", objs, err)
	}
	if objs[0].ETag != e1 {
		t.Errorf("List 和 Put 的 etag 不一致: %q vs %q", objs[0].ETag, e1)
	}
}
