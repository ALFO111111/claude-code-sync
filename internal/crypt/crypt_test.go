package crypt

import (
	"bytes"
	"testing"
)

// 加密后能原样解开；同一口令派生出同一密钥（两台设备才能互通）。
func TestSealOpenRoundTrip(t *testing.T) {
	key := DeriveKey("口令 带空格", KeySalt)
	if !bytes.Equal(key, DeriveKey("口令 带空格", KeySalt)) {
		t.Fatal("同一口令应派生出同一密钥")
	}
	plain := []byte(`{"cwd":"D:\\work\\项目"}` + "\n")
	enc, err := Seal(key, "files/abc/s1.jsonl", plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, []byte("cwd")) {
		t.Error("密文里不应出现明文内容")
	}
	got, err := Open(key, "files/abc/s1.jsonl", enc)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("解密结果不对: %v %q", err, got)
	}
}

// 口令不对、文件被挪到别的路径（调包）、不是本工具的格式，都必须解密失败。
func TestOpenRejectsWrongKeyOrPath(t *testing.T) {
	key := DeriveKey("right", KeySalt)
	enc, err := Seal(key, "files/a", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(DeriveKey("wrong", KeySalt), "files/a", enc); err == nil {
		t.Error("口令不对应解密失败")
	}
	if _, err := Open(key, "files/b", enc); err == nil {
		t.Error("对象路径不同（被调包）应解密失败")
	}
	if _, err := Open(key, "files/a", []byte("plain text")); err == nil {
		t.Error("非加密格式应报错")
	}
}
