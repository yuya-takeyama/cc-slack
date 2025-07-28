package slack

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

// ChannelInfo represents channel information
type ChannelInfo struct {
	ID   string
	Name string
}

// ChannelCache caches channel information
type ChannelCache struct {
	client  *slack.Client
	cache   map[string]*ChannelInfo
	mu      sync.RWMutex
	ttl     time.Duration
	lastGet map[string]time.Time
}

// NewChannelCache creates a new channel cache
func NewChannelCache(client *slack.Client, ttl time.Duration) *ChannelCache {
	return &ChannelCache{
		client:  client,
		cache:   make(map[string]*ChannelInfo),
		ttl:     ttl,
		lastGet: make(map[string]time.Time),
	}
}

// GetChannel gets channel info with cache
func (c *ChannelCache) GetChannel(ctx context.Context, channelID string) (*ChannelInfo, error) {
	// Check cache first
	c.mu.RLock()
	if info, ok := c.cache[channelID]; ok {
		if time.Since(c.lastGet[channelID]) < c.ttl {
			c.mu.RUnlock()
			return info, nil
		}
	}
	c.mu.RUnlock()

	// Fetch from Slack API
	channel, err := c.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID:         channelID,
		IncludeLocale:     false,
		IncludeNumMembers: false,
	})
	if err != nil {
		log.Error().Err(err).Str("channel_id", channelID).Msg("Failed to get channel info")
		return nil, err
	}

	info := &ChannelInfo{
		ID:   channel.ID,
		Name: channel.Name,
	}

	// Update cache
	c.mu.Lock()
	c.cache[channelID] = info
	c.lastGet[channelID] = time.Now()
	c.mu.Unlock()

	return info, nil
}

// GetChannelName gets channel name from ID
func (c *ChannelCache) GetChannelName(ctx context.Context, channelID string) string {
	info, err := c.GetChannel(ctx, channelID)
	if err != nil {
		// Return ID as fallback
		return channelID
	}
	return info.Name
}
