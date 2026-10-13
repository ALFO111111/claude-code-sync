// Package i18n 界面、命令行、日志文字的多语言。
//
// 源语言是中文：代码里直接写中文原文当 key（和 gettext 一个思路），
// 其他语言在对照表（en.go）里按原文查译文，查不到就原样显示中文——不会出错，只是没翻译。
//
//	i18n.T("设备名不能为空")                        → 当前语言的文字
//	i18n.T("项目 %s 在本机有 %d 个克隆", pid, n)     → 先翻译格式串，再填参数
//	fmt.Errorf(i18n.T("连接失败: %w"), err)          → 要包 %w 时只翻译格式串，参数交给 Errorf
//	var ErrX = i18n.Err("另一个同步正在进行")         → 包级变量：用到时才翻译（启动时语言还没设好）
//	log.Printf("发现项目 %s", ...)                   → logging.Logger 自己会翻译格式串，不用包
//
// 新增中文字符串后要在 en.go 补一条英文，i18n_test.go 会扫描全部源码检查有没有漏。
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

const (
	ZhCN = "zh-CN"
	En   = "en"
)

// Supported 支持的语言（界面下拉框的顺序）。
var Supported = []string{ZhCN, En}

// catalogs 语言 → 中文原文 → 译文。中文不需要表。
var catalogs = map[string]map[string]string{En: en}

// cur 当前语言。默认中文：测试结果不随机器语言变化；程序启动时再按配置或系统语言 Set。
var cur atomic.Value

func init() { cur.Store(ZhCN) }

// Normalize 把各种写法归成支持的语言：zh / zh-CN / zh_TW.UTF-8 → zh-CN，其他 → en。空串原样返回（表示"跟随系统"）。
func Normalize(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return ZhCN
	}
	return En
}

// Set 切换语言。空串 = 跟随系统。
func Set(lang string) {
	l := Normalize(lang)
	if l == "" {
		l = Detect()
	}
	cur.Store(l)
}

// Lang 当前语言。
func Lang() string { return cur.Load().(string) }

// Detect 系统界面语言（中文系统 → zh-CN，其他 → en）。
func Detect() string {
	if l := Normalize(systemLanguage()); l != "" {
		return l
	}
	return En
}

// T 把中文原文翻成当前语言。带参数时按格式串填参数（格式串先翻译）。
func T(format string, args ...any) string {
	s := format
	if l := Lang(); l != ZhCN {
		if t, ok := catalogs[l][format]; ok {
			s = t
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Err 用到时才翻译的错误，给包级变量用（errors.Is / == 比较照常有效）。
func Err(format string, args ...any) error { return &lazyErr{format, args} }

type lazyErr struct {
	format string
	args   []any
}

func (e *lazyErr) Error() string { return T(e.format, e.args...) }
