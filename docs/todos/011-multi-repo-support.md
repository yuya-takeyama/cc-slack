---
title: Multi-repository support with simple UI and single directory mode
status: done
---

# Multi-repository Support

## Overview

Enable cc-slack to work with multiple working directories through two modes:
1. **Single Directory Mode** - Zero-config mode with command-line argument
2. **Multi-Directory Mode** - Select from configured directories via Slack modal

This allows users to easily try cc-slack with a single directory, then upgrade to multi-directory configuration if needed.

## Design Decisions

### Two Modes of Operation

**1. Single Directory Mode (Zero-config)**
- Start cc-slack with command-line argument: `./cc-slack -w /path/to/project`
- No configuration file needed
- Modal shows only initial prompt input (no directory selection)
- Perfect for trying cc-slack or single-project teams

**2. Multi-Directory Mode (Configured)**
- Configure multiple directories in `config.yaml`
- Modal shows directory selection dropdown
- Full feature set as originally designed

### Working Directory Selection Approach

**For Multi-Directory Mode: Modal UI with explicit selection**
- Use Slack's Block Kit to create a modal when user types `/cc` command
- User explicitly selects working directory from dropdown
- Initial prompt can be entered in the same modal

**Why not LLM Router (from PR #37)?**
- Too complex for initial implementation
- Hard to debug when misrouting happens
- Requires significant prompt engineering
- Can lead to unpredictable behavior

### Working Directory Configuration

**For Multi-Directory Mode: Store in configuration file (config.yaml only)**
```yaml
working_dirs:
  - name: "cc-slack"
    path: "/Users/yuya/src/github.com/yuya-takeyama/cc-slack"
    description: "Claude Code Slack integration"
  - name: "my-project"
    path: "/Users/yuya/src/github.com/yuya-takeyama/my-project"
    description: "Another project"
```

**For Single Directory Mode: Command-line argument**
```bash
./cc-slack --working-dirs /Users/yuya/src/github.com/yuya-takeyama/cc-slack
```

**Why not database?**
- Directory paths can change easily (user moves directories)
- Configuration is more transparent and version-controllable
- Easier to manage in deployment scenarios
- No need for complex migrations when paths change

### Working Directory-Thread Relationship

**1 Thread = 1 Working Directory**
- Each Slack thread is bound to a single working directory
- Working directory is selected when the thread is created (via `/cc` command)
- To work with a different directory, users must create a new thread
- This keeps the context clear and implementation simple

### Database References

- Working directory stored in `threads.working_directory` column
- Always store absolute path
- No foreign keys to repositories table
- Path acts as the stable identifier
- Once set, working directory is immutable for the thread's lifetime

## Implementation Plan

### Phase 0: Single Directory Mode 🚀

1. **Add command-line flag**
   - Use `flag` package or `cobra`/`viper` integration
   - `--working-dirs` flag to specify directories (supports multiple)
   - Store in global config or context

2. **Mode detection**
   - If `--working-dirs` flag is provided with single directory → Single Directory Mode
   - Otherwise → Multi-Directory Mode (use config.yaml)

3. **Update modal logic**
   - Single Directory Mode: Only show prompt input
   - Multi-Directory Mode: Show directory selection + prompt input

4. **Benefits**
   - Zero configuration needed to try cc-slack
   - Easy migration path: try single → configure multiple

### Phase 1: Slash Command Modal ✨

1. **Create `/cc` slash command handler**
   - Add new route `/slack/commands` for slash commands
   - Use existing `/slack/interactive` for modal interactions
   - Return modal with working directory dropdown and text input

2. **Modal components**
   - Working directory dropdown (select_menu)
   - Initial prompt input (rich_text_input - allows formatting, code blocks, lists)
   - Submit/Cancel buttons

3. **Modal submission handler**
   - Extract selected directory path from working directory config
   - Extract initial prompt
   - Create new thread record with working directory
   - Start Claude process with selected path as pwd
   - Working directory is now fixed for this thread

### Phase 2: Configuration Management

1. **Working directory configuration structure**
   ```go
   type WorkingDirectoryConfig struct {
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
   - Ensures sessions always resume in the same working directory
   - No need for directory selection on resume

### Phase 4: UI Polish

1. **Better working directory display**
   - Show directory name in thread responses
   - Include in session status messages

2. **Error handling**
   - Modal submission errors: Show validation errors in modal
   - Working directory invalid: Send ephemeral message (consider prompt loss)
   - Modal cancelled: Send ephemeral \"Cancelled\" message
   - No repositories configured: Fail at startup with helpful message

## Success Metrics

- [ ] User can select working directory via `/cc` command
- [ ] Claude process starts in correct directory
- [ ] Working directory persisted for session resume
- [ ] Configuration is easy to understand and modify
- [ ] Clear error messages for misconfiguration

## Future Enhancements (Not in scope)

- Multi-worktree support within repositories
- Dynamic directory discovery
- Directory-specific settings

## Integration with Existing Features

### Message Event Handling
- Current implementation uses `MessageEvent` (not `app_mention`)
- `/cc` command creates initial thread with working directory selection
- Subsequent messages in thread use existing message event flow
- No changes needed to existing message handling logic

### Thread Creation Flow
1. User types `/cc` → Opens modal
2. User selects working directory and enters prompt → Submit
3. Create thread record with `working_directory`
4. Post initial message to new thread
5. Start Claude process in selected directory
6. Continue with normal message event handling

## Technical Notes

### Critical Implementation Points (from ChatGPT research)

1. **3-Second Rule for Slash Commands**
   - Must respond within 3 seconds or Slack will timeout
   - Solution: Return 200 OK immediately, open modal asynchronously
   ```go
   go openRepoModal(slackClient, cmd.TriggerID, repoConfigs)
   w.WriteHeader(http.StatusOK)
   ```

2. **Block ID vs Action ID**
   - Error responses use `block_id` (NOT `action_id`)
   - Keep consistent naming: `repo_block`/`repo_select`, `prompt_block`/`prompt_input`

3. **View Submission Response**
   - Must return JSON with `response_action`
   - Options: `"errors"` (keep modal open) or `"clear"` (close modal)
   - Heavy processing should be done asynchronously after responding

4. **Endpoint Structure**
   - `/slack/commands` - New endpoint for slash commands
   - `/slack/interactive` - Existing endpoint for modal interactions
   - Share signing secret verification logic

### Slash Command Handler Flow
```go
func handleSlashCC(w http.ResponseWriter, r *http.Request) {
    // 1. Verify signature
    // 2. Parse slash command
    cmd, _ := slack.SlashCommandParse(r)
    
    // 3. Open modal asynchronously
    go openRepoModal(api, cmd.TriggerID, repos)
    
    // 4. Return 200 immediately
    w.WriteHeader(http.StatusOK)
}
```

### Slack Block Kit Modal Example (Updated)
```json
{
  "type": "modal",
  "callback_id": "repo_modal",
  "title": {
    "type": "plain_text",
    "text": "Start Claude Session"
  },
  "submit": {
    "type": "plain_text",
    "text": "Start"
  },
  "close": {
    "type": "plain_text",
    "text": "Cancel"
  },
  "blocks": [
    {
      "type": "input",
      "block_id": "repo_block",
      "label": {
        "type": "plain_text",
        "text": "Select working directory"
      },
      "element": {
        "type": "static_select",
        "action_id": "repo_select",
        "placeholder": {
          "type": "plain_text",
          "text": "Choose directory"
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
      "block_id": "prompt_block",
      "label": {
        "type": "plain_text",
        "text": "Initial prompt"
      },
      "element": {
        "type": "rich_text_input",
        "action_id": "prompt_input",
        "placeholder": {
          "type": "plain_text",
          "text": "What would you like to work on? You can use **bold**, `code`, lists, etc."
        }
      }
    }
  ]
}
```

### View Submission Handler Example
```go
func handleViewSubmission(callback slack.InteractionCallback) error {
    values := callback.View.State.Values
    
    // Extract values
    repoPath := values["repo_block"]["repo_select"].SelectedOption.Value
    
    // Rich text input handling - need to extract from raw JSON
    // Note: slack-go may not have direct support yet
    promptBlock := values["prompt_block"]["prompt_input"]
    var prompt string
    
    // Check if RichTextValue exists (may need type assertion)
    if richTextData, ok := promptBlock.Value.(map[string]interface{}); ok {
        // Parse the rich_text JSON structure
        prompt = convertRichTextToMarkdown(richTextData)
    } else {
        // Fallback or error handling
        prompt = ""
    }
    
    // Validation
    if repoPath == "" {
        return respondWithErrors(map[string]string{
            "repo_block": "Please select a working directory",
        })
    }
    
    // Success - close modal and create thread
    respondWithClear()
    go createThreadAndStartSession(repoPath, prompt, callback.User.ID)
    
    return nil
}

// convertRichTextToMarkdown converts Slack rich_text to Markdown
// Preserves formatting: **bold**, *italic*, `code`, lists, links
func convertRichTextToMarkdown(richTextData map[string]interface{}) string {
    // Parse rich_text JSON structure and convert:
    // - text with style.bold → **text**
    // - text with style.italic → *text*
    // - text with style.code → `text`
    // - link elements → [text](url)
    // - list elements → - item or 1. item
    // This provides best UX: rich input → markdown for Claude
}
```

### Implementation Notes for Rich Text Input

1. **slack-go/slack library support**
   - May not have direct RichTextValue field yet
   - Need to investigate actual JSON structure in view.State.Values
   - May require custom unmarshaling or type assertions

2. **Rich Text JSON Structure** (from ChatGPT research)
   ```json
   {
     "type": "rich_text",
     "elements": [
       {
         "type": "rich_text_section",
         "elements": [
           {"type": "text", "text": "Hello "},
           {"type": "text", "text": "world", "style": {"bold": true}}
         ]
       }
     ]
   }
   ```

3. **Benefits of rich_text_input**
   - Better UX for complex prompts
   - Support for code blocks, lists, emphasis
   - Familiar Slack editing experience

4. **Fallback strategy**
   - If implementation proves complex, can start with plain_text_input
   - Upgrade to rich_text_input in later iteration

### Database Note
The `threads` table already has a `working_directory` column, so no migration is needed! We just need to ensure it's populated when creating threads via the modal.

## Implementation Progress

### ✅ Completed (2025-08-02)

1. **Fixed config format** - Changed from nested structure to flat array:
   - Removed `WorkingDirectoriesConfig` with `Default` and `Configured` fields
   - Changed to direct `[]WorkingDirectoryConfig` array
   - Updated validation logic for both modes

2. **Implemented single directory mode**:
   - Added `--working-dirs` command-line flag (supports multiple directories)
   - Stores paths in `Config.WorkingDirFlags` field
   - Mode detection based on flag presence

3. **Updated modal logic**:
   - Single mode: Shows only prompt input (no directory selection)
   - Multi mode: Shows directory dropdown + prompt input
   - Added `handleSingleDirModalSubmission` for single mode

4. **Fixed config tests**:
   - Created `testdata/config.yaml` for tests
   - Updated all test functions to use test config
   - All tests passing

5. **Tested implementation**:
   - `./scripts/check-all` passes all checks
   - Ready for real-world testing

### ✅ Phase 0-3 Complete! (2025-08-02)

**Phase 0: Single Directory Mode** ✅
- Command-line flag `--working-dirs` implemented (supports multiple)
- Mode detection working correctly
- Modal shows only prompt input in single mode

**Phase 1: Slash Command Modal** ✅
- `/cc` slash command handler implemented at `/slack/commands`
- Modal opens with 3-second timeout handling
- Multi-mode: Directory dropdown + rich text prompt input
- Single-mode: Only rich text prompt input

**Phase 2: Configuration Management** ✅
- Working directory configuration in `config.yaml`
- Flat array structure (not nested)
- Validation at startup
- Clear error messages for misconfiguration

**Phase 3: Process Management Updates** ✅
- Claude process starts with selected working directory
- Working directory stored in threads table
- Session resume uses stored directory

**Phase 4: UI Polish** ✅
- Rich text input implemented with markdown conversion
- `richtext.ConvertToString()` handles all formatting
- Working directory shown in thread responses
- Proper error handling for all edge cases

### 🎉 All Tasks Complete!

All implementation and documentation tasks have been successfully completed on 2025-08-02.

### 📝 Implementation Details

**Rich Text Support**
- Using `rich_text_input` in modal (not plain text)
- Full markdown formatting support (bold, italic, code, lists)
- Conversion handled by `internal/richtext` package
- Better UX for complex prompts ✅

**Error Handling**
- Modal validation errors shown in-modal
- Invalid directory: Ephemeral message sent
- Modal cancellation: Proper cleanup
- No repos configured: Fails at startup with help

### 🎉 Success Metrics Achieved

- ✅ User can select working directory via `/cc` command
- ✅ Claude process starts in correct directory
- ✅ Working directory persisted for session resume
- ✅ Configuration is easy to understand and modify
- ✅ Clear error messages for misconfiguration