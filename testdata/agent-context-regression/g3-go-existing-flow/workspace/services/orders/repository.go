package orders

import "context"

type OrderRepository interface {
	DeleteByOrderID(ctx context.Context, orderID string) error
}

type SQLOrderRepository struct{}

func (repository *SQLOrderRepository) DeleteByOrderID(
	ctx context.Context,
	orderID string,
) error {
	return nil
}
