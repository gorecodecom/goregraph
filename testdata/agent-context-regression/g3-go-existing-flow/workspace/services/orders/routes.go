package orders

import "net/http"

type Router struct{}

func (router *Router) DELETE(path string, handler http.HandlerFunc) {}

func (router *Router) GET(path string, handler http.HandlerFunc) {}

var orderService *OrderService

func routes(router *Router) {
	router.DELETE("/orders/{orderId}", deleteOrder)
	router.GET("/orders", listOrders)
}

func deleteOrder(response http.ResponseWriter, request *http.Request) {
	orderID := request.PathValue("orderId")
	_ = orderService.Remove(request.Context(), orderID)
	response.WriteHeader(http.StatusNoContent)
}

func listOrders(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusOK)
}
