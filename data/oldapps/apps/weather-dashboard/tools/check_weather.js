function handle(params) {
  var geo = http.get('https://geocoding-api.open-meteo.com/v1/search?name=' + encodeURIComponent(params.city) + '&count=1');
  if (geo.status !== 200) return { error: 'Geocoding failed', status: geo.status };
  var results = geo.body.results;
  if (!results || results.length === 0) return { error: 'City not found: ' + params.city };
  var loc = results[0];
  var url = 'https://api.open-meteo.com/v1/forecast?latitude=' + loc.latitude + '&longitude=' + loc.longitude + '&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code';
  var wx = http.get(url);
  if (wx.status !== 200) return { error: 'Weather API failed', status: wx.status };
  var c = wx.body.current;
  var codes = {0:'Clear sky',1:'Mainly clear',2:'Partly cloudy',3:'Overcast',45:'Foggy',48:'Rime fog',51:'Light drizzle',53:'Moderate drizzle',55:'Dense drizzle',61:'Slight rain',63:'Moderate rain',65:'Heavy rain',71:'Slight snow',73:'Moderate snow',75:'Heavy snow',80:'Slight showers',81:'Moderate showers',82:'Violent showers',95:'Thunderstorm',96:'Thunderstorm with hail',99:'Severe thunderstorm'};
  return {
    city: loc.name,
    country: loc.country,
    temperature_c: c.temperature_2m,
    humidity_pct: c.relative_humidity_2m,
    wind_speed_kmh: c.wind_speed_10m,
    description: codes[c.weather_code] || ('Code ' + c.weather_code)
  };
}