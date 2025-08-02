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

**Store in configuration file (config.yaml only)**
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

- Working directory stored in `threads.working_directory` column
- Always store absolute path
- No foreign keys to repositories table
- Path acts as the stable identifier
- Once set, working directory is immutable for the thread's lifetime

## Implementation Plan

### Phase 1: Slash Command Modal ✨

1. **Create `/cc` slash command handler**
   - Add new route `/slack/commands` for slash commands
   - Use existing `/slack/interactive` for modal interactions
   - Return modal with repository dropdown and text input

2. **Modal components**
   - Repository dropdown (select_menu)
   - Initial prompt input (plain_text_input)
   - Submit/Cancel buttons

3. **Modal submission handler**
   - Extract selected directory path from repository config
   - Extract initial prompt
   - Create new thread record with working directory
   - Start Claude process with selected path as pwd
   - Working directory is now fixed for this thread

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
   - Read configuration at startup only
   - Support config.yaml only (no environment variables)

3. **Validation**
   - Check if configured directory paths exist at startup
   - **Fail to start if no repositories configured**
   - Clear error message explaining how to configure repositories
   - Warn if some paths are invalid but allow startup

### Phase 3: Process Management Updates

1. **Update Claude process creation**
   - Accept working directory parameter
   - Set as working directory for process

2. **Note on database schema**
   - No schema changes needed! Already have `working_directory` column
   - Just need to populate it from modal selection

3. **Session resume logic**
   - Use stored working directory from threads table when resuming
   - Ensures sessions always resume in the same repository
   - No need for repository selection on resume

### Phase 4: UI Polish

1. **Better repository display**
   - Show repository name in thread responses
   - Include in session status messages

2. **Error handling**
   - Modal submission errors: Show validation errors in modal
   - Working directory invalid: Send ephemeral message (consider prompt loss)
   - Modal cancelled: Send ephemeral \"Cancelled\" message
   - No repositories configured: Fail at startup with helpful message

## Success Metrics

- [ ] User can select repository via `/cc` command
- [ ] Claude process starts in correct directory
- [ ] Working directory persisted for session resume
- [ ] Configuration is easy to understand and modify
- [ ] Clear error messages for misconfiguration

## Future Enhancements (Not in scope)

- Multi-worktree support within repositories
- Dynamic repository discovery
- Repository-specific settings

## Integration with Existing Features

### Message Event Handling
- Current implementation uses `MessageEvent` (not `app_mention`)
- `/cc` command creates initial thread with repository selection
- Subsequent messages in thread use existing message event flow
- No changes needed to existing message handling logic

### Thread Creation Flow
1. User types `/cc` → Opens modal
2. User selects repository and enters prompt → Submit
3. Create thread record with `working_directory`
4. Post initial message to new thread
5. Start Claude process in selected repository
6. Continue with normal message event handling

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

### Database Note
The `threads` table already has a `working_directory` column, so no migration is needed! We just need to ensure it's populated when creating threads via the modal.