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
	ID        string
	Name      string
	IsChannel bool
	IsGroup   bool
	IsIM      bool
	IsMpim    bool
	IsPrivate bool
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
		ID:        channel.ID,
		Name:      channel.Name,
		IsChannel: channel.IsChannel,
		IsGroup:   channel.IsGroup,
		IsIM:      channel.IsIM,
		IsMpim:    channel.IsMpIM,
		IsPrivate: channel.IsPrivate,
	}

	// Update cache
	c.mu.Lock()
	c.cache[channelID] = info
	c.lastGet[channelID] = time.Now()
	c.mu.Unlock()

	return info, nil
}

// GetChannelName gets channel name from ID with appropriate prefix
func (c *ChannelCache) GetChannelName(ctx context.Context, channelID string) string {
	info, err := c.GetChannel(ctx, channelID)
	if err != nil {
		// Return ID as fallback
		return channelID
	}

	// Handle different channel types
	switch {
	case info.IsIM:
		// Direct message - for now just show the channel ID
		// TODO: Could fetch user info to show @username
		return "Direct Message"
	case info.IsMpim:
		// Multi-party IM (group DM)
		return "Group DM"
	case info.IsPrivate:
		// Private channel
		if info.Name != "" {
			return "🔒" + info.Name
		}
		return "Private Channel"
	case info.IsChannel || info.IsGroup:
		// Public channel or private group
		if info.Name != "" {
			return "#" + info.Name
		}
		return channelID
	default:
		// Unknown type, return name if available
		if info.Name != "" {
			return info.Name
		}
		return channelID
	}
}
