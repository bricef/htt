package tui

import (
	"testing"

	"github.com/bricef/htt/internal/storage"
)

func TestInit_ShowsTheUpdateNoticeOnTheStatusLine(t *testing.T) {
	repo := storage.NewMemoryRepository()
	ctx, err := repo.CurrentContext()
	if err != nil {
		t.Fatal(err)
	}
	m := Model(repo, ctx, Options{UpdateNotice: func() string { return "htt v9.9.9 is out" }})
	m, buf := withLogBuffer(m)

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init should wait for the update check")
	}
	result, _ := m.Update(cmd())
	if got := asModel(t, result).status; got != "htt v9.9.9 is out" {
		t.Errorf("status = %q, want the update notice", got)
	}
	assertLogged(t, buf.String(), "level=INFO", `msg="update available"`)
}

func TestInit_NothingToSayLeavesTheStatusAlone(t *testing.T) {
	repo := storage.NewMemoryRepository()
	ctx, err := repo.CurrentContext()
	if err != nil {
		t.Fatal(err)
	}
	if cmd := Model(repo, ctx, Options{}).Init(); cmd != nil {
		t.Errorf("Init with no update check should do nothing")
	}
	cmd := Model(repo, ctx, Options{UpdateNotice: func() string { return "" }}).Init()
	if msg := cmd(); msg != nil {
		t.Errorf("an empty notice should produce no message, got %v", msg)
	}
}
