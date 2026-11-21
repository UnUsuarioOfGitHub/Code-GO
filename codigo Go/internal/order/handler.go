package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxRequestBody = 1 << 20

type CreateOrderRequest struct {
	CustomerID string                   `json:"customerId"`
	Items      []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	ProductID string      `json:"productId"`
	Quantity  int32       `json:"quantity"`
	Price     json.Number `json:"price"`
}

type OrderResponse struct {
	OrderID     string    `json:"orderId"`
	TotalAmount Money     `json:"totalAmount"`
	ItemsCount  int32     `json:"itemsCount"`
	ProcessedAt time.Time `json:"processedAt"`
}

type Money int64

func (m Money) MarshalJSON() ([]byte, error) {
	cents := int64(m)
	if cents < 0 {
		return nil, fmt.Errorf("el importe no puede ser negativo")
	}
	return []byte(strconv.FormatInt(cents/100, 10) + "." + fmt.Sprintf("%02d", cents%100)), nil
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type createOrder interface {
	CreateOrder(context.Context, CreateOrderCommand) (Order, error)
}

type Handler struct {
	createOrder createOrder
	logger      *slog.Logger
	ping        func(context.Context) error
}

type requestIDContextKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func NewHandler(createOrder createOrder, logger *slog.Logger, ping func(context.Context) error) *Handler {
	return &Handler{
		createOrder: createOrder,
		logger:      logger,
		ping:        ping,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/orders", h.create)
	mux.HandleFunc("/api/orders", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Metodo no permitido")
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("/healthz", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/readyz", methodNotAllowed(http.MethodGet))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Ruta no encontrada")
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var request CreateOrderRequest
	if err := decoder.Decode(&request); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, "request_too_large", "El cuerpo supera el limite de 1 MB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "El cuerpo debe contener un unico objeto JSON")
		return
	}

	if utf8.RuneCountInString(request.CustomerID) == 0 || utf8.RuneCountInString(request.CustomerID) > MaxTextLength {
		writeError(w, http.StatusBadRequest, "invalid_order", "customerId es obligatorio y debe tener hasta 50 caracteres")
		return
	}
	if len(request.Items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_order", "items debe contener al menos un elemento")
		return
	}

	items := make([]OrderItem, 0, len(request.Items))
	for index, item := range request.Items {
		if utf8.RuneCountInString(item.ProductID) == 0 || utf8.RuneCountInString(item.ProductID) > MaxTextLength {
			writeError(w, http.StatusBadRequest, "invalid_order", fmt.Sprintf("items[%d].productId es obligatorio y debe tener hasta 50 caracteres", index))
			return
		}
		if item.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_order", fmt.Sprintf("items[%d].quantity debe ser un entero positivo", index))
			return
		}
		priceCents, err := parsePriceCents(item.Price)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_price", fmt.Sprintf("items[%d].price debe ser un importe no negativo con hasta dos decimales", index))
			return
		}
		if priceCents > MaxAmountCents {
			writeError(w, http.StatusBadRequest, "limit_exceeded", fmt.Sprintf("items[%d].price supera el limite de NUMERIC(10,2)", index))
			return
		}
		items = append(items, OrderItem{
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			PriceCents: priceCents,
		})
	}

	created, err := h.createOrder.CreateOrder(r.Context(), CreateOrderCommand{
		CustomerID: request.CustomerID,
		Items:      items,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidOrder) {
			writeError(w, http.StatusBadRequest, "invalid_order", "El pedido no es valido")
			return
		}
		if errors.Is(err, ErrLimitExceeded) {
			writeError(w, http.StatusBadRequest, "limit_exceeded", "El pedido supera los limites admitidos")
			return
		}
		h.logger.Error("fallo al crear pedido",
			"request_id", RequestIDFromContext(r.Context()),
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal_error", "No se pudo procesar el pedido")
		return
	}

	writeJSON(w, http.StatusCreated, OrderResponse{
		OrderID:     created.OrderID,
		TotalAmount: Money(created.TotalAmountCents),
		ItemsCount:  created.ItemsCount,
		ProcessedAt: created.ProcessedAt.UTC(),
	})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.ping(ctx); err != nil {
		h.logger.Warn("base de datos no disponible",
			"request_id", RequestIDFromContext(r.Context()),
			"error", err,
		)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func parsePriceCents(number json.Number) (int64, error) {
	value := string(number)
	if value == "" {
		return 0, fmt.Errorf("precio ausente")
	}

	exponent := int64(0)
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		parsed, err := strconv.ParseInt(value[index+1:], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("exponente invalido: %w", err)
		}
		exponent = parsed
		value = value[:index]
	}

	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")

	fractionDigits := 0
	if index := strings.IndexByte(value, '.'); index >= 0 {
		fractionDigits = len(value) - index - 1
		value = value[:index] + value[index+1:]
	}
	digits := strings.TrimLeft(value, "0")
	if digits == "" {
		return 0, nil
	}

	scale := int64(fractionDigits) - exponent
	shift := int64(2) - scale
	if shift >= 0 {
		if shift > 10 || int64(len(digits))+shift > 10 {
			return 0, fmt.Errorf("importe fuera de rango")
		}
		digits += strings.Repeat("0", int(shift))
	} else {
		remove := -shift
		if remove > int64(len(digits)) {
			return 0, fmt.Errorf("mas de dos decimales no nulos")
		}
		cut := len(digits) - int(remove)
		if strings.Trim(digits[cut:], "0") != "" {
			return 0, fmt.Errorf("mas de dos decimales no nulos")
		}
		digits = digits[:cut]
		if digits == "" {
			digits = "0"
		}
	}

	cents, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || cents > MaxAmountCents {
		return 0, fmt.Errorf("importe fuera de rango")
	}
	if negative && cents > 0 {
		return 0, fmt.Errorf("importe negativo")
	}
	return cents, nil
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("JSON adicional")
		}
		return err
	}
	return nil
}

func methodNotAllowed(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allowed)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Metodo no permitido")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: message, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}
