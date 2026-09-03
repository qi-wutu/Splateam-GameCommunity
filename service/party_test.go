package service

import (
	"strings"
	"sync"
	"testing"

	"splatoon-backend/config"
	"splatoon-backend/models"
	"splatoon-backend/utils"
)

// mustCreateParty 直接落库一个 party 用于测试（绕过 CreateParty 的创建者自动入队，便于控制 playernum/max_num）。
func mustCreateParty(t testing.TB, maxNum, playernum int) models.Party {
	t.Helper()
	p := models.Party{
		Title:        "title",
		Game:         "splatoon",
		Introduction: "intro",
		Playernum:    playernum,
		MaxNum:       maxNum,
		OwnerID:      "1",
		OwnerName:    "owner",
	}
	if err := config.Db.Create(&p).Error; err != nil {
		t.Fatalf("create party: %v", err)
	}
	return p
}

func TestJoinParty_IncrementsPlayernumAndCreatesMember(t *testing.T) {
	newTestDB(t)
	p := mustCreateParty(t, 5, 1)

	member, err := JoinParty("userA", uintStr(p.ID))
	if err != nil {
		t.Fatalf("JoinParty: %v", err)
	}
	if member.UserID != "userA" || member.PartyID != p.ID {
		t.Fatalf("wrong member: %+v", member)
	}
	if got := reloadParty(t, p.ID).Playernum; got != 2 {
		t.Fatalf("playernum = %d, want 2", got)
	}
	if !utils.RuserinP("userA", uintStr(p.ID)) {
		t.Fatal("expected userA to be in party")
	}
}

func TestJoinParty_FullRejected(t *testing.T) {
	newTestDB(t)
	p := mustCreateParty(t, 2, 2) // 已满

	_, err := JoinParty("userA", uintStr(p.ID))
	if err == nil {
		t.Fatal("expected error joining full party, got nil")
	}
	if !strings.Contains(err.Error(), "full") {
		t.Fatalf("expected 'full' error, got: %v", err)
	}
	if got := reloadParty(t, p.ID).Playernum; got != 2 {
		t.Fatalf("playernum changed to %d, want 2", got)
	}
}

func TestJoinParty_DuplicateRejected(t *testing.T) {
	newTestDB(t)
	p := mustCreateParty(t, 5, 1)

	if _, err := JoinParty("userA", uintStr(p.ID)); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, err := JoinParty("userA", uintStr(p.ID)); err == nil {
		t.Fatal("expected duplicate join error")
	}
}

func TestJoinParty_InvalidID(t *testing.T) {
	newTestDB(t)
	if _, err := JoinParty("userA", "not-a-number"); err == nil {
		t.Fatal("expected error for invalid party id")
	}
}

// TestJoinParty_Concurrency 验证并发加入不会超员：
// max_num=5，30 个不同用户同时加入，最终恰好 5 个成功、playernum 不超过上限。
// 旧实现"先读 playernum 再 +1"存在竞态（都读到旧值），会超员；原子条件更新后此测试应稳定通过。
func TestJoinParty_Concurrency(t *testing.T) {
	newTestDB(t)
	const maxNum = 5
	const goroutines = 30
	p := mustCreateParty(t, maxNum, 0) // playernum 从 0 起

	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if _, err := JoinParty(itoa(id), uintStr(p.ID)); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if got := reloadParty(t, p.ID).Playernum; got != maxNum {
		t.Fatalf("playernum = %d, want %d（不应超过上限）", got, maxNum)
	}
	if success != maxNum {
		t.Fatalf("success = %d, want %d", success, maxNum)
	}
}

func TestLeaveParty_RemovesMemberAndDecrements(t *testing.T) {
	newTestDB(t)
	p := mustCreateParty(t, 5, 1)

	if _, err := JoinParty("userA", uintStr(p.ID)); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := LeaveParty("userA", uintStr(p.ID)); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if got := reloadParty(t, p.ID).Playernum; got != 1 {
		t.Fatalf("playernum = %d, want 1", got)
	}
	if utils.RuserinP("userA", uintStr(p.ID)) {
		t.Fatal("expected userA removed from party")
	}
}

func TestDeleteParty_OnlyOwner(t *testing.T) {
	newTestDB(t)
	p := mustCreateParty(t, 5, 1)

	if _, err := DeleteParty(uintStr(p.ID), "999"); err == nil {
		t.Fatal("expected non-owner delete to fail")
	}
	if _, err := DeleteParty(uintStr(p.ID), "1"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if err := config.Db.First(&models.Party{}, p.ID).Error; err == nil {
		t.Fatal("expected party to be deleted")
	}
}

func TestGetPartyList_PaginationDB(t *testing.T) {
	newTestDB(t)
	// 创建 3 个 party，分页大小 2 → 第 2 页应只剩 1 条且 hasMore=false。
	// 走 DB 降级分支（测试环境 RedisClient 为 nil）。
	for i := 0; i < 3; i++ {
		mustCreateParty(t, 5, 1)
	}

	page1, err := GetPartyList("", 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Items) != 2 || !page1.HasMore || page1.NextCursor == "" {
		t.Fatalf("page1 = %d items, hasMore=%v, next=%q", len(page1.Items), page1.HasMore, page1.NextCursor)
	}

	page2, err := GetPartyList(page1.NextCursor, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.HasMore {
		t.Fatalf("page2 = %d items, hasMore=%v", len(page2.Items), page2.HasMore)
	}
}

func TestPartyCursor_RoundTrip(t *testing.T) {
	cur, err := parsePartyCursor(encodePartyCursor(nowUTC(), 42))
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if cur.ID != 42 {
		t.Fatalf("id = %d, want 42", cur.ID)
	}
}

func TestOrderPartyRows_TieBreak(t *testing.T) {
	ts := nowUTC()
	build := func(id uint) models.Party {
		p := models.Party{}
		p.ID = id
		p.CreatedAt = ts
		return p
	}
	rows := map[uint]models.Party{3: build(3), 1: build(1), 2: build(2)}
	// 相同 created_at，按 id 越大越在前
	out := orderPartyRows(rows, []uint{1, 2, 3})
	if out[0].ID != 3 || out[1].ID != 2 || out[2].ID != 1 {
		t.Fatalf("tie-break order wrong: %+v", out)
	}
}
