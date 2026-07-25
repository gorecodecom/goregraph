package orders

import (
	"context"
	"testing"
)

type recordingOrderRepository struct {
	deletedOrderID string
}

func (repository *recordingOrderRepository) DeleteByOrderID(
	ctx context.Context,
	orderID string,
) error {
	repository.deletedOrderID = orderID
	return nil
}

func TestRemoveDeletesByOrderID(t *testing.T) {
	repository := &recordingOrderRepository{}
	service := &OrderService{repository: repository}

	if err := service.Remove(context.Background(), "order-7"); err != nil {
		t.Fatal(err)
	}
	if repository.deletedOrderID != "order-7" {
		t.Fatalf("deleted order ID = %q, want order-7", repository.deletedOrderID)
	}
}
