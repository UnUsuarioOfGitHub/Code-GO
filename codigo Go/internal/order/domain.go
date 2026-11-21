package order

import (
	"errors"
	"math"
	"time"
	"unicode/utf8"
)

const (
	MaxTextLength  = 50
	MaxInt32       = int64(math.MaxInt32)
	MaxAmountCents = int64(9_999_999_999)
)

var (
	ErrInvalidOrder  = errors.New("pedido invalido")
	ErrLimitExceeded = errors.New("el pedido supera los limites admitidos")
)

type OrderItem struct {
	ProductID  string
	Quantity   int32
	PriceCents int64
}

type Order struct {
	OrderID          string
	CustomerID       string
	Items            []OrderItem
	TotalAmountCents int64
	ItemsCount       int32
	ProcessedAt      time.Time
}

func ValidateOrder(customerID string, items []OrderItem) error {
	if utf8.RuneCountInString(customerID) == 0 || utf8.RuneCountInString(customerID) > MaxTextLength {
		return ErrInvalidOrder
	}
	if len(items) == 0 {
		return ErrInvalidOrder
	}
	for _, item := range items {
		if utf8.RuneCountInString(item.ProductID) == 0 || utf8.RuneCountInString(item.ProductID) > MaxTextLength {
			return ErrInvalidOrder
		}
		if item.Quantity <= 0 || item.PriceCents < 0 {
			return ErrInvalidOrder
		}
		if item.PriceCents > MaxAmountCents {
			return ErrLimitExceeded
		}
	}
	return nil
}

func CalculateTotals(items []OrderItem) (int64, int32, error) {
	var total int64
	var itemsCount int64

	for _, item := range items {
		if item.Quantity <= 0 || item.PriceCents < 0 {
			return 0, 0, ErrInvalidOrder
		}
		quantity := int64(item.Quantity)
		if item.PriceCents > 0 && quantity > MaxAmountCents/item.PriceCents {
			return 0, 0, ErrLimitExceeded
		}
		lineTotal := item.PriceCents * quantity
		if lineTotal > MaxAmountCents-total {
			return 0, 0, ErrLimitExceeded
		}
		total += lineTotal
		itemsCount += quantity
		if itemsCount > MaxInt32 {
			return 0, 0, ErrLimitExceeded
		}
	}

	return total, int32(itemsCount), nil
}
