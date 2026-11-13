// Package domain contains the immutable business data shared by local and MCP tools.
package domain

import "regexp"

var orderID = regexp.MustCompile(`^TR-[0-9]{4}$`)

// QueryOrder validates the order ID and returns the catalog entry as JSON.
func QueryOrder(id string) string {
	if !orderID.MatchString(id) {
		return `{"error":"INVALID_ORDER_ID"}`
	}
	switch id {
	case "TR-1001":
		return `{"orderId":"TR-1001","item":"barraca","status":"IN_TRANSIT","deliveryEstimate":"até 3 dias úteis"}`
	case "TR-1002":
		return `{"orderId":"TR-1002","item":"mochila","status":"DELIVERED","deliveryEstimate":"entregue"}`
	default:
		return `{"error":"ORDER_NOT_ALLOWED"}`
	}
}
