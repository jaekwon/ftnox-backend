import httplib
import urllib
import json
import copy
import random
import os
import time

CONFIG = json.loads("\n".join(open(os.environ['HOME']+"/.ftnox.com/config.json").readlines()))

FTNOX_API_HOST = "ftnox.com"
CREDENTIALS = {"api_key": "34XC6S5kYE1652DF6doFH6uB"}
COIN = "BTC"
BASIS_COIN = "USD"
MARKET = COIN+"/"+BASIS_COIN
DEFAULT_PRICE = 400.0
CURRENT_TIME = 0
BITCOIN_PRICE = 400.0

def API(path, params={}):
    global CURRENT_TIME
    params = params.copy()
    params.update(CREDENTIALS)
    paramsStr = urllib.urlencode(params)
    conn = httplib.HTTPSConnection(FTNOX_API_HOST)
    conn.request("POST", path, paramsStr, {"Content-type": "application/x-www-form-urlencoded", "Accept": "text/json"})
    res = conn.getresponse()
    data = res.read()
    #print path, params, data
    data = json.loads(data)
    if data["status"] != "OK":
        raise Exception(data)
    else:
        CURRENT_TIME = res.getheader("X-Server-Time")
        return data["data"]

def getLastPrice():
    markets = API("/exchange/markets")
    for market in markets:
        if market["coin"]+"/"+market["basisCoin"] == MARKET:
            return float(market["last"]) or DEFAULT_PRICE
    raise Exception("No such market "+MARKET)

def getMinTradeAmount(coin):
    for c in CONFIG["Coins"]:
        if c["Name"] == coin:
            return c["MinTrade"]
    raise Exception("Unknown coin "+coin)

def roundPrice(price):
    if random.random() < 0.25:
        sigFigs = 1
    elif random.random() < 0.33:
        sigFigs = 5
    elif random.random() < 0.33:
        sigFigs = 2
    elif random.random() < 0.3:
        sigFigs = 3
    else:
        sigFigs = 4
    return float(('%.'+str(sigFigs)+'g') % price)

def roundAmount(amount):
    if random.random() < 0.31:
        sigFigs = 1
    elif random.random() < 0.25:
        sigFigs = 2
    elif random.random() < 0.35:
        sigFigs = 3
    elif random.random() < 0.4:
        sigFigs = 4
    else:
        sigFigs = 5
    return int(float(('%.'+str(sigFigs)+'g') % amount))

while True:
    # sleep for a random amount of time
    #time.sleep(random.expovariate(1.0) * 55)
    time.sleep(1)
    
    try:
        balance = API("/account/balance")
        orders  = API("/exchange/pending_orders", {"market": MARKET})
        lastPrice = getLastPrice()
        #print lastPrice, type(lastPrice)

        if len(orders) > 0 and random.random() < 0.1: # cancel an order
            print "cancel order"
            order = orders[random.randint(0, len(orders)-1)]
            data = API("/exchange/cancel_order", {"id": order["id"]})
        elif random.random() < 0.5: # place a new bid
            print "place bid"
            if balance[BASIS_COIN] > getMinTradeAmount(BASIS_COIN):
                price = random.lognormvariate(1.0, 0.1) / 2.7182818284590451 * lastPrice
                price = roundPrice(price)
                dollarAmount = random.weibullvariate(1.9, 0.4) * 100 # hack
                print dollarAmount
                basisAmount = dollarAmount / BITCOIN_PRICE * 100000000
                if basisAmount > balance[BASIS_COIN]: basisAmount = balance[BASIS_COIN]
                if basisAmount < getMinTradeAmount(BASIS_COIN): basisAmount = getMinTradeAmount(BASIS_COIN)
                amount = int(float(basisAmount / price - 10)) # HACK
                #amount = basisAmount / price
                #amount = roundAmount(amount)
                if amount < getMinTradeAmount(COIN): amount = getMinTradeAmount(COIN)
                data = API("/exchange/add_order", {"market": MARKET, "amount": amount, "price": price, "order_type": "B"})
                #print "Bid response:", data
            else:
                print "not enough basis coins to place bid"
        else: # place a new ask
            print "place ask"
            if balance[COIN] > getMinTradeAmount(COIN):
                price = random.lognormvariate(1.0, 0.1) / 2.7182818284590451 * lastPrice
                print dollarAmount
                price = roundPrice(price)
                dollarAmount = random.weibullvariate(1.9, 0.4) * 100 # hack
                basisAmount = dollarAmount / BITCOIN_PRICE * 100000000
                amount = basisAmount / price
                amount = roundAmount(amount)
                if amount > balance[COIN]: amount = balance[COIN]
                if amount < getMinTradeAmount(COIN): amount = getMinTradeAmount(COIN)
                data = API("/exchange/add_order", {"market": MARKET, "amount": amount, "price": price, "order_type": "A"})
                #print "Ask response:", data
            else:
                print "not enough coins to place ask"
    except Exception, err:
        import traceback
        traceback.print_exc()
        print "ERROR:", err
