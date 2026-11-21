package order

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
}

type CreateOrderCommand struct {
	CustomerID string
	Items      []OrderItem
}

type UseCase struct {
	repository OrderRepository
	now        func() time.Time
}

func NewUseCase(repository OrderRepository) *UseCase {
	return &UseCase{
		repository: repository,
		now:        time.Now,
	}
}

func (u *UseCase) CreateOrder(ctx context.Context, command CreateOrderCommand) (Order, error) {
	if err := ValidateOrder(command.CustomerID, command.Items); err != nil {
		return Order{}, err
	}

	totalCents, itemsCount, err := CalculateTotals(command.Items)
	if err != nil {
		return Order{}, err
	}

	createdAt := u.now().UTC()
	id, err := ulid.New(ulid.Timestamp(createdAt), rand.Reader)
	if err != nil {
		return Order{}, fmt.Errorf("generar identificador de pedido: %w", err)
	}

	created := Order{
		OrderID:          "ORD-" + id.String(),
		CustomerID:       command.CustomerID,
		Items:            append([]OrderItem(nil), command.Items...),
		TotalAmountCents: totalCents,
		ItemsCount:       itemsCount,
	}
	if err := u.repository.Create(ctx, &created); err != nil {
		return Order{}, fmt.Errorf("guardar pedido: %w", err)
	}

	return created, nil
}
