package platform

import (
	"runtime"
	"testing"
)

// unschedule 重复执行不应报错（任务不存在 = 目标状态已达成）。
// 旧实现靠匹配"找不到/not found"，英文系统报错文案对不上会误报失败。
func TestRemoveScheduleIdempotent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("定时任务仅 Windows")
	}
	old := TaskName
	TaskName = "CCS-Test-NoSuchTask-000"
	defer func() { TaskName = old }()

	if err := RemoveSchedule(); err != nil {
		t.Fatalf("第一次删除不存在的任务不应报错: %v", err)
	}
	if err := RemoveSchedule(); err != nil {
		t.Fatalf("重复删除不应报错: %v", err)
	}

	oldLegacy := legacyTaskNames
	legacyTaskNames = []string{"CCS-Test-NoSuchLegacy-000"}
	defer func() { legacyTaskNames = oldLegacy }()
	if err := RemoveLegacySchedule(); err != nil {
		t.Fatalf("改名前的任务不存在时删除不应报错: %v", err)
	}
}
