function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  if (!params.filter) return { error: "filter is required" };

  // Build the time range
  var fromTo = resolveTimeRange(params);

  // Build the request body matching the Rubix /history API format
  var body = {
    from: fromTo.from,
    to: fromTo.to,
    filters: [
      {
        name: "query",
        filter: params.filter,
        portHandle: params.portHandle || "output"
      }
    ]
  };

  if (params.interval) body.interval = params.interval;

  var resp = http.post(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + "/query/history",
    body,
    { headers: { "Authorization": "Bearer " + auth.token, "Content-Type": "application/json" } }
  );

  if (resp.status !== 200) {
    return { error: "History query failed (" + resp.status + "): " + resp.body };
  }

  var data = resp.json.data || resp.json || {};
  var nodes = data.nodes || [];
  var result = {
    totalNodes: data.totalNodes || nodes.length,
    totalSamples: data.totalSamples || 0,
    resolution: data.resolution || null,
    downsampled: data.downsampled || false,
    nodes: []
  };

  for (var i = 0; i < nodes.length; i++) {
    var n = nodes[i];
    var samples = n.samples || [];
    var node = {
      id: n.id,
      name: n.name,
      portHandle: n.portHandle,
      sampleCount: samples.length
    };

    if (samples.length > 0) {
      var values = [];
      for (var j = 0; j < samples.length; j++) {
        if (typeof samples[j].value === "number") values.push(samples[j].value);
      }

      if (values.length > 0) {
        node.stats = computeStats(values);
        node.trend = detectTrend(values);
        node.anomalies = detectAnomalies(samples);
      }

      node.firstSample = samples[0];
      node.lastSample = samples[samples.length - 1];
      node.samples = samples;
    }

    result.nodes.push(node);
  }

  return result;
}

function resolveTimeRange(params) {
  if (params.from && params.to) {
    return { from: params.from, to: params.to };
  }

  var now = new Date();
  var to = now.toISOString();
  var ms = now.getTime();
  var range = params.range || "last24h";

  var offsets = {
    last1h:  3600000,
    last6h:  21600000,
    last24h: 86400000,
    last7d:  604800000,
    last30d: 2592000000
  };

  var offset = offsets[range];
  if (!offset) offset = 86400000;

  var from = new Date(ms - offset).toISOString();
  return { from: from, to: to };
}

function computeStats(values) {
  var sorted = values.slice().sort(function(a, b) { return a - b; });
  var sum = 0;
  for (var i = 0; i < values.length; i++) sum += values[i];
  var avg = sum / values.length;

  var mid = Math.floor(sorted.length / 2);
  var median = sorted.length % 2 === 0
    ? (sorted[mid - 1] + sorted[mid]) / 2
    : sorted[mid];

  var sqDiffSum = 0;
  for (var i = 0; i < values.length; i++) {
    var diff = values[i] - avg;
    sqDiffSum += diff * diff;
  }
  var stddev = Math.sqrt(sqDiffSum / values.length);

  return {
    min: sorted[0],
    max: sorted[sorted.length - 1],
    avg: round(avg),
    median: round(median),
    stddev: round(stddev),
    count: values.length
  };
}

function detectTrend(values) {
  if (values.length < 4) return { direction: "insufficient_data" };

  var q = Math.max(1, Math.floor(values.length / 4));
  var firstSum = 0, lastSum = 0;
  for (var i = 0; i < q; i++) firstSum += values[i];
  for (var i = values.length - q; i < values.length; i++) lastSum += values[i];
  var firstAvg = firstSum / q;
  var lastAvg = lastSum / q;

  var range = Math.max.apply(null, values) - Math.min.apply(null, values);
  if (range === 0) return { direction: "flat", changePercent: 0 };

  var change = lastAvg - firstAvg;
  var changePct = round((change / range) * 100);

  var direction = "stable";
  if (changePct > 15) direction = "rising";
  else if (changePct < -15) direction = "falling";

  return {
    direction: direction,
    changePercent: changePct,
    firstQuarterAvg: round(firstAvg),
    lastQuarterAvg: round(lastAvg)
  };
}

function detectAnomalies(samples) {
  var values = [];
  for (var i = 0; i < samples.length; i++) {
    if (typeof samples[i].value === "number") values.push(samples[i].value);
  }
  if (values.length < 10) return [];

  var sum = 0;
  for (var i = 0; i < values.length; i++) sum += values[i];
  var avg = sum / values.length;
  var sqDiffSum = 0;
  for (var i = 0; i < values.length; i++) {
    var diff = values[i] - avg;
    sqDiffSum += diff * diff;
  }
  var stddev = Math.sqrt(sqDiffSum / values.length);
  if (stddev === 0) return [];

  var anomalies = [];
  for (var i = 0; i < samples.length; i++) {
    if (typeof samples[i].value !== "number") continue;
    var z = Math.abs((samples[i].value - avg) / stddev);
    if (z >= 2.5) {
      anomalies.push({
        timestamp: samples[i].timestamp,
        value: samples[i].value,
        zScore: round(z),
        type: samples[i].value > avg ? "spike" : "drop"
      });
    }
  }

  return anomalies;
}

function round(v) {
  return Math.round(v * 100) / 100;
}

function login() {
  var resp = http.post(
    config.rubix_host + "/api/v1/auth/login",
    { email: "admin@rubix.io", password: "admin@rubix.io" },
    { headers: { "Content-Type": "application/json" } }
  );
  if (resp.status !== 200) {
    return { error: "Login failed: " + resp.body };
  }
  var d = resp.json.data;
  return { token: d.token, orgId: d.orgId, deviceId: d.deviceId };
}
