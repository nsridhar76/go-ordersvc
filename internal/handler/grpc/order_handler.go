package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	orderv1 "github.com/nsridhar76/go-ordersvc/api/proto/order/v1"
	"github.com/nsridhar76/go-ordersvc/internal/config"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"github.com/nsridhar76/go-ordersvc/internal/messaging"
	"github.com/nsridhar76/go-ordersvc/internal/service"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type orderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	svc      service.OrderService
	kafkaCfg config.KafkaConfig
}

// RegisterOrderServer registers the gRPC order service on the given server.
func RegisterOrderServer(srv *grpc.Server, svc service.OrderService, kafkaCfg config.KafkaConfig) {
	orderv1.RegisterOrderServiceServer(srv, &orderHandler{
		svc:      svc,
		kafkaCfg: kafkaCfg,
	})
}

func (h *orderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, err := h.svc.GetOrderByID(ctx, req.GetOrderId())
	if err != nil {
		return nil, domainToGRPCError(err)
	}
	return &orderv1.GetOrderResponse{Order: orderToProto(order)}, nil
}

func (h *orderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	listReq := service.ListOrdersRequest{
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	}
	if req.GetStatus() != "" {
		s := domain.OrderStatus(req.GetStatus())
		listReq.Status = &s
	}
	if req.GetCustomerId() != "" {
		cid := req.GetCustomerId()
		listReq.CustomerID = &cid
	}

	result, err := h.svc.ListOrders(ctx, listReq)
	if err != nil {
		return nil, domainToGRPCError(err)
	}

	orders := make([]*orderv1.Order, len(result.Data))
	for i, o := range result.Data {
		orders[i] = orderToProto(o)
	}

	return &orderv1.ListOrdersResponse{
		Orders:     orders,
		Page:       int32(result.Page),     // #nosec G115 -- page is bounded by query params
		PageSize:   int32(result.PageSize), // #nosec G115 -- page size is bounded by query params
		TotalCount: result.TotalCount,
		TotalPages: int32(result.TotalPages), // #nosec G115 -- total pages derived from bounded values
	}, nil
}

func (h *orderHandler) WatchOrders(req *orderv1.WatchOrdersRequest, stream grpc.ServerStreamingServer[orderv1.OrderEvent]) error {
	if len(h.kafkaCfg.Brokers) == 0 || h.kafkaCfg.Brokers[0] == "" {
		return status.Error(codes.Unavailable, "Kafka not configured")
	}

	// Per-client consumer with unique group ID for fan-out
	groupID := fmt.Sprintf("%s-watch-%s", h.kafkaCfg.GroupID, uuid.New().String()[:8])
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: h.kafkaCfg.Brokers,
		Topic:   h.kafkaCfg.Topic,
		GroupID: groupID,
	})
	defer func() {
		if err := reader.Close(); err != nil {
			slog.Warn("failed to close Kafka reader", slog.String("error", err.Error()))
		}
	}()

	// Build status filter set
	statusFilter := make(map[string]struct{}, len(req.GetStatuses()))
	for _, s := range req.GetStatuses() {
		statusFilter[s] = struct{}{}
	}

	ctx := stream.Context()
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // client disconnected
			}
			return status.Errorf(codes.Internal, "failed to read Kafka message: %v", err)
		}

		var evt messaging.OrderEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.Warn("failed to unmarshal event", slog.String("error", err.Error()))
			continue
		}

		// Apply status filter if specified
		if len(statusFilter) > 0 {
			if _, ok := statusFilter[evt.Status]; !ok {
				continue
			}
		}

		protoEvt := &orderv1.OrderEvent{
			EventType:  evt.EventType,
			OrderId:    evt.OrderID,
			CustomerId: evt.CustomerID,
			Status:     evt.Status,
			OldStatus:  evt.OldStatus,
			NewStatus:  evt.NewStatus,
			Total:      evt.Total,
			Version:    int32(evt.Version), // #nosec G115 -- version is a small incrementing counter
			OccurredAt: timestamppb.New(evt.OccurredAt),
		}

		if err := stream.Send(protoEvt); err != nil {
			return err
		}
	}
}

func domainToGRPCError(err error) error {
	switch err {
	case domain.ErrOrderNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrInvalidCustomerID, domain.ErrNoItems, domain.ErrInvalidQuantity,
		domain.ErrInvalidPrice, domain.ErrInvalidProductID, domain.ErrInvalidProductName,
		domain.ErrInvalidTransition:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrConcurrentModification:
		return status.Error(codes.Aborted, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
