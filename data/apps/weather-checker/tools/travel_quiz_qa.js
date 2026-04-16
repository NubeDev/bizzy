function handle(params) {
  if (params._answers !== undefined) return chatMode(params._answers);
  if (!params._submit) return formDefinition();
  return formSubmit(params);
}

function chatMode(answers) {
  if (!answers.city) {
    return {
      type: 'question', field: 'city',
      label: 'What city are you visiting?',
      input: 'text', required: true, min_length: 2,
      placeholder: 'e.g. Sydney, London, Tokyo'
    };
  }
  if (!answers.scenario) {
    return {
      type: 'question', field: 'scenario',
      label: 'What activity are you planning in ' + answers.city + '?',
      input: 'select', required: true,
      options: [
        {value: 'beach', label: 'Beach day'},
        {value: 'hiking', label: 'Mountain hike'},
        {value: 'picnic', label: 'Park picnic'},
        {value: 'cycling', label: 'Bike ride'},
        {value: 'running', label: 'Outdoor run'}
      ]
    };
  }
  var quiz = buildQuiz(answers.city, answers.scenario);
  if (quiz.error) return quiz;
  if (!answers.answer) {
    return {
      type: 'question', field: 'answer',
      label: quiz.question,
      input: 'select', required: true,
      context: { weather: quiz.weather, city: quiz.city, country: quiz.country },
      options: [
        {value: 'A', label: quiz.options.A},
        {value: 'B', label: quiz.options.B},
        {value: 'C', label: quiz.options.C},
        {value: 'D', label: quiz.options.D}
      ]
    };
  }
  var correct = quiz.correct_answer === answers.answer;
  return {
    type: 'result',
    title: correct ? 'Correct!' : 'Not quite!',
    city: quiz.city,
    country: quiz.country,
    weather: quiz.weather,
    scenario: answers.scenario,
    your_answer: answers.answer + ': ' + quiz.options[answers.answer],
    correct_answer: quiz.correct_answer + ': ' + quiz.options[quiz.correct_answer],
    is_correct: correct,
    explanation: quiz.explanation
  };
}

function formDefinition() {
  return {
    type: 'qa',
    title: 'Travel Weather Quiz',
    description: 'Test your travel smarts! We fetch live weather and quiz you on the safest decision.',
    fields: [
      {name: 'city', label: 'City', type: 'text', required: true, min_length: 2, placeholder: 'e.g. Sydney'},
      {name: 'scenario', label: 'Activity', type: 'select', required: true, options: [
        {value: 'beach', label: 'Beach day'}, {value: 'hiking', label: 'Mountain hike'},
        {value: 'picnic', label: 'Park picnic'}, {value: 'cycling', label: 'Bike ride'},
        {value: 'running', label: 'Outdoor run'}
      ]},
      {name: 'answer', label: 'Your answer (shown after weather loads)', type: 'text', required: false}
    ]
  };
}

function formSubmit(params) {
  if (!params.city || params.city.length < 2) return {type: 'validation_error', errors: [{field: 'city', message: 'City is required'}]};
  if (!params.scenario) return {type: 'validation_error', errors: [{field: 'scenario', message: 'Pick an activity'}]};
  var quiz = buildQuiz(params.city, params.scenario);
  if (quiz.error) return quiz;
  if (!params.answer) {
    return { type: 'quiz', city: quiz.city, country: quiz.country, weather: quiz.weather, scenario: params.scenario, question: quiz.question, options: quiz.options, instructions: 'Call again with answer=A/B/C/D and _submit=true to check your answer' };
  }
  var correct = quiz.correct_answer === params.answer.toUpperCase();
  return { type: 'result', city: quiz.city, country: quiz.country, weather: quiz.weather, scenario: params.scenario, your_answer: params.answer.toUpperCase(), correct_answer: quiz.correct_answer, is_correct: correct, explanation: quiz.explanation };
}

function buildQuiz(city, scenario) {
  var geo = http.get('https://geocoding-api.open-meteo.com/v1/search?name=' + encodeURIComponent(city) + '&count=1');
  if (!geo.json || !geo.json.results || geo.json.results.length === 0) return {error: 'City not found: ' + city};
  var loc = geo.json.results[0];
  var url = 'https://api.open-meteo.com/v1/forecast?latitude=' + loc.latitude + '&longitude=' + loc.longitude + '&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code,apparent_temperature&timezone=auto';
  var wx = http.get(url);
  if (!wx.json || !wx.json.current) return {error: 'Failed to fetch weather'};
  var cur = wx.json.current;
  var codes = {0:'Clear sky',1:'Mainly clear',2:'Partly cloudy',3:'Overcast',45:'Foggy',48:'Rime fog',51:'Light drizzle',53:'Moderate drizzle',55:'Dense drizzle',61:'Slight rain',63:'Moderate rain',65:'Heavy rain',71:'Slight snow',73:'Moderate snow',75:'Heavy snow',80:'Slight showers',81:'Moderate showers',82:'Violent showers',95:'Thunderstorm',96:'Thunderstorm with hail',99:'Thunderstorm with heavy hail'};
  var temp = cur.temperature_2m; var feels = cur.apparent_temperature; var wind = cur.wind_speed_10m; var humid = cur.relative_humidity_2m; var code = cur.weather_code;
  var cond = codes[code] || 'Unknown';
  var isRain = code >= 51 && code <= 82; var isSnow = code >= 71 && code <= 75; var isStorm = code >= 95; var isClear = code <= 2;
  var weather = {temperature_c: temp, feels_like_c: feels, conditions: cond, wind_kmh: wind, humidity_pct: humid};
  var ws = temp + '\u00b0C (feels ' + feels + '\u00b0C), ' + cond + ', wind ' + wind + ' km/h';
  var q, opts, ans, why;
  if (scenario === 'beach') {
    q = 'It is ' + ws + ' in ' + loc.name + '. You want a beach day. What should you do?';
    if (isStorm) { opts={A:'Head to the beach',B:'Wait at the hotel',C:'Go to an indoor pool instead',D:'Drive to another beach'}; ans='C'; why='Thunderstorms at the beach are dangerous (lightning, rough seas). Go indoors.'; }
    else if (isRain) { opts={A:'Go with an umbrella',B:'Wear a rain jacket and swim',C:'Switch to an indoor activity',D:'Wait 30 minutes'}; ans='C'; why='Rain makes the beach unpleasant and unsafe. Switch to indoor plans.'; }
    else if (temp < 15) { opts={A:'Swim anyway',B:'Bring a wetsuit',C:'Skip the beach, coastal walk instead',D:'Stay in bed'}; ans='C'; why='At ' + temp + '\u00b0C the water is too cold for most people. A coastal walk is better.'; }
    else if (temp < 24) { opts={A:'Perfect beach weather',B:'Go but bring a jacket for later',C:'Too cold for the beach',D:'Only go if windless'}; ans='B'; why='At ' + temp + '\u00b0C it could cool off later. Bring a jacket.'; }
    else { opts={A:'Head out with sunscreen',B:'Too hot, stay inside',C:'Only go in the evening',D:'Sunscreen is optional'}; ans='A'; why='At ' + temp + '\u00b0C it is ideal beach weather. Sunscreen is essential!'; }
  } else if (scenario === 'hiking') {
    q = 'It is ' + ws + ' in ' + loc.name + '. You planned a mountain hike. Best decision?';
    if (isStorm) { opts={A:'Hike anyway',B:'Cancel and reschedule',C:'Do a short trail instead',D:'Wait at the trailhead'}; ans='B'; why='Lightning on exposed trails is extremely dangerous. Cancel.'; }
    else if (isRain) { opts={A:'Hike in the rain',B:'Stick to low sheltered trails',C:'Cancel entirely',D:'Run the trail quickly'}; ans='B'; why='Rain makes trails slippery. Stick to lower, sheltered paths.'; }
    else if (temp > 35) { opts={A:'Start at dawn, finish before noon',B:'Hike at midday',C:'Temperature doesn\'t matter',D:'Go in the afternoon'}; ans='A'; why='At ' + temp + '\u00b0C heat exhaustion is a real risk. Start early.'; }
    else { opts={A:'Great conditions, enjoy the hike',B:'Too risky in any weather',C:'Only go with a guide',D:'Wait for better conditions'}; ans='A'; why=temp + '\u00b0C with ' + cond.toLowerCase() + ' is solid hiking weather.'; }
  } else if (scenario === 'picnic') {
    q = 'It is ' + ws + ' in ' + loc.name + '. You want a park picnic. What should you do?';
    if (isStorm || isRain) { opts={A:'Picnic under a tree',B:'Move indoors to a cafe',C:'Use a tarp and power through',D:'Eat in the car'}; ans='B'; why='Rain ruins food and trees attract lightning. Move indoors.'; }
    else if (temp < 8) { opts={A:'Bundle up in the cold',B:'Bring hot soup and blankets',C:'Too cold, reschedule',D:'Sit on a sunny bench'}; ans='B'; why='At ' + temp + '\u00b0C you can picnic with hot drinks and warm blankets.'; }
    else if (temp > 32) { opts={A:'Find shade and bring cold drinks',B:'Sit in direct sun',C:'Cancel, food will spoil',D:'Eat quickly'}; ans='A'; why='At ' + temp + '\u00b0C shade is essential. Keep food and drinks cool.'; }
    else { opts={A:'Perfect picnic weather, enjoy!',B:'Too windy for a picnic',C:'Only picnic after 5pm',D:'Picnics are only for summer'}; ans='A'; why=temp + '\u00b0C with ' + cond.toLowerCase() + ' is lovely picnic weather!'; }
  } else if (scenario === 'cycling') {
    q = 'It is ' + ws + ' in ' + loc.name + '. You want a long bike ride. Smartest move?';
    if (isStorm) { opts={A:'Ride fast to beat the storm',B:'Use an indoor trainer',C:'Wear bright colors and go',D:'Wait 10 minutes'}; ans='B'; why='Cycling in a thunderstorm is extremely dangerous. Ride indoors.'; }
    else if (isRain) { opts={A:'Ride slowly, avoid painted road markings',B:'Rain doesn\'t affect cycling',C:'Cancel, bikes don\'t work in rain',D:'Only ride trails'}; ans='A'; why='You can ride in rain but road markings and metal get slippery. Slow down.'; }
    else if (wind > 30) { opts={A:'Ride into wind first, tailwind home',B:'Wind doesn\'t matter',C:'Only ride downhill',D:'Wait for calm'}; ans='A'; why='Ride into the wind while fresh, enjoy the tailwind on the way back.'; }
    else { opts={A:'Great conditions, get riding',B:'Too dangerous without a helmet',C:'Only before sunrise',D:'Only safe on trails'}; ans='A'; why=temp + '\u00b0C with ' + cond.toLowerCase() + ' is excellent cycling weather.'; }
  } else {
    q = 'It is ' + ws + ' in ' + loc.name + '. You want to go for a run. What should you consider?';
    if (isStorm) { opts={A:'Treadmill instead',B:'Storm running builds toughness',C:'Rubber shoes prevent lightning',D:'Run between raindrops'}; ans='A'; why='Lightning doesn\'t care how fast you are. Use a treadmill.'; }
    else if (temp > 32) { opts={A:'Run early morning or late evening',B:'Midday for vitamin D',C:'Coffee before hot runs',D:'Dark clothing absorbs sweat'}; ans='A'; why='At ' + temp + '\u00b0C run during cooler hours to avoid heat stroke.'; }
    else if (temp < 0) { opts={A:'Layers and cover exposed skin',B:'Run faster to stay warm',C:'Cold air is good for lungs',D:'Shorts if fast enough'}; ans='A'; why='At ' + temp + '\u00b0C exposed skin risks frostbite. Layer up.'; }
    else { opts={A:'Lace up and enjoy!',B:'Too cold for running',C:'Only run indoors',D:'Wait for warmer weather'}; ans='A'; why=temp + '\u00b0C with ' + cond.toLowerCase() + ' is great running weather!'; }
  }
  return {city: loc.name, country: loc.country, weather: weather, question: q, options: opts, correct_answer: ans, explanation: why};
}