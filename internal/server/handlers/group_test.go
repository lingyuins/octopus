package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	grp "github.com/lingyuins/octopus/internal/op/group"
)

func TestClassifyGroupMutationError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
		wantOK     bool
	}{
		{
			name:       "legacy schema missing endpoint_type",
			err:        errors.New("SQL logic error: table groups has no column named endpoint_type (1)"),
			wantStatus: http.StatusServiceUnavailable,
			wantMsg:    "database schema is outdated",
			wantOK:     true,
		},
		{
			name:       "duplicate group name",
			err:        errors.New("UNIQUE constraint failed: groups.name"),
			wantStatus: http.StatusConflict,
			wantMsg:    "group name already exists",
			wantOK:     true,
		},
		{
			name:       "duplicate group item",
			err:        errors.New("UNIQUE constraint failed: group_items.group_id, group_items.channel_id, group_items.model_name"),
			wantStatus: http.StatusConflict,
			wantMsg:    "group contains duplicate channel/model items",
			wantOK:     true,
		},
		{
			name:    "unexpected error",
			err:     errors.New("database is locked"),
			wantOK:  false,
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, msg, ok := classifyGroupMutationError(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%t, got %t", tt.wantOK, ok)
			}
			if status != tt.wantStatus {
				t.Fatalf("expected status=%d, got %d", tt.wantStatus, status)
			}
			if tt.wantMsg == "" {
				if msg != "" {
					t.Fatalf("expected empty message, got %q", msg)
				}
				return
			}
			if !strings.Contains(msg, tt.wantMsg) {
				t.Fatalf("expected message containing %q, got %q", tt.wantMsg, msg)
			}
		})
	}
}

func TestGetGroupListEncodesEmptyItemsAsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	group := &model.Group{
		Name:         "empty-items",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := grp.GroupCreate(group, ctx); err != nil {
		t.Fatalf("create group: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/group/list", nil)

	getGroupList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Data []struct {
			Items json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("unexpected response payload: %+v", response.Data)
	}
	if string(response.Data[0].Items) != "[]" {
		t.Fatalf("items = %s, want []", response.Data[0].Items)
	}
}
