---
name: weekend_flights
description: Find all Friday-to-Monday weekend flights from Da Nang to KUL and BKK for a given month
arguments:
  - name: month
    description: "Month and year (e.g. 'june 2026', 'december 2025')"
    required: true
  - name: destination
    description: "KUL, BKK, or both (default: both)"
    required: false
---

The user wants to find weekend flights for: {{month}}
Destination preference: {{destination}}

Parse the month and year from the input, then call `flight-search.search_weekend_flights` with the month number and year.

Present the results as a clean table for each destination:

1. Show the **weekend dates** (Friday to Monday)
2. Show **typical flight times** and airlines for the route
3. Show **estimated price range** in both VND and AUD
4. Include the **Google Flights link** for each weekend so they can see live prices

Format the Google Flights links as clickable markdown links.

If the user asked for a specific destination (KUL or BKK), only show that one.
If they said "both" or didn't specify, show both.
