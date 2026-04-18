var BASE = "https://api.openweathermap.org/data/2.5";

function weatherGet(path, extra) {
  var sep = path.indexOf("?") === -1 ? "?" : "&";
  var url = BASE + path + sep + "appid=" + secrets.api_key + "&units=" + (config.units || "metric");
  if (extra) url = url + "&" + extra;
  var res = http.get(url);
  if (res.status !== 200) {
    var msg = res.json ? res.json.message : res.body;
    return { error: "API error (" + res.status + "): " + msg };
  }
  return res.json;
}

function degToCompass(deg) {
  var dirs = ["N","NNE","NE","ENE","E","ESE","SE","SSE","S","SSW","SW","WSW","W","WNW","NW","NNW"];
  return dirs[Math.round(deg / 22.5) % 16];
}

function formatSunTime(ts, offsetSec) {
  var d = new Date((ts + offsetSec) * 1000);
  var h = d.getUTCHours();
  var m = d.getUTCMinutes();
  var ampm = h >= 12 ? "PM" : "AM";
  h = h % 12 || 12;
  return h + ":" + (m < 10 ? "0" : "") + m + " " + ampm;
}

function handle(params) {
  var data = weatherGet("/forecast?q=" + encodeURIComponent(params.city));
  if (data.error) return data;

  var days = {};
  var unit = (config.units || "metric") === "metric" ? "°C" : "°F";

  for (var i = 0; i < data.list.length; i++) {
    var item = data.list[i];
    var date = item.dt_txt.split(" ")[0];
    if (!days[date]) {
      days[date] = {
        date: date,
        temps: [],
        conditions: [],
        humidity: [],
        wind_speeds: [],
        rain_total: 0
      };
    }
    days[date].temps.push(item.main.temp);
    days[date].conditions.push(item.weather[0].main);
    days[date].humidity.push(item.main.humidity);
    days[date].wind_speeds.push(item.wind.speed);
    if (item.rain && item.rain["3h"]) days[date].rain_total += item.rain["3h"];
  }

  var forecast = [];
  var dateKeys = Object.keys(days);
  for (var j = 0; j < dateKeys.length; j++) {
    var d = days[dateKeys[j]];
    var minT = Math.min.apply(null, d.temps);
    var maxT = Math.max.apply(null, d.temps);
    var avgHum = Math.round(d.humidity.reduce(function(a, b) { return a + b; }, 0) / d.humidity.length);
    var maxWind = Math.max.apply(null, d.wind_speeds);

    var condCount = {};
    for (var k = 0; k < d.conditions.length; k++) {
      condCount[d.conditions[k]] = (condCount[d.conditions[k]] || 0) + 1;
    }
    var dominant = Object.keys(condCount).sort(function(a, b) { return condCount[b] - condCount[a]; })[0];

    forecast.push({
      date: d.date,
      condition: dominant,
      temp_min: Math.round(minT * 10) / 10,
      temp_max: Math.round(maxT * 10) / 10,
      unit: unit,
      humidity_avg: avgHum,
      wind_max: Math.round(maxWind * 10) / 10,
      rain_mm: Math.round(d.rain_total * 10) / 10
    });
  }

  return {
    city: data.city.name,
    country: data.city.country,
    forecast: forecast
  };
}