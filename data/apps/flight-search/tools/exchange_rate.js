function handle(params) {
  var rates = getExchangeRates();
  if (!rates) {
    return { error: "Failed to fetch exchange rates" };
  }

  var result = {
    base: "USD",
    rates: {
      VND: rates.VND,
      AUD: rates.AUD
    }
  };

  if (params.amount_usd) {
    result.converted = {
      usd: "$" + params.amount_usd,
      vnd: Math.round(params.amount_usd * rates.VND).toLocaleString() + " VND",
      aud: "A$" + (params.amount_usd * rates.AUD).toFixed(2)
    };
  }

  return result;
}
