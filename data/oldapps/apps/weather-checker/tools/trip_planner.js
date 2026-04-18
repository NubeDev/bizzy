function handle(params) {
  var geo = http.get("https://geocoding-api.open-meteo.com/v1/search?name=" + encodeURIComponent(params.city) + "&count=1");
  if (!geo.json || !geo.json.results || geo.json.results.length === 0) {
    return { error: "City not found: " + params.city };
  }
  var loc = geo.json.results[0];
  var url = "https://api.open-meteo.com/v1/forecast?latitude=" + loc.latitude + "&longitude=" + loc.longitude + "&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code,apparent_temperature&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,weather_code&timezone=auto&forecast_days=3";
  var wx = http.get(url);
  if (!wx.json || !wx.json.current) return { error: "Failed to fetch weather" };
  var c = wx.json.current;
  var codes = {0:"Clear sky",1:"Mainly clear",2:"Partly cloudy",3:"Overcast",45:"Foggy",48:"Rime fog",51:"Light drizzle",53:"Moderate drizzle",55:"Dense drizzle",61:"Slight rain",63:"Moderate rain",65:"Heavy rain",71:"Slight snow",73:"Moderate snow",75:"Heavy snow",80:"Slight showers",81:"Moderate showers",82:"Violent showers",95:"Thunderstorm",96:"Thunderstorm with hail",99:"Thunderstorm with heavy hail"};
  var temp = c.temperature_2m;
  var feels = c.apparent_temperature;
  var humid = c.relative_humidity_2m;
  var wind = c.wind_speed_10m;
  var code = c.weather_code;
  var cond = codes[code] || "Unknown";
  var isRain = code >= 51 && code <= 82;
  var isSnow = code >= 71 && code <= 75;
  var isStorm = code >= 95;
  var isClear = code <= 2;
  var score = 10;
  if (isStorm) score -= 6;
  else if (isRain) score -= 3;
  else if (isSnow) score -= 2;
  if (temp < 0) score -= 2;
  else if (temp < 10) score -= 1;
  else if (temp > 38) score -= 2;
  if (wind > 40) score -= 2;
  else if (wind > 25) score -= 1;
  if (score < 1) score = 1;
  var rating = score >= 8 ? "Excellent" : score >= 6 ? "Good" : score >= 4 ? "Fair" : "Poor";
  var packing = [];
  if (temp < 10) packing.push("warm jacket", "layers");
  else if (temp < 20) packing.push("light jacket");
  else packing.push("light clothing");
  if (isRain || isStorm) packing.push("umbrella", "rain jacket");
  if (isClear && temp > 20) packing.push("sunscreen", "sunglasses", "hat");
  if (isSnow) packing.push("waterproof boots", "gloves");
  if (wind > 25) packing.push("windbreaker");
  var activities = [];
  var pref = (params.activity || "").toLowerCase();
  if (pref === "beach") {
    if (isClear && temp > 24) activities.push({name: "Beach day", verdict: "Perfect conditions!"});
    else if (temp > 20 && !isRain) activities.push({name: "Beach day", verdict: "Decent but not ideal"});
    else activities.push({name: "Beach day", verdict: "Not recommended today"});
  } else if (pref === "hiking") {
    if (!isStorm && !isRain && temp > 5 && temp < 35) activities.push({name: "Hiking", verdict: "Good conditions"});
    else activities.push({name: "Hiking", verdict: "Not recommended — check conditions"});
  } else if (pref === "skiing") {
    if (isSnow || temp < 5) activities.push({name: "Skiing", verdict: "Great conditions!"});
    else activities.push({name: "Skiing", verdict: "Too warm for skiing"});
  } else if (pref === "sightseeing") {
    if (!isStorm) activities.push({name: "Sightseeing", verdict: isRain ? "Bring an umbrella" : "Great day for it!"});
    else activities.push({name: "Sightseeing", verdict: "Stay indoors — storm conditions"});
  }
  if (isClear && temp >= 15 && temp <= 30) activities.push({name: "Outdoor dining", verdict: "Recommended"});
  if (isClear && temp > 24) activities.push({name: "Beach / swimming", verdict: "Great conditions"});
  if (!isRain && !isStorm && temp > 5) activities.push({name: "Walking tour", verdict: "Good conditions"});
  if (isRain || isStorm) activities.push({name: "Museums / indoor", verdict: "Recommended today"});
  if (isSnow) activities.push({name: "Snow activities", verdict: "Conditions are right"});
  var forecast = [];
  if (wx.json.daily) {
    var d = wx.json.daily;
    for (var i = 0; i < d.time.length; i++) {
      forecast.push({date: d.time[i], high: d.temperature_2m_max[i], low: d.temperature_2m_min[i], rain_mm: d.precipitation_sum[i], conditions: codes[d.weather_code[i]] || "Unknown"});
    }
  }
  return {
    city: loc.name, country: loc.country,
    current: {temperature_c: temp, feels_like_c: feels, humidity_pct: humid, wind_kmh: wind, conditions: cond},
    outdoor_score: {score: score, rating: rating, out_of: 10},
    suggested_activities: activities,
    packing_list: packing,
    forecast_3day: forecast
  };
}