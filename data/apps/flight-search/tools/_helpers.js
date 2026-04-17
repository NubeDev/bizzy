// Shared helpers for flight-search tools

// Known direct flight info for DAD routes (these are stable, well-established routes)
var ROUTES = {
  KUL: {
    city: "Kuala Lumpur",
    country: "Malaysia",
    airlines: [
      { code: "AK", name: "AirAsia", typical_duration: "3h 00m", frequency: "daily" },
      { code: "VJ", name: "VietJet Air", typical_duration: "3h 10m", frequency: "daily" }
    ],
    typical_flight_times: [
      { depart: "06:00", arrive: "09:00", airline: "AirAsia" },
      { depart: "11:30", arrive: "14:30", airline: "VietJet Air" },
      { depart: "17:00", arrive: "20:00", airline: "AirAsia" }
    ],
    price_range_usd: { low: 45, high: 150 }
  },
  BKK: {
    city: "Bangkok",
    country: "Thailand",
    airlines: [
      { code: "VJ", name: "VietJet Air", typical_duration: "1h 50m", frequency: "daily" },
      { code: "VN", name: "Vietnam Airlines", typical_duration: "1h 45m", frequency: "daily" },
      { code: "FD", name: "Thai AirAsia", typical_duration: "1h 55m", frequency: "4x weekly" }
    ],
    typical_flight_times: [
      { depart: "07:30", arrive: "09:20", airline: "VietJet Air" },
      { depart: "12:00", arrive: "13:45", airline: "Vietnam Airlines" },
      { depart: "18:00", arrive: "19:50", airline: "VietJet Air" }
    ],
    price_range_usd: { low: 35, high: 120 }
  }
};

var MONTH_NAMES = ["January","February","March","April","May","June","July","August","September","October","November","December"];

function pad(n) {
  return n < 10 ? "0" + n : "" + n;
}

function formatDate(d) {
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
}

function formatDisplay(d) {
  var days = ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"];
  return days[d.getDay()] + " " + d.getDate() + " " + MONTH_NAMES[d.getMonth()];
}

// Find all Fridays in a given month/year, return Friday-Monday pairs
function findWeekends(month, year) {
  var weekends = [];
  var d = new Date(year, month - 1, 1);
  while (d.getMonth() === month - 1) {
    if (d.getDay() === 5) { // Friday
      var fri = new Date(d.getTime());
      var mon = new Date(d.getTime());
      mon.setDate(mon.getDate() + 3);
      weekends.push({
        friday: fri,
        monday: mon,
        friday_str: formatDate(fri),
        monday_str: formatDate(mon),
        friday_display: formatDisplay(fri),
        monday_display: formatDisplay(mon)
      });
    }
    d.setDate(d.getDate() + 1);
  }
  return weekends;
}

// Build a Google Flights search URL
// Format: https://www.google.com/travel/flights?q=Flights+from+DAD+to+KUL+on+2026-06-05+return+2026-06-08
function googleFlightsUrl(origin, dest, departDate, returnDate) {
  return "https://www.google.com/travel/flights?q=Flights+from+"
    + origin + "+to+" + dest
    + "+on+" + departDate
    + "+return+" + returnDate
    + "&curr=VND";
}

// Build a Skyscanner search URL as backup
function skyscannerUrl(origin, dest, departDate, returnDate) {
  var depParts = departDate.split("-");
  var retParts = returnDate.split("-");
  return "https://www.skyscanner.com/transport/flights/"
    + origin.toLowerCase() + "/" + dest.toLowerCase() + "/"
    + depParts[0] + depParts[1] + depParts[2] + "/"
    + retParts[0] + retParts[1] + retParts[2]
    + "/?adults=1";
}

function getExchangeRates() {
  var resp = http.get("https://api.frankfurter.dev/v1/latest?from=USD&to=VND,AUD");
  if (resp.status === 200 && resp.json && resp.json.rates) {
    return resp.json.rates;
  }
  return null;
}
