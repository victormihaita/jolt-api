package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	gqlmiddleware "github.com/user/remind-me/backend/internal/graphql/middleware"
	"github.com/user/remind-me/backend/internal/graphql/model"
	"github.com/user/remind-me/backend/internal/graphql/resolver"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
	"github.com/user/remind-me/backend/pkg/jwt"
)

// GraphQLRequest represents an incoming GraphQL request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Handler holds the GraphQL handler dependencies
type Handler struct {
	Resolver   *resolver.Resolver
	JWTManager *jwt.Manager
}

// NewHandler creates a new GraphQL handler
func NewHandler(r *resolver.Resolver, jwtManager *jwt.Manager) *Handler {
	return &Handler{
		Resolver:   r,
		JWTManager: jwtManager,
	}
}

// GraphQL handles GraphQL HTTP requests
func (h *Handler) GraphQL(c *gin.Context) {
	var req GraphQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Invalid request body"}},
		})
		return
	}

	// Create context with auth info
	ctx := h.contextWithAuth(c)

	// Execute the query
	result := h.execute(ctx, req)

	c.JSON(http.StatusOK, result)
}

// contextWithAuth extracts auth info from the request and adds it to the context
func (h *Handler) contextWithAuth(c *gin.Context) context.Context {
	ctx := c.Request.Context()

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		fmt.Printf("contextWithAuth: no Authorization header\n")
		return ctx
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		fmt.Printf("contextWithAuth: Authorization header doesn't start with 'Bearer '\n")
		return ctx
	}

	fmt.Printf("contextWithAuth: validating token (length=%d)\n", len(token))
	claims, err := h.JWTManager.ValidateToken(token)
	if err != nil {
		fmt.Printf("contextWithAuth: token validation failed: %v\n", err)
		return ctx
	}

	fmt.Printf("contextWithAuth: token valid, userID=%s\n", claims.UserID)
	ctx = gqlmiddleware.WithUserID(ctx, claims.UserID)
	if claims.DeviceID != nil {
		ctx = gqlmiddleware.WithDeviceID(ctx, *claims.DeviceID)
	}

	return ctx
}

// execute processes the GraphQL request
func (h *Handler) execute(ctx context.Context, req GraphQLRequest) GraphQLResponse {
	query := strings.TrimSpace(req.Query)

	if strings.HasPrefix(query, "query") || (!strings.HasPrefix(query, "mutation") && !strings.HasPrefix(query, "subscription")) {
		return h.executeQuery(ctx, req)
	} else if strings.HasPrefix(query, "mutation") {
		return h.executeMutation(ctx, req)
	}

	return GraphQLResponse{
		Errors: []GraphQLError{{Message: "Subscriptions are only supported over WebSocket"}},
	}
}

// executeQuery handles query operations
func (h *Handler) executeQuery(ctx context.Context, req GraphQLRequest) GraphQLResponse {
	data := make(map[string]interface{})
	var errs []GraphQLError

	opName := strings.ToLower(req.OperationName)
	query := strings.ToLower(req.Query)

	fmt.Printf("executeQuery: opName=%q, query=%q\n", opName, query)

	if strings.Contains(query, "me") && (opName == "" || opName == "me" || opName == "getme") {
		result, err := h.Resolver.Me(ctx)
		if err != nil {
			fmt.Printf("Me query error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			data["me"] = nil
		} else {
			fmt.Printf("Me query result: %+v\n", result)
			data["me"] = result
		}
	}

	if strings.Contains(query, "devices") && (opName == "" || opName == "devices" || opName == "getdevices") {
		result, err := h.Resolver.Devices(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["devices"] = nil
		} else {
			data["devices"] = result
		}
	}

	if strings.Contains(query, "reminder(") || strings.Contains(query, "reminder (") {
		if idVar, ok := req.Variables["id"]; ok {
			if idStr, ok := idVar.(string); ok {
				id, err := uuid.Parse(idStr)
				if err == nil {
					result, err := h.Resolver.Reminder(ctx, id)
					if err != nil {
						errs = append(errs, errorToGraphQLError(err))
						data["reminder"] = nil
					} else {
						data["reminder"] = result
					}
				}
			}
		}
	}

	if strings.Contains(query, "reminders") && !strings.Contains(query, "reminder(") {
		var filter *model.ReminderFilter
		var pagination *model.PaginationInput

		if filterVar, ok := req.Variables["filter"]; ok {
			filterBytes, _ := json.Marshal(filterVar)
			json.Unmarshal(filterBytes, &filter)
		}
		if paginationVar, ok := req.Variables["pagination"]; ok {
			paginationBytes, _ := json.Marshal(paginationVar)
			json.Unmarshal(paginationBytes, &pagination)
		}

		result, err := h.Resolver.Reminders(ctx, filter, pagination)
		if err != nil {
			fmt.Printf("Reminders query error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			data["reminders"] = nil
		} else {
			fmt.Printf("Reminders query result: %d edges\n", len(result.Edges))
			data["reminders"] = result
		}
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	fmt.Printf("executeQuery response: data keys=%v, errors=%d\n", keys, len(errs))
	return GraphQLResponse{Data: data, Errors: errs}
}

// executeMutation handles mutation operations
func (h *Handler) executeMutation(ctx context.Context, req GraphQLRequest) GraphQLResponse {
	data := make(map[string]interface{})
	var errs []GraphQLError

	query := strings.ToLower(req.Query)
	fmt.Printf("executeMutation: opName=%q, query=%q\n", req.OperationName, query)

	if strings.Contains(query, "authenticatewithgoogle") {
		idToken, _ := req.Variables["idToken"].(string)
		result, err := h.Resolver.AuthenticateWithGoogle(ctx, idToken)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["authenticateWithGoogle"] = result
		}
	}

	// Check for refreshToken mutation - must handle various formatting (newlines, spaces)
	// Match when refreshtoken is used as a mutation operation, not as a field in response selection
	isRefreshTokenMutation := strings.Contains(query, "mutation") &&
		(strings.Contains(query, "refreshtoken(") || strings.Contains(query, "refreshtoken ("))
	if isRefreshTokenMutation {
		// Try both camelCase and lowercase variable names
		refreshToken, _ := req.Variables["refreshToken"].(string)
		if refreshToken == "" {
			refreshToken, _ = req.Variables["refreshtoken"].(string)
		}
		fmt.Printf("RefreshToken mutation detected, refreshToken length=%d\n", len(refreshToken))
		result, err := h.Resolver.RefreshToken(ctx, refreshToken)
		if err != nil {
			fmt.Printf("RefreshToken error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			fmt.Printf("RefreshToken success, new accessToken length=%d\n", len(result.AccessToken))
			data["refreshToken"] = result
		}
	}

	if strings.Contains(query, "logout") {
		result, err := h.Resolver.Logout(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["logout"] = nil
		} else {
			data["logout"] = result
		}
	}

	if strings.Contains(query, "verifysubscription") {
		result, err := h.Resolver.VerifySubscription(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["verifySubscription"] = result
		}
	}

	if strings.Contains(query, "createreminder") {
		var input model.CreateReminderInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			if err := json.Unmarshal(inputBytes, &input); err != nil {
				fmt.Printf("CreateReminder input unmarshal error: %v\n", err)
				fmt.Printf("CreateReminder raw input: %s\n", string(inputBytes))
			}
		}
		result, err := h.Resolver.CreateReminder(ctx, input)
		if err != nil {
			fmt.Printf("CreateReminder error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			// Debug: print the result as JSON
			resultJSON, _ := json.Marshal(result)
			fmt.Printf("CreateReminder result: %s\n", string(resultJSON))
			data["createReminder"] = result
		}
	}

	if strings.Contains(query, "updatereminder") {
		fmt.Printf("UpdateReminder mutation detected\n")
		idStr, _ := req.Variables["id"].(string)
		fmt.Printf("UpdateReminder id: %q\n", idStr)
		id, parseErr := uuid.Parse(idStr)
		if parseErr != nil {
			fmt.Printf("UpdateReminder id parse error: %v\n", parseErr)
		}
		var input model.UpdateReminderInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			fmt.Printf("UpdateReminder raw input: %s\n", string(inputBytes))
			if unmarshalErr := json.Unmarshal(inputBytes, &input); unmarshalErr != nil {
				fmt.Printf("UpdateReminder input unmarshal error: %v\n", unmarshalErr)
			}
			fmt.Printf("UpdateReminder parsed input: %+v\n", input)
		} else {
			fmt.Printf("UpdateReminder: no input variable found\n")
		}
		result, err := h.Resolver.UpdateReminder(ctx, id, input)
		if err != nil {
			fmt.Printf("UpdateReminder error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// Don't include data["updateReminder"] = nil for non-nullable return type
			// GraphQL spec: on error for non-nullable, omit the data field entirely
		} else {
			// Debug: print the result as JSON
			resultJSON, _ := json.Marshal(result)
			fmt.Printf("UpdateReminder result: %s\n", string(resultJSON))
			data["updateReminder"] = result
		}
	}

	if strings.Contains(query, "deletereminder") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		result, err := h.Resolver.DeleteReminder(ctx, id)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["deleteReminder"] = nil
		} else {
			data["deleteReminder"] = result
		}
	}

	if strings.Contains(query, "snoozereminder") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		minutes, _ := req.Variables["minutes"].(float64)
		result, err := h.Resolver.SnoozeReminder(ctx, id, int(minutes))
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["snoozeReminder"] = result
		}
	}

	if strings.Contains(query, "completereminder") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		result, err := h.Resolver.CompleteReminder(ctx, id)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["completeReminder"] = result
		}
	}

	if strings.Contains(query, "dismissreminder") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		result, err := h.Resolver.DismissReminder(ctx, id)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["dismissReminder"] = nil
		} else {
			data["dismissReminder"] = result
		}
	}

	if strings.Contains(query, "registerdevice") {
		var input model.RegisterDeviceInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			json.Unmarshal(inputBytes, &input)
		}
		result, err := h.Resolver.RegisterDevice(ctx, input)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["registerDevice"] = result
		}
	}

	if strings.Contains(query, "unregisterdevice") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		result, err := h.Resolver.UnregisterDevice(ctx, id)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["unregisterDevice"] = nil
		} else {
			data["unregisterDevice"] = result
		}
	}

	// Per GraphQL spec: if any error occurs on a non-nullable field,
	// data should be null (not an empty object or object with null field)
	if len(errs) > 0 && len(data) == 0 {
		return GraphQLResponse{Data: nil, Errors: errs}
	}
	return GraphQLResponse{Data: data, Errors: errs}
}

func errorToGraphQLError(err error) GraphQLError {
	if appErr := apperrors.GetAppError(err); appErr != nil {
		return GraphQLError{
			Message: appErr.Message,
			Extensions: map[string]interface{}{
				"code": appErr.Code,
			},
		}
	}
	return GraphQLError{Message: err.Error()}
}

// Playground serves the GraphQL Playground UI
func (h *Handler) Playground(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, playgroundHTML)
}

var playgroundHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset=utf-8/>
  <meta name="viewport" content="user-scalable=no, initial-scale=1.0, minimum-scale=1.0, maximum-scale=1.0, minimal-ui">
  <title>Jolt GraphQL Playground</title>
  <link rel="stylesheet" href="//cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="//cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="//cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root">
    <style>
      body { background-color: rgb(23, 42, 58); font-family: Open Sans, sans-serif; height: 90vh; }
      #root { height: 100%; width: 100%; display: flex; align-items: center; justify-content: center; }
      .loading { font-size: 32px; font-weight: 200; color: rgba(255, 255, 255, .6); margin-left: 28px; }
      img { width: 78px; height: 78px; }
      .title { font-weight: 400; }
    </style>
    <img src='//cdn.jsdelivr.net/npm/graphql-playground-react/build/logo.png' alt=''>
    <div class="loading"> Loading
      <span class="title">Jolt GraphQL</span>
    </div>
  </div>
  <script>window.addEventListener('load', function (event) {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/graphql',
        subscriptionEndpoint: '/graphql'
      })
    })</script>
</body>
</html>`

// WebSocket upgrader for graphql-ws protocol
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	Subprotocols: []string{"graphql-transport-ws"},
}

// WebSocketHandler handles GraphQL subscriptions over WebSocket using graphql-transport-ws protocol
func (h *Handler) WebSocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var userID uuid.UUID
	var authenticated bool
	subscriptions := make(map[string]context.CancelFunc)

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Handle keep-alive in background
	go func() {
		for range ticker.C {
			conn.WriteJSON(map[string]string{"type": "ka"})
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// Clean up all subscriptions on disconnect
			for _, cancel := range subscriptions {
				cancel()
			}
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)

		switch msgType {
		case "connection_init":
			// Handle authentication from connection params
			if payload, ok := msg["payload"].(map[string]interface{}); ok {
				// Try different auth header formats
				var authToken string
				if auth, ok := payload["Authorization"].(string); ok {
					authToken = auth
				} else if auth, ok := payload["authorization"].(string); ok {
					authToken = auth
				} else if headers, ok := payload["headers"].(map[string]interface{}); ok {
					if auth, ok := headers["Authorization"].(string); ok {
						authToken = auth
					}
				}

				if authToken != "" {
					token := strings.TrimPrefix(authToken, "Bearer ")
					claims, err := h.JWTManager.ValidateToken(token)
					if err == nil {
						userID = claims.UserID
						authenticated = true
					}
				}
			}
			conn.WriteJSON(map[string]string{"type": "connection_ack"})

		case "subscribe":
			if !authenticated {
				conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"id":      msg["id"],
					"payload": []map[string]string{{"message": "Unauthorized"}},
				})
				continue
			}

			id, _ := msg["id"].(string)

			// Create context with user ID and cancellation
			ctx, cancel := context.WithCancel(context.Background())
			ctx = gqlmiddleware.WithUserID(ctx, userID)
			subscriptions[id] = cancel

			// Start subscription
			eventChan, err := h.Resolver.ReminderChanged(ctx)
			if err != nil {
				conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"id":      id,
					"payload": []map[string]string{{"message": err.Error()}},
				})
				cancel()
				delete(subscriptions, id)
				continue
			}

			// Send events to client in background
			go func(subID string) {
				defer func() {
					delete(subscriptions, subID)
				}()

				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-eventChan:
						if !ok {
							// Channel closed, send complete
							conn.WriteJSON(map[string]interface{}{
								"type": "complete",
								"id":   subID,
							})
							return
						}
						conn.WriteJSON(map[string]interface{}{
							"type": "next",
							"id":   subID,
							"payload": map[string]interface{}{
								"data": map[string]interface{}{
									"reminderChanged": event,
								},
							},
						})
					}
				}
			}(id)

		case "complete":
			// Client wants to unsubscribe
			id, _ := msg["id"].(string)
			if cancel, ok := subscriptions[id]; ok {
				cancel()
				delete(subscriptions, id)
			}

		case "ping":
			conn.WriteJSON(map[string]string{"type": "pong"})
		}
	}
}
