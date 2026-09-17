# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- **Build:** `go build -o bot cmd/bot/main.go`
- **Run:** `go run cmd/bot/main.go`
- **Tests:** Currently no dedicated tests.

## Architecture

The project is a Telegram bot that interacts with a SUI API.

- `cmd/bot/main.go`: Application entry point. It initializes the Telegram bot, loads environment variables, sets up the Sui client, and handles incoming Telegram messages/commands.
- `internal/sui/client.go`: Contains the `Client` struct and methods to interact with the Sui API.

## Environment Variables

The following environment variables must be set (or provided via a `.env` file):

- `TELEGRAM_TOKEN`: The API token for your Telegram bot.
- `SUI_URL`: The URL of the SUI API.
- `SUI_TOKEN`: The authentication token for the SUI API.
- `ADMIN_IDS`: A comma-separated string of Telegram user IDs that are authorized to use the bot.
