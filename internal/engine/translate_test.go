package engine

import "testing"

func TestTranslatorSameOSAndCrossOS(t *testing.T) {
	cases := []struct {
		name           string
		foreign, local string
		in, want       string
	}{
		{"Windows→Windows 反斜杠", `D:\code\x`, `E:\work\x`,
			`{"cwd":"D:\\code\\x","f":"D:\\code\\x\\src\\a.go"}`, `{"cwd":"E:\\work\\x","f":"E:\\work\\x\\src\\a.go"}`},
		{"Windows→Windows 正斜杠写法保持", `D:\code\x`, `E:\work\x`,
			`{"f":"D:/code/x/src/a.go"}`, `{"f":"E:/work/x/src/a.go"}`},
		{"Windows→Mac 后半段分隔符也换", `D:\code\x`, `/Users/me/code/x`,
			`{"cwd":"D:\\code\\x","f":"D:\\code\\x\\src\\a.go","g":"d:/code/x/b.go"}`,
			`{"cwd":"/Users/me/code/x","f":"/Users/me/code/x/src/a.go","g":"/Users/me/code/x/b.go"}`},
		{"Mac→Windows", `/Users/me/code/x`, `D:\code\x`,
			`{"cwd":"/Users/me/code/x","f":"/Users/me/code/x/src/a.go"}`, `{"cwd":"D:\\code\\x","f":"D:\\code\\x\\src\\a.go"}`},
		{"Mac→Mac", `/Users/a/x`, `/Users/b/x`,
			`{"f":"/Users/a/x/y"}`, `{"f":"/Users/b/x/y"}`},
		{"相似前缀的兄弟目录不动", `D:\code\x`, `/Users/me/x`,
			`{"f":"D:\\code\\x2\\a","g":"D:\\code\\xy"}`, `{"f":"D:\\code\\x2\\a","g":"D:\\code\\xy"}`},
		{"正文中间的路径不动", `D:\code\x`, `/Users/me/x`,
			`{"text":"cd D:\\code\\x && ls"}`, `{"text":"cd D:\\code\\x && ls"}`},
		{"字符串里带转义引号", `D:\code\x`, `/Users/me/x`,
			`{"f":"D:\\code\\x\\a \"b\".txt","n":1}`, `{"f":"/Users/me/x/a \"b\".txt","n":1}`},
		{"Git Bash 写法 → Mac", `D:\code\x`, `/Users/me/x`,
			`{"cwd":"/d/code/x","f":"/D/code/x/src/a.go"}`, `{"cwd":"/Users/me/x","f":"/Users/me/x/src/a.go"}`},
		{"Git Bash 写法 → Windows 保持 Git Bash 写法", `D:\code\x`, `E:\work\x`,
			`{"cwd":"/d/code/x/src"}`, `{"cwd":"/e/work/x/src"}`},
		{"Git Bash 写法的兄弟目录不动", `D:\code\x`, `/Users/me/x`,
			`{"cwd":"/d/code/x2"}`, `{"cwd":"/d/code/x2"}`},
		{"同一台电脑同一路径的 Git Bash 写法不动", `D:\code\x`, `d:\code\x`,
			`{"cwd":"/d/code/x"}`, `{"cwd":"/d/code/x"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(newTranslator([]string{c.foreign}, c.local).apply([]byte(c.in)))
			if got != c.want {
				t.Fatalf("\n得到 %s\n想要 %s", got, c.want)
			}
		})
	}
}

func TestPairTranslatorLongestFirst(t *testing.T) {
	tr := newPairTranslator([][2]string{{`C:\Users\a`, `/Users/b`}, {`C:\Users\a\proj`, `/Volumes/work/proj`}})
	got := string(tr.apply([]byte(`{"x":"C:\\Users\\a\\proj\\m.go","y":"C:\\Users\\a\\.codex\\m"}`)))
	want := `{"x":"/Volumes/work/proj/m.go","y":"/Users/b/.codex/m"}`
	if got != want {
		t.Fatalf("得到 %s，想要 %s", got, want)
	}
}
