## 做了什么 / What

<!-- 简单说明改动和原因 / Describe the change and why -->

## 检查 / Checklist

- [ ] `go test ./...` 通过 / passes
- [ ] `gofmt -l .` 没有输出 / prints nothing
- [ ] `cd frontend && npm run check-i18n` 通过 / passes
- [ ] 改了同步逻辑的话：想过最坏情况会不会丢用户的会话，并补了测试 / If sync logic changed: considered whether sessions could be lost in the worst case, and added tests
- [ ] 新增的界面文字 / 中文字符串都有英文翻译 / New UI text and Chinese strings have English translations
