---
title: Database Schema Refactoring - Move working_directory to threads
status: in_progress
---

# Database Schema Refactoring

## Overview

Refactor database schema to improve data consistency and simplify the architecture:
- Move `working_directory` from `sessions` to `threads` table
- Remove `working_directories` table in favor of configuration file

## Rationale

1. **Working Directory Consistency**: The same thread should maintain consistent working directory throughout its lifecycle
2. **Configuration Simplification**: Channel-specific working directories should be managed via config file, not database
3. **Thread-level Settings**: Enable future expansion of thread-specific settings

## Tasks

### 1. Database Migration

- [ ] Create migration to add `working_directory` column to `threads` table
- [ ] Create migration to migrate existing data from `sessions.working_directory` to `threads.working_directory`
- [ ] Create migration to remove `working_directory` from `sessions` table
- [ ] Create migration to drop `working_directories` table

### 2. Code Updates

- [ ] Update sqlc queries:
  - [ ] Modify thread queries to include `working_directory`
  - [ ] Remove `working_directory` from session queries
  - [ ] Remove all `working_directories` related queries
- [ ] Update session creation logic:
  - [ ] Store `working_directory` in thread on first creation
  - [ ] Retrieve `working_directory` from thread for subsequent sessions
- [ ] Update session resume logic:
  - [ ] Get `working_directory` from thread instead of previous session
- [ ] Remove all references to `working_directories` table
- [ ] Update configuration to support channel-specific working directories

### 3. Testing

- [ ] Test session creation with new schema
- [ ] Test session resume functionality
- [ ] Test data migration on existing database
- [ ] Verify backward compatibility

## Implementation Details

### New Thread Table Structure
```sql
CREATE TABLE threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    working_directory TEXT NOT NULL,  -- Moved from sessions
    settings JSON,                    -- For future expansions
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);
```

### Configuration File Structure
```yaml
working_directories:
  default: "/Users/yuya/src/github.com/yuya-takeyama/cc-slack"
  channels:
    C12345678: "/path/to/project1"
    C87654321: "/path/to/project2"
```

## Impact Analysis

- **Session Manager**: Major changes to handle working directory at thread level
- **Database queries**: All session-related queries need updates
- **Configuration**: New structure for working directory settings
- **Existing data**: Migration required for production databases