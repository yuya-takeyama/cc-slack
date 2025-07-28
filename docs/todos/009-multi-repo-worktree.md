---
title: Multi-Repository and Multi-Worktree Support
status: draft
---

# Multi-Repository and Multi-Worktree Support

## Overview

This task implements two major features to enable parallel Claude Code operations:

1. **Multi-Repository Support**: Allow Claude Code to work with multiple repositories simultaneously
2. **Multi-Worktree Support**: Enable parallel work within a single repository using Git worktree

## Tasks

### Phase 1: Multi-Repository Foundation
- [ ] Create `repositories` table (migration)
- [ ] Implement Repository management
  - [ ] Define Repository struct and interface
  - [ ] Implement RepositoryManager
- [ ] Extend configuration schema for repository settings
- [ ] Implement AI Repository Router (Option D)
  - [ ] Manage router Claude Code process
  - [ ] Design system prompt with repository configuration
  - [ ] Define JSON Schema for response format
  - [ ] Implement routing logic using Sonnet model
  - [ ] Add caching mechanism for routing results

### Phase 2: Multi-Worktree Foundation
- [ ] Create `worktrees` table (migration)
- [ ] Implement Git worktree wrapper
  - [ ] worktree creation function
  - [ ] worktree deletion function
  - [ ] worktree status check function
- [ ] Implement WorktreeManager
- [ ] Auto-create worktree on session start

### Phase 3: Integration and Cleanup
- [ ] Update SessionManager
  - [ ] Session creation with repository and worktree
  - [ ] Update working directory resolution logic
- [ ] Implement cleanup jobs
  - [ ] Delete old worktrees
  - [ ] Monitor disk usage
- [ ] Update Web UI
  - [ ] Repository list view
  - [ ] Worktree status display

### Phase 4: Advanced Features
- [ ] Implement advanced repository selection options (B, C)
- [ ] Optimize worktree reuse
- [ ] Add concurrency limits
- [ ] Resource usage monitoring

## Design Decisions

### Repository Selection: AI Router (Option D)
- Router is activated ONLY when multiple repositories are configured for a channel
- Single repository channels bypass router for direct execution (performance optimization)
- When router is needed:
  - Use Claude Code SDK custom system prompt feature
  - Deploy Sonnet model (`--model claude-3-5-sonnet-latest`) for cost efficiency
  - Router process determines appropriate repository from message content

### Worktree Management
- 1 thread = 1 worktree principle
- Automatic cleanup after configurable period (default: 24h)
- Reuse existing worktrees for resume functionality

## References

- Full design document: `/docs/issues/multi-repo-worktree-support.md`
- [Claude Code SDK Documentation](https://docs.anthropic.com/en/docs/claude-code/sdk)
- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)