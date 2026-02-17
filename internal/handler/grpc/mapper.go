// Package grpc provides gRPC handlers for the order service.
package grpc

import (
	orderv1 "github.com/nsridhar76/go-ordersvc/api/proto/order/v1"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func orderToProto(o *domain.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = &orderv1.OrderItem{
			Id:        item.ID.String(),
			ProductId: item.ProductID,
			Name:      item.Name,
			Quantity:  int32(item.Quantity), // #nosec G115 -- quantity is bounded by validation
			Price:     item.Price,
			Subtotal:  item.Subtotal,
		}
	}
	return &orderv1.Order{
		Id:         o.ID.String(),
		CustomerId: o.CustomerID,
		Items:      items,
		Status:     string(o.Status),
		Total:      o.Total,
		Version:    int32(o.Version), // #nosec G115 -- version is a small incrementing counter
		CreatedAt:  timestamppb.New(o.CreatedAt),
		UpdatedAt:  timestamppb.New(o.UpdatedAt),
	}
}
