function handle(params) {
  var geo = http.get("https://geocoding-api.open-meteo.com/v1/search?name=" + encodeURIComponent(params.city) + "&count=1");
  if (!geo.json || !geo.json.results || geo.json.results.length === 0) {
    return { error: "City not found: " + params.city };
  }
  var loc = geo.json.results[0];
  var lat = loc.latitude;
  var lon = loc.longitude;
  var url = "https://api.open-meteo.com/v1/forecast?latitude=" + lat + "&longitude=" + lon + "&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code&timezone=auto";
  var wx = http.get(url);
  if (!wx.json || !wx.json.current) {
    return { error: "Failed to fetch weather data" };
  }
  var c = wx.json.current;
  var codes = {0:"Clear sky",1:"Mainly clear",2:"Partly cloudy",3:"Overcast",45:"Foggy",48:"Rime fog",51:"Light drizzle",53:"Moderate drizzle",55:"Dense drizzle",61:"Slight rain",63:"Moderate rain",65:"Heavy rain",71:"Slight snow",73:"Moderate snow",75:"Heavy snow",80:"Slight showers",81:"Moderate showers",82:"Violent showers",95:"Thunderstorm",96:"Thunderstorm with hail",99:"Thunderstorm with heavy hail"};
  return {
    city: loc.name,
    country: loc.country,
    latitude: lat,
    longitude: lon,
    temperature_c: c.temperature_2m,
    humidity_pct: c.relative_humidity_2m,
    wind_speed_kmh: c.wind_speed_10m,
    conditions: codes[c.weather_code] || "Unknown (" + c.weather_code + ")"
  };
}