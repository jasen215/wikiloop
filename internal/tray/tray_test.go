//go:build darwin

package tray

import "testing"

func TestLabelsForZh(t *testing.T) {
	l := labelsFor("zh")
	if l.OpenDashboard != "打开控制台" {
		t.Errorf("expected 打开控制台, got %q", l.OpenDashboard)
	}
	if l.Quit != "退出" {
		t.Errorf("expected 退出, got %q", l.Quit)
	}
}

func TestLabelsForEn(t *testing.T) {
	l := labelsFor("en")
	if l.OpenDashboard != "Open Dashboard" {
		t.Errorf("expected Open Dashboard, got %q", l.OpenDashboard)
	}
	if l.Quit != "Quit" {
		t.Errorf("expected Quit, got %q", l.Quit)
	}
}

func TestLabelsForUnknownFallsBackToEn(t *testing.T) {
	l := labelsFor("fr")
	if l.Quit != "Quit" {
		t.Errorf("unknown lang should fall back to en, got %q", l.Quit)
	}
}
