//////////////////////////////////// API

var http = require("http");
var querystring = require('querystring');

var FTNOX_API_PORT = 8888;
var FTNOX_API_HOST = 'localhost';

function API(credentials, path, params, cb) {
    var reqParams = {
        user_id:    credentials.userId,
        api_key:    credentials.APIKey,
    };
    for (var key in params) {
        reqParams[key] = params[key];
    }
    var reqData = querystring.stringify(reqParams);

    var options = {
        hostname:   FTNOX_API_HOST,
        port:       FTNOX_API_PORT,
        path:       path,
        method:     'POST',
        headers: {
            'Content-Type':     'application/x-www-form-urlencoded',
            'Content-Length':   Buffer.byteLength(reqData),
        },
    };

    var req = http.request(options, function(res) {
        res.setEncoding('utf8');
        res.on('data', function (chunk) {
            try {
                var response = JSON.parse(chunk);
                cb(response.status, response.data);
            } catch(e) {
                cb("ERROR", e);
            }
        });
    });

    req.on("error", function(e) {
        cb("ERROR", e);
    });

    req.write(reqData);
    req.end();
}

/*
Sample usage:
var credentials = {userId: 23, APIKey: "rxNokIjTmSKiQpt7YGNryXsW"};

API(credentials, "/account/balance", {}, function(code, data) {
    console.log("code:", code);
    console.log("data:", data);
});
*/

////////////////////////////////////

var credentials = {userId: 23, APIKey: "rxNokIjTmSKiQpt7YGNryXsW"};
var coin = "DOGE";
var basisCoin = "BTC";
var market = coin+"/"+basisCoin;
var targetPrice = 0.00000122;

API(credentials, "/account/balance", {}, function(code, data) {
    if (code != "OK") {
        console.log("ERROR:", code);
        process.exit(0);
    }
    var balance = data;
    console.log("BALANCE:", balance);

    // Trade.
    // TODO: retry logic when failure?
    API(credentials, "/exchange/add_order",
        {
            "market":       market,
            "order_type":   "A",
            "amount":       100 * 100000000,
            "price":        1.0,
        }
        , function(code, data) {
            console.log(code);
            console.log(data);
        }
    );
});
