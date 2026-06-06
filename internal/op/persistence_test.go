package op

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	st "github.com/lingyuins/octopus/internal/op/stats"
)

func TestChannelKeySaveDB_RequeuesDirtyIDsOnWriteFailure(t *testing.T) {
	restore := snapshotChannelKeyPersistenceState()
	defer restore()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	channelKeyCache.Set(7, model.ChannelKey{ID: 7, ChannelID: 3, Enabled: true, ChannelKey: "sk-test"})
	channelKeyCacheNeedUpdateLock.Lock()
	channelKeyCacheNeedUpdate[7] = struct{}{}
	channelKeyCacheNeedUpdateLock.Unlock()

	if err := db.Close(); err != nil {
		t.Fatalf("close db before failure simulation: %v", err)
	}

	if err := ChannelKeySaveDB(context.Background()); err == nil {
		t.Fatal("ChannelKeySaveDB() error = nil, want write failure")
	}

	channelKeyCacheNeedUpdateLock.Lock()
	_, ok := channelKeyCacheNeedUpdate[7]
	channelKeyCacheNeedUpdateLock.Unlock()
	if !ok {
		t.Fatal("dirty channel key id 7 was not requeued after write failure")
	}
}

func TestStatsSaveDB_RequeuesDirtyIDsOnWriteFailure(t *testing.T) {
	restore := snapshotStatsPersistenceState()
	defer restore()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	st.ResetCachesForTest(
		model.StatsTotal{ID: 1},
		model.StatsDaily{Date: time.Now().Format("20060102")},
		11, 22, 33,
	)

	if err := db.Close(); err != nil {
		t.Fatalf("close db before failure simulation: %v", err)
	}

	if err := StatsSaveDB(context.Background()); err == nil {
		t.Fatal("StatsSaveDB() error = nil, want write failure")
	}

	channelDirtyIDs := st.GetChannelDirtyIDs()
	if !containsInt(channelDirtyIDs, 11) {
		t.Fatal("channel dirty id 11 was not requeued after write failure")
	}

	modelDirtyIDs := st.GetModelDirtyIDs()
	if !containsInt(modelDirtyIDs, 22) {
		t.Fatal("model dirty id 22 was not requeued after write failure")
	}

	apiKeyDirtyIDs := st.GetAPIKeyDirtyIDs()
	if !containsInt(apiKeyDirtyIDs, 33) {
		t.Fatal("api key dirty id 33 was not requeued after write failure")
	}
}

func snapshotChannelKeyPersistenceState() func() {
	oldKeys := channelKeyCache.GetAll()
	channelKeyCacheNeedUpdateLock.Lock()
	oldDirty := make(map[int]struct{}, len(channelKeyCacheNeedUpdate))
	for id := range channelKeyCacheNeedUpdate {
		oldDirty[id] = struct{}{}
	}
	channelKeyCacheNeedUpdateLock.Unlock()

	return func() {
		channelKeyCache.Clear()
		for id, key := range oldKeys {
			channelKeyCache.Set(id, key)
		}

		channelKeyCacheNeedUpdateLock.Lock()
		for id := range channelKeyCacheNeedUpdate {
			delete(channelKeyCacheNeedUpdate, id)
		}
		for id := range oldDirty {
			channelKeyCacheNeedUpdate[id] = struct{}{}
		}
		channelKeyCacheNeedUpdateLock.Unlock()
	}
}

func containsInt(slice []int, target int) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

func snapshotStatsPersistenceState() func() {
	oldTotal := st.TotalGet()
	oldDaily := st.TodayGet()
	st.ClearAllCachesForTest()
	return func() {
		st.ClearAllCachesForTest()
		st.ResetCachesForTest(oldTotal, oldDaily, 0, 0, 0)
	}
}



