---
name: daily_weather_briefing
description: Generate a morning weather briefing for your monitored cities with outfit and activity suggestions.
arguments:
  - name: cities
    description: Comma-separated list of cities to include in the briefing
    required: true
  - name: activity
    description: Planned activity type: commute, outdoor, travel, or general
---

{{cities}}

{{activity}}
