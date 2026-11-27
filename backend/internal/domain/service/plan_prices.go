package service

import (
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/plans"
)

func PlanPrices() dto.PlanPriceList {
	list := dto.PlanPriceList{Currency: plans.Currency, TrialDays: plans.TrialDays}
	for _, p := range plans.All {
		list.Plans = append(list.Plans, dto.PlanPrice{ID: string(p), Monthly: plans.Price(p, plans.Monthly), Annual: plans.Price(p, plans.Annual)})
	}
	return list
}
