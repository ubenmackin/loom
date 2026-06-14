-- Migration 011: Rename acp_base_url setting to acp_command
-- The ACP protocol changed from WebSocket URL to stdio subprocess command.
-- Rename the key and reset the value since ws:// URLs are not valid commands.
UPDATE settings
SET key = 'acp_command',
    value = 'opencode acp'
WHERE key = 'acp_base_url';
