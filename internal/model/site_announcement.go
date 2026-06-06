package model

import "time"

// SiteAnnouncement caches a notice/announcement fetched from a remote site.
type SiteAnnouncement struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	RemoteSiteID int       `json:"remote_site_id" gorm:"not null;index"`
	Content      string    `json:"content" gorm:"type:text"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// RemoteSiteToken represents an API token on a remote site, cached locally.
type RemoteSiteToken struct {
	ID             int64      `json:"id" gorm:"primaryKey"`
	RemoteSiteID   int        `json:"remote_site_id" gorm:"not null;index"`
	RemoteTokenID  int        `json:"remote_token_id"`
	Name           string     `json:"name"`
	Key            string     `json:"key"`
	Status         int        `json:"status"`
	RemainQuota    float64    `json:"remain_quota"`
	UsedQuota      float64    `json:"used_quota"`
	UnlimitedQuota bool       `json:"unlimited_quota"`
	ModelLimits    string     `json:"model_limits"`
	ExpiredTime    int64      `json:"expired_time"`
	CreatedTime    int64      `json:"created_time"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
}

// SyncToChannelRequest is the payload for importing a remote token as a local channel.
type SyncToChannelRequest struct {
	RemoteSiteID int    `json:"remote_site_id" binding:"required"`
	TokenID      int64  `json:"token_id" binding:"required"`
	ChannelName  string `json:"channel_name"`
	Models       string `json:"models"`
}

func (SyncToChannelRequest) TableName() string { return "-" }
