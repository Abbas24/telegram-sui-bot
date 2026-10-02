# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

The project includes definitions for [[inbound-management]].

## Deployment
- **Bot Path:** `/root/telegram-sui-bot/bot`

## Environment Variables

The following environment variables must be set (or provided via a `.env` file):

- `TELEGRAM_TOKEN`: The API token for your Telegram bot.
- `SUI_URL`: The URL of the SUI API.
- `SUI_TOKEN`: The authentication token for the SUI API.
- `ADMIN_IDS`: A comma-separated string of Telegram user IDs that are authorized to use the bot.

## Remote Debugging

- **Access:** Remote server is at `95.211.45.38` via port `2026`.
- **Logs:** Use `journalctl -u telegram-bot` to inspect application logs on the server.
- **SSH Key:** Use the key located at `/home/abbas/.ssh/claude_ssh_key` for authentication (as root).

## Documentation
- Refer to the S-UI Wiki located in `s-ui-wiki/` for all API and configuration questions.
