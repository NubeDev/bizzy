---
name: weather_report
description: Generate a weather report for a city
arguments:
  - name: city
    description: City to report on
    required: true
---

Use the get_weather tool to fetch the current weather for {{city}}, then write a brief weather report including:
- Current conditions and temperature
- Whether it is good weather for outdoor activities
- What to wear based on the conditions
- Any weather warnings if applicable
