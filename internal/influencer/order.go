package influencer

import "github.com/swayops/sway/internal/common"

type OrderedDeals []*common.Deal

func (od OrderedDeals) Len() int {
	return len(od)
}

func (od OrderedDeals) Swap(i, j int) {
	od[i], od[j] = od[j], od[i]
}

func (od OrderedDeals) Less(i, j int) bool {
	return od[i].Spendable > od[j].Spendable
}
