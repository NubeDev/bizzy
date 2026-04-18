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
  var data = weatherGet("/weather?q=" + encodeURIComponent(params.city));
  if (data.error) return data;

  return {
    city: data.name,
    country: data.sys.country,
    coordinates: { lat: data.coord.lat, lon: data.coord.lon },
    condition: data.weather[0].main,
    description: data.weather[0].description,
    icon: data.weather[0].icon,
    temperature: {
      current: data.main.temp,
      feels_like: data.main.feels_like,
      min: data.main.temp_min,
      max: data.main.temp_max,
      unit: (config.units || "metric") === "metric" ? "°C" : "°F"
    },
    humidity: data.main.humidity,
    pressure: data.main.pressure,
    visibility: data.visibility ? Math.round(data.visibility / 1000 * 10) / 10 : null,
    wind: {
      speed: data.wind.speed,
      direction: data.wind.deg ? degToCompass(data.wind.deg) : "N/A",
      gust: data.wind.gust || null,
      unit: (config.units || "metric") === "metric" ? "m/s" : "mph"
    },
    clouds: data.clouds.all,
    sunrise: formatSunTime(data.sys.sunrise, data.timezone),
    sunset: formatSunTime(data.sys.sunset, data.timezone),
    timezone_offset: data.timezone
  };
}