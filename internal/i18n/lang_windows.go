package i18n

import "golang.org/x/sys/windows"

// systemLanguage Windows 的界面显示语言（设置 → 时间和语言 → Windows 显示语言），如 "zh-CN"、"en-US"。
func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
