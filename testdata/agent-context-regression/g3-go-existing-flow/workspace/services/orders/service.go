package orders

import "context"

type OrderService struct {
	repository OrderRepository
}

func (service *OrderService) Remove(ctx context.Context, orderID string) error {
	return service.repository.DeleteByOrderID(ctx, orderID)
}
