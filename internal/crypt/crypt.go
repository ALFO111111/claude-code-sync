// Package crypt 云端文件的加解密：口令 → Argon2id 密钥，内容 gzip 后 AES-256-GCM 加密。
package crypt

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"

	"github.com/ALFO111111/agent-sync/internal/i18n"
)

// 云端文件格式: "CCS1" | 12字节随机数 | AES-256-GCM(gzip(明文))
// 对象路径作为附加认证数据，防止两个文件被调包。
var magic = []byte("CCS1")

// 下面三处的 "claude-code-sync" 是改名前的旧名字，属于云端数据格式，不能跟着改名：
// 改了口令校验就对不上、派生出的密钥也会变，云端旧数据全解不开。

// KeycheckPlain 口令校验文件的明文：用同一口令能解开它，说明两台设备口令一致。
const KeycheckPlain = "claude-code-sync keycheck v1"

// KeySalt 派生密钥用的盐。固定值，改了所有设备派生出的密钥都会变、云端旧数据全解不开。
const KeySalt = "claude-code-sync"

// DeriveKey 用 Argon2id 从口令派生密钥。同一口令 + 同一盐 = 同一密钥，所以各设备只需输入相同口令。
func DeriveKey(passphrase, salt string) []byte {
	s := sha256.Sum256([]byte("github.com/ALFO111111/claude-code-sync/v1/" + salt))
	return argon2.IDKey([]byte(passphrase), s[:16], 3, 64*1024, 4, 32)
}

// Seal 压缩并加密。aad 传云端对象路径，防止文件被调包。
func Seal(key []byte, aad string, plain []byte) ([]byte, error) {
	var zbuf bytes.Buffer
	zw := gzip.NewWriter(&zbuf)
	if _, err := zw.Write(plain); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{}, magic...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, zbuf.Bytes(), []byte(aad)), nil
}

// Open 解密并解压，Seal 的逆操作。
func Open(key []byte, aad string, data []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(data) < len(magic)+gcm.NonceSize() || !bytes.Equal(data[:len(magic)], magic) {
		return nil, errors.New(i18n.T("不是 agent-sync 加密格式"))
	}
	data = data[len(magic):]
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	z, err := gcm.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return nil, errors.New(i18n.T("解密失败（口令不对或文件被篡改）"))
	}
	zr, err := gzip.NewReader(bytes.NewReader(z))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// SHA256Hex 返回内容的 sha256 十六进制串。
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
