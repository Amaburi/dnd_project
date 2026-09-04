package models

import "fmt"

// Coin denominations in copper pieces, the smallest unit.
const (
	CopperPerSilver   = 10
	CopperPerElectrum = 50
	CopperPerGold     = 100
	CopperPerPlatinum = 1000
)

// Currency is a character's coin purse.
//
// Coins are tracked per denomination rather than as a single total because
// that is how they are found, spent and carried -- and because 50 coins of any
// kind weigh a pound, so the split affects encumbrance.
type Currency struct {
	Copper   int `json:"copper" bson:"copper"`
	Silver   int `json:"silver" bson:"silver"`
	Electrum int `json:"electrum" bson:"electrum"`
	Gold     int `json:"gold" bson:"gold"`
	Platinum int `json:"platinum" bson:"platinum"`
}

// TotalInCopper converts the whole purse to copper pieces.
func (c Currency) TotalInCopper() int {
	return c.Copper +
		c.Silver*CopperPerSilver +
		c.Electrum*CopperPerElectrum +
		c.Gold*CopperPerGold +
		c.Platinum*CopperPerPlatinum
}

// TotalInGold converts the whole purse to gold pieces, which is how prices are
// usually quoted.
func (c Currency) TotalInGold() float64 {
	return float64(c.TotalInCopper()) / float64(CopperPerGold)
}

// CoinCount is the number of physical coins carried.
func (c Currency) CoinCount() int {
	return c.Copper + c.Silver + c.Electrum + c.Gold + c.Platinum
}

// CoinsPerPound is how many coins make up a pound of weight.
const CoinsPerPound = 50

// Weight returns the weight of the purse in pounds.
func (c Currency) Weight() float64 {
	return float64(c.CoinCount()) / float64(CoinsPerPound)
}

// Add combines two purses.
func (c Currency) Add(other Currency) Currency {
	return Currency{
		Copper:   c.Copper + other.Copper,
		Silver:   c.Silver + other.Silver,
		Electrum: c.Electrum + other.Electrum,
		Gold:     c.Gold + other.Gold,
		Platinum: c.Platinum + other.Platinum,
	}
}

// Spend removes an amount from the purse, making change from larger coins when
// a denomination runs short.
//
// Coins are fungible in 5e -- a shopkeeper taking 3 gp will take 30 sp -- so
// spending checks the total rather than refusing for want of the exact coin.
func (c *Currency) Spend(cost Currency) error {
	need := cost.TotalInCopper()
	have := c.TotalInCopper()
	if need > have {
		return Invalid("insufficient funds: need %d cp, have %d cp", need, have)
	}

	remaining := have - need
	// Settle back into the largest denominations that fit.
	c.Platinum = remaining / CopperPerPlatinum
	remaining %= CopperPerPlatinum
	c.Gold = remaining / CopperPerGold
	remaining %= CopperPerGold
	// Electrum is deliberately skipped when making change: it is rare enough
	// in play that handing it back surprises people.
	c.Electrum = 0
	c.Silver = remaining / CopperPerSilver
	c.Copper = remaining % CopperPerSilver
	return nil
}

// String renders the purse in the usual shorthand, largest coin first.
func (c Currency) String() string {
	parts := ""
	appendPart := func(n int, suffix string) {
		if n == 0 {
			return
		}
		if parts != "" {
			parts += " "
		}
		parts += fmt.Sprintf("%d%s", n, suffix)
	}

	appendPart(c.Platinum, "pp")
	appendPart(c.Gold, "gp")
	appendPart(c.Electrum, "ep")
	appendPart(c.Silver, "sp")
	appendPart(c.Copper, "cp")

	if parts == "" {
		return "0cp"
	}
	return parts
}
