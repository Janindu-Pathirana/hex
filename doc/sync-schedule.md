# Keeping the data fresh

History only grows while `hex sync` runs: the CSE returns about one year of daily bars (240 points) per fetch, and the market statistics endpoint returns only the last two days. Every day that is not synced is data that cannot be fetched later. Run a daily sync after the market closes (14:30 Asia/Colombo, Monday to Friday) and a weekly one for reports.

The commands below assume `hex` is on the path (`brew install`, or the full path to the binary) and the machine's clock is on Colombo time. Adjust the hour if it is not.

## What to run

| When | Command | What it does |
|---|---|---|
| Daily, 15:00 on weekdays | `hex sync market index quotes history --all-symbols` | Market statistics (builds up day by day), a year of ASPI and S&P SL20 levels, today's quotes, daily bars for every company |
| Weekly, Sunday | `hex sync actions reports pdfs fundamentals --all-symbols` | Announcements, the report catalogue, new PDFs and the figures read from them |
| After either | `hex data audit` | Shows what is missing or stale, with the command that fixes it |

Notes:
- A full daily run is about 300 companies at 5 requests a second: a few minutes. Work still inside its freshness window is skipped, and an interrupted run resumes where it stopped.
- `hex sync status` lists failed jobs. `--dry-run` shows what would run without fetching.
- The weekly run downloads PDFs and is much heavier. Use `--symbols jkh,comb,...` for a watch list instead of `--all-symbols` if you only follow a few companies.
- By default 6 interim reports per company are read, enough for trailing twelve months and year-on-year comparisons. Use `--quarterly-reports` and `--annual-reports` to change that.

## macOS (launchd)

Save as `~/Library/LaunchAgents/lk.hex.sync-daily.plist`, then `launchctl load ~/Library/LaunchAgents/lk.hex.sync-daily.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>lk.hex.sync-daily</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/hex</string>
    <string>sync</string><string>market</string><string>index</string><string>quotes</string><string>history</string>
    <string>--all-symbols</string>
  </array>
  <key>StartCalendarInterval</key>
  <array>
    <dict><key>Weekday</key><integer>1</integer><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
    <dict><key>Weekday</key><integer>2</integer><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
    <dict><key>Weekday</key><integer>3</integer><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
    <dict><key>Weekday</key><integer>4</integer><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
    <dict><key>Weekday</key><integer>5</integer><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
  </array>
  <key>StandardOutPath</key><string>/tmp/hex-sync.log</string>
  <key>StandardErrorPath</key><string>/tmp/hex-sync.log</string>
</dict>
</plist>
```

launchd runs a missed job when the Mac wakes, so a laptop that was asleep at 15:00 still syncs. For the weekly job copy the file, change the label and arguments, and use a single `Weekday` of `0` (Sunday).

## Linux or macOS (cron)

```cron
# daily, 15:00 Monday to Friday (machine on Colombo time)
0 15 * * 1-5 hex sync market index quotes history --all-symbols >> $HOME/hex-sync.log 2>&1
# weekly, Sunday 10:00
0 10 * * 0 hex sync actions reports pdfs fundamentals --all-symbols >> $HOME/hex-sync.log 2>&1
```

Cron does not catch up after the machine was off. If it is not always on, prefer launchd, or run `hex sync market index quotes history --all-symbols` by hand when you start using it.

## Where the data lives

`os.UserConfigDir()/hex/hex.db` (macOS: `~/Library/Application Support/hex/hex.db`); set `HEX_DATA_DIR` to move it. Back it up before `hex clean db`, which deletes it, manual values included.
