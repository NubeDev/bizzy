function handle(params) {
  if (!params.month || params.month < 1 || params.month > 12) {
    return { error: "month is required (1-12)" };
  }
  if (!params.year || params.year < 2024) {
    return { error: "year is required (e.g. 2026)" };
  }

  var destinations = [];
  if (params.destination) {
    var dest = params.destination.toUpperCase();
    if (dest !== "KUL" && dest !== "BKK") {
      return { error: "destination must be KUL or BKK" };
    }
    destinations.push(dest);
  } else {
    destinations = ["KUL", "BKK"];
  }

  var weekends = findWeekends(params.month, params.year);
  if (weekends.length === 0) {
    return { error: "No Fridays found in " + MONTH_NAMES[params.month - 1] + " " + params.year };
  }

  // Get exchange rates for price estimates
  var rates = getExchangeRates();
  var vndRate = rates ? rates.VND : null;
  var audRate = rates ? rates.AUD : null;

  var results = [];
  for (var d = 0; d < destinations.length; d++) {
    var dest = destinations[d];
    var route = ROUTES[dest];
    var weekendResults = [];

    for (var w = 0; w < weekends.length; w++) {
      var wk = weekends[w];
      var priceEstimate = null;
      if (route.price_range_usd) {
        var low = route.price_range_usd.low;
        var high = route.price_range_usd.high;
        priceEstimate = {
          round_trip_usd: "$" + low + " - $" + high,
          round_trip_vnd: vndRate ? (Math.round(low * vndRate)).toLocaleString() + " - " + (Math.round(high * vndRate)).toLocaleString() + " VND" : "unavailable",
          round_trip_aud: audRate ? "A$" + Math.round(low * audRate) + " - A$" + Math.round(high * audRate) : "unavailable"
        };
      }

      weekendResults.push({
        weekend: wk.friday_display + " to " + wk.monday_display,
        depart: wk.friday_str,
        return: wk.monday_str,
        google_flights: googleFlightsUrl("DAD", dest, wk.friday_str, wk.monday_str),
        skyscanner: skyscannerUrl("DAD", dest, wk.friday_str, wk.monday_str),
        price_estimate: priceEstimate
      });
    }

    results.push({
      destination: dest + " — " + route.city + ", " + route.country,
      airlines: route.airlines,
      typical_flights: route.typical_flight_times,
      weekends: weekendResults
    });
  }

  return {
    origin: "DAD — Da Nang, Vietnam",
    month: MONTH_NAMES[params.month - 1] + " " + params.year,
    total_weekends: weekends.length,
    exchange_rates: rates ? { USD_to_VND: vndRate, USD_to_AUD: audRate } : "unavailable — check links for live prices",
    routes: results
  };
}
