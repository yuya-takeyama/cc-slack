---
title: Multi-repository support with simple UI
status: draft
---

# Multi-repository Support

## Overview

Enable cc-slack to work with multiple repositories through a simple Slack modal interface. Users can select which repository to work with when starting a Claude session.

## Design Decisions

### Repository Selection Approach

**Chosen: Modal UI with explicit selection**
- Use Slack's Block Kit to create a modal when user types `/cc` command
- User explicitly selects repository from dropdown
- Initial prompt can be entered in the same modal

**Why not LLM Router (from PR #37)?**
- Too complex for initial implementation
- Hard to debug when misrouting happens
- Requires significant prompt engineering
- Can lead to unpredictable behavior

### Repository Configuration

**Store in configuration file (config.yaml or environment)**
```yaml
repositories:
  - name: "cc-slack"
    path: "/Users/yuya/src/github.com/yuya-takeyama/cc-slack"
    description: "Claude Code Slack integration"
  - name: "my-project"
    path: "/Users/yuya/src/github.com/yuya-takeyama/my-project"
    description: "Another project"
```

**Why not database?**
- Repository paths can change easily (user moves directories)
- Configuration is more transparent and version-controllable
- Easier to manage in deployment scenarios
- No need for complex migrations when paths change

### Repository-Thread Relationship

**1 Thread = 1 Repository**
- Each Slack thread is bound to a single repository
- Repository is selected when the thread is created (via `/cc` command)
- To work with a different repository, users must create a new thread
- This keeps the context clear and implementation simple

### Database References

- Repository path stored in `threads` table
- Always store absolute repository path
- No foreign keys to repositories table
- Path acts as the stable identifier
- Once set, repository path is immutable for the thread's lifetime

## Implementation Plan

### Phase 1: Slash Command Modal ✨

1. **Create `/cc` slash command handler**
   - Register new route for slash commands
   - Return modal with repository dropdown and text input

2. **Modal components**
   - Repository dropdown (select_menu)
   - Initial prompt input (plain_text_input)
   - Submit/Cancel buttons

3. **Modal submission handler**
   - Extract selected repository path
   - Extract initial prompt
   - Create new thread record with repository path
   - Start Claude process with selected path as pwd
   - Repository path is now fixed for this thread

### Phase 2: Configuration Management

1. **Repository configuration structure**
   ```go
   type RepositoryConfig struct {
       Name        string
       Path        string
       Description string
   }
   ```

2. **Load from config**
   - Add to existing Viper configuration
   - Support both config.yaml and environment variables

3. **Validation**
   - Check if repository paths exist
   - Warn on startup if paths are invalid

### Phase 3: Process Management Updates

1. **Update Claude process creation**
   - Accept repository path parameter
   - Set as working directory for process

2. **Update database schema**
   ```sql
   ALTER TABLE threads ADD COLUMN repository_path TEXT;
   ```

3. **Session resume logic**
   - Use stored repository path from threads table when resuming
   - Ensures sessions always resume in the same repository
   - No need for repository selection on resume

### Phase 4: UI Polish

1. **Better repository display**
   - Show repository name in thread responses
   - Include in session status messages

2. **Error handling**
   - Clear error when repository not found
   - Helpful message when no repositories configured

## Success Metrics

- [ ] User can select repository via `/cc` command
- [ ] Claude process starts in correct directory
- [ ] Repository path persisted for session resume
- [ ] Configuration is easy to understand and modify
- [ ] Clear error messages for misconfiguration

## Future Enhancements (Not in scope)

- Multi-worktree support within repositories
- Dynamic repository discovery
- Repository-specific settings

## Technical Notes

### Slack Block Kit Modal Example
```json
{
  "type": "modal",
  "title": {
    "type": "plain_text",
    "text": "Start Claude Session"
  },
  "blocks": [
    {
      "type": "section",
      "block_id": "repository_select",
      "text": {
        "type": "mrkdwn",
        "text": "Select a repository to work with:"
      },
      "accessory": {
        "type": "static_select",
        "action_id": "repository",
        "placeholder": {
          "type": "plain_text",
          "text": "Choose repository"
        },
        "options": [
          {
            "text": {
              "type": "plain_text",
              "text": "cc-slack"
            },
            "value": "/Users/yuya/src/github.com/yuya-takeyama/cc-slack"
          }
        ]
      }
    },
    {
      "type": "input",
      "block_id": "initial_prompt",
      "label": {
        "type": "plain_text",
        "text": "Initial prompt (optional)"
      },
      "element": {
        "type": "plain_text_input",
        "action_id": "prompt",
        "multiline": true
      },
      "optional": true
    }
  ]
}
```

### Migration SQL
```sql
-- migrations/XXXXXX_add_repository_path.up.sql
ALTER TABLE threads ADD COLUMN repository_path TEXT;

-- Set default for existing threads (current working directory)
UPDATE threads SET repository_path = '/Users/yuya/src/github.com/yuya-takeyama/cc-slack' WHERE repository_path IS NULL;

-- migrations/XXXXXX_add_repository_path.down.sql
ALTER TABLE threads DROP COLUMN repository_path;
```