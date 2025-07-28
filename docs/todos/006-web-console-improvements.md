---
title: Web Management Console UI/UX Improvements
status: draft
---

# Web Management Console UI/UX Improvements

## Overview

Initial web console is working but needs several improvements for better usability:
- Thread timestamps are hard to understand
- Channel IDs should be displayed as channel names
- Session count shows 0 (bug)

## Tasks

### High Priority

- [ ] Convert thread_ts to timezone-aware datetime display
  - Parse Unix timestamp from thread_ts (e.g., 1753663466.387799)
  - Convert to user's local timezone
  - Display in human-readable format (e.g., "2024-12-27 14:24:26 JST")

- [ ] Display channel names instead of IDs
  - Add Slack API permission: `channels:read`
  - Implement channel info fetching using Slack API
  - Map channel IDs to names in API response

- [ ] Fix session count bug
  - Debug why sessions are not being counted correctly
  - Current query might be filtering by 'active' status only
  - Should count all sessions for each thread

### Medium Priority

- [ ] Implement channel name caching
  - In-memory cache with TTL (e.g., 1 hour)
  - Reduce API calls to Slack
  - Consider using sync.Map for thread-safe access

- [ ] Improve frontend datetime formatting
  - Use consistent date format across the app
  - Add relative time display (e.g., "2 hours ago")
  - Consider using a library like date-fns or dayjs

### Low Priority

- [ ] Better error handling and loading states
  - Show specific error messages
  - Add retry functionality
  - Improve loading indicators

## Technical Notes

### Thread Timestamp Conversion
```go
// Example: Convert thread_ts to time.Time
threadTsFloat, _ := strconv.ParseFloat(thread.ThreadTs, 64)
threadTime := time.Unix(int64(threadTsFloat), int64((threadTsFloat-math.Floor(threadTsFloat))*1e9))
```

### Slack API Permissions
Need to add to Slack app manifest:
```yaml
oauth_config:
  scopes:
    bot:
      - channels:read  # For public channels
      - groups:read    # For private channels
```

### Session Count Query Fix
Current query might be using wrong filter:
```sql
-- Current (might be wrong)
SELECT * FROM sessions
WHERE status = 'active'

-- Should be
SELECT * FROM sessions
WHERE thread_id = ?
```