package order

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, order *Order) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transaccion: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("revertir transaccion: %w", rollbackErr))
		}
	}()

	if err = tx.QueryRow(ctx, `
		INSERT INTO orders (order_id, customer_id, total_amount, items_count)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`,
		order.OrderID,
		order.CustomerID,
		numericFromCents(order.TotalAmountCents),
		order.ItemsCount,
	).Scan(&order.ProcessedAt); err != nil {
		return fmt.Errorf("insertar cabecera del pedido: %w", err)
	}

	batch := &pgx.Batch{}
	for _, item := range order.Items {
		batch.Queue(`
			INSERT INTO order_items (order_id, product_id, quantity, price)
			VALUES ($1, $2, $3, $4)`,
			order.OrderID,
			item.ProductID,
			item.Quantity,
			numericFromCents(item.PriceCents),
		)
	}

	results := tx.SendBatch(ctx, batch)
	for range order.Items {
		if _, err = results.Exec(); err != nil {
			closeErr := results.Close()
			if closeErr != nil {
				err = errors.Join(err, fmt.Errorf("cerrar lote de detalles: %w", closeErr))
			}
			return fmt.Errorf("insertar detalle del pedido: %w", err)
		}
	}
	if err = results.Close(); err != nil {
		return fmt.Errorf("cerrar lote de detalles: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar transaccion: %w", err)
	}
	committed = true
	order.ProcessedAt = order.ProcessedAt.UTC()
	return nil
}

func numericFromCents(cents int64) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(cents),
		Exp:   -2,
		Valid: true,
	}
}

var _ OrderRepository = (*PostgresRepository)(nil)
