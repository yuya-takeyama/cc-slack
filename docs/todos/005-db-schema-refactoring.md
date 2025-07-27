---
title: Database Schema Refactoring - Move working_directory to threads
status: in_progress
---

# Database Schema Refactoring

## Overview

Refactor database schema to improve data consistency:
- Move `working_directory` from `sessions` to `threads` table

## Rationale

**Working Directory Consistency**: The same thread should maintain consistent working directory throughout its lifecycle

## Tasks

### 1. Database Migration

- [ ] Create migration to add `working_directory` column to `threads` table
- [ ] Create migration to migrate existing data from `sessions.working_directory` to `threads.working_directory`
- [ ] Create migration to remove `working_directory` from `sessions` table

### 2. Code Updates

- [ ] Update sqlc queries:
  - [ ] Modify thread queries to include `working_directory`
  - [ ] Remove `working_directory` from session queries
- [ ] Update session creation logic:
  - [ ] Store `working_directory` in thread on first creation
  - [ ] Retrieve `working_directory` from thread for subsequent sessions
- [ ] Update session resume logic:
  - [ ] Get `working_directory` from thread instead of previous session

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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);
```

## Impact Analysis

- **Session Manager**: Major changes to handle working directory at thread level
- **Database queries**: All session-related queries need updates
- **Existing data**: Migration required for production databases