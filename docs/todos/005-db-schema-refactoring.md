---
title: Database Schema Refactoring - Move working_directory to threads
status: done
---

# Database Schema Refactoring

## Overview

Refactor database schema to improve data consistency:
- Move `working_directory` from `sessions` to `threads` table
- Remove unused `working_directories` table

## Rationale

**Working Directory Consistency**: The same thread should maintain consistent working directory throughout its lifecycle

## Tasks

### 1. Database Migration

- [x] Create migration to add `working_directory` column to `threads` table
- [x] Create migration to migrate existing data from `sessions.working_directory` to `threads.working_directory`
- [x] Create migration to remove `working_directory` from `sessions` table
- [x] Create migration to drop `working_directories` table

### 2. Code Updates

- [x] Update sqlc queries:
  - [x] Modify thread queries to include `working_directory`
  - [x] Remove `working_directory` from session queries
  - [x] Delete `working_directories.sql` query file
- [x] Update session creation logic:
  - [x] Store `working_directory` in thread on first creation
  - [x] Retrieve `working_directory` from thread for subsequent sessions
- [x] Update session resume logic:
  - [x] Get `working_directory` from thread instead of previous session
- [x] Remove all references to `working_directories` table in the codebase

### 3. Testing

- [x] Test session creation with new schema
- [x] Test session resume functionality
- [x] Test data migration on existing database
- [x] Verify backward compatibility

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