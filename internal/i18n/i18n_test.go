package i18n

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestT(t *testing.T) {
	defer Set(ZhCN)
	Set(ZhCN)
	if got := T("项目 %s 在本机有 %d 个克隆，请用 %s claim %s <路径> 指定一个", "p", 2, "app", "p"); !strings.HasPrefix(got, "项目 p 在本机有 2 个克隆") {
		t.Errorf("中文: %q", got)
	}
	Set(En)
	if got := T("设备名不能为空"); got != "Device name cannot be empty" {
		t.Errorf("英文: %q", got)
	}
	if got := T("这句没有翻译"); got != "这句没有翻译" {
		t.Errorf("没翻译的应原样返回: %q", got)
	}
}

func TestErrIsLazy(t *testing.T) {
	defer Set(ZhCN)
	err := Err("另一个同步正在进行")
	Set(En)
	if err.Error() != en["另一个同步正在进行"] {
		t.Errorf("Err 应按调用时的语言翻译: %q", err.Error())
	}
	if !errors.Is(err, err) {
		t.Error("errors.Is 应该有效")
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"": "", "zh": ZhCN, "zh-CN": ZhCN, "zh_TW.UTF-8": ZhCN, "en-US": En, "ja-JP": En} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestTranslationsKeepVerbs 译文和原文的格式占位符（%s %d %w……）要一一对应，否则填参数会错位。
func TestTranslationsKeepVerbs(t *testing.T) {
	for zh, tr := range en {
		if a, b := verbs(zh), verbs(tr); a != b {
			t.Errorf("占位符不一致:\n  原文 %q → %s\n  译文 %q → %s", zh, a, tr, b)
		}
	}
}

func verbs(s string) string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			continue
		}
		j := i + 1
		for j < len(s) && strings.IndexByte("+-# 0123456789.", s[j]) >= 0 {
			j++
		}
		if j < len(s) {
			out = append(out, s[i:j+1])
		}
		i = j
	}
	return strings.Join(out, " ")
}

// TestCatalogCoversSource 扫描全部 Go 源码（不含测试），保证：
//  1. 每个中文字符串都经过翻译：作为 i18n.T / i18n.Err 或日志 Printf 的第一个参数
//  2. 每个要翻译的字符串在英文对照表里都有译文
//  3. 对照表里没有源码已经不用的旧条目
func TestCatalogCoversSource(t *testing.T) {
	root := filepath.Join("..", "..")
	used := map[string]bool{}
	var problems []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".test", "frontend", "build", "tools", "testutil":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || filepath.Base(p) == "en.go" {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			defer func() { stack = append(stack, n) }()
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			kind := argKind(stack, lit)
			if err != nil || !hasHan(s) && kind != "i18n" {
				return true // 不含中文、也没交给 i18n 的字符串不用管
			}
			pos := fset.Position(lit.Pos())
			if kind == "" {
				problems = append(problems, pos.String()+": 中文字符串没有经过 i18n.T / i18n.Err / 日志 Printf: "+strconv.Quote(s))
			}
			used[s] = true
			if _, ok := en[s]; !ok {
				problems = append(problems, pos.String()+": en.go 缺少译文: "+strconv.Quote(s))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k := range en {
		if !used[k] {
			problems = append(problems, "en.go 有源码里已经不用的条目: "+strconv.Quote(k))
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

// argKind 字符串字面量作为谁的第一个参数："i18n"（i18n.T / i18n.Err）、"printf"（日志对象的 .Printf，
// logging.Logger 自己会翻译；fmt.Printf 不算）；都不是返回空串。
func argKind(stack []ast.Node, lit *ast.BasicLit) string {
	if len(stack) == 0 {
		return ""
	}
	call, ok := stack[len(stack)-1].(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || call.Args[0] != ast.Expr(lit) {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, _ := sel.X.(*ast.Ident)
	switch {
	case pkg != nil && pkg.Name == "i18n" && (sel.Sel.Name == "T" || sel.Sel.Name == "Err"):
		return "i18n"
	case sel.Sel.Name == "Printf" && (pkg == nil || pkg.Name != "fmt"):
		return "printf"
	}
	return ""
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
