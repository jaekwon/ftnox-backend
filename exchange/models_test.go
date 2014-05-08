package exchange

import (
    . "ftnox.com/common"
    "testing"
)

func TestComputeTrade(t *testing.T) {
    testTrade := func (order, match *Order, tradeAmount, tradeBasisAmount uint64) {
        ta, tba := order.ComputeTrade(match)
        if ta != tradeAmount       { panic(NewError("Wrong trade amount: expected %v actual %v", tradeAmount, ta)) }
        if tba != tradeBasisAmount { panic(NewError("Wrong trade basis amount: expected %v actual %v", tradeBasisAmount, tba)) }
    }

    testTrade(
        &Order{ // Market Order
            Type:   "A",
            Amount: 100,
        },
        &Order{ // Limit Order
            Type:   "B",
            BasisAmount: 90,
            Price:  1.0,
        }, 90, 90,
    )

                                                                                                 //  Coin,  Basis
    testTrade(&Order{Type:"B", BasisAmount:100,},   &Order{Type:"A", Amount:200,        Price:1.0},   100,    100)
    testTrade(&Order{Type:"B", BasisAmount:100,},   &Order{Type:"A", Amount:50,         Price:1.0},    50,     50)
    testTrade(&Order{Type:"B", BasisAmount:100,},   &Order{Type:"A", Amount:50,         Price:0.5},    50,     25)

    testTrade(&Order{Type:"A", Amount:100,},        &Order{Type:"B", BasisAmount:200,   Price:1.0},   100,    100)
    testTrade(&Order{Type:"A", Amount:100,},        &Order{Type:"B", BasisAmount:50,    Price:1.0},    50,     50)
    testTrade(&Order{Type:"A", Amount:100,},        &Order{Type:"B", BasisAmount:50,    Price:0.5},   100,     50)


    // Mixed limits...
    testTrade(&Order{Amount:100,            BasisAmount:50},                    &Order{Amount:200, BasisAmount:200, Price:1.0},   50,50)
    testTrade(&Order{Amount:100, Filled:60, BasisAmount:50},                    &Order{Amount:200, BasisAmount:200, Price:1.0},   40,40)
    testTrade(&Order{Amount:100, Filled:60, BasisAmount:50, BasisFilled:30},    &Order{Amount:200, BasisAmount:200, Price:1.0},   20,20)

    testTrade(&Order{Amount:100}, &Order{BasisAmount:200, Price:2.0}, 100,200)
    testTrade(&Order{Amount:90},  &Order{BasisAmount:200, Price:2.0}, 90, 180)
    testTrade(&Order{Amount:100}, &Order{BasisAmount:190, Price:2.0}, 95, 190)
    testTrade(&Order{Amount:100}, &Order{Amount:96,BasisAmount:190, Price:2.0}, 95, 190)
    testTrade(&Order{Amount:100}, &Order{Amount:94,BasisAmount:190, Price:2.0}, 94, 188)

    testTrade(&Order{BasisAmount:100},              &Order{Amount:200, Price:2.0},  50,100)
    testTrade(&Order{BasisAmount:100},              &Order{Amount:200, Price:0.5}, 200,100)
    testTrade(&Order{Amount:199,BasisAmount:100},   &Order{Amount:200, Price:0.5}, 199,100)

}
