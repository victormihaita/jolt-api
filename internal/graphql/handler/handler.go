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
	Data   interface{}    `json:"data"`
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

// GraphQL handles GraphQL HTTP POST requests
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

	// Set no-cache headers to prevent any caching
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.JSON(http.StatusOK, result)
}

// GraphQLGet handles GraphQL HTTP GET requests (query passed as URL parameter)
func (h *Handler) GraphQLGet(c *gin.Context) {
	req := GraphQLRequest{
		Query:         c.Query("query"),
		OperationName: c.Query("operationName"),
	}

	// Parse variables from query string if present
	if varsStr := c.Query("variables"); varsStr != "" {
		var vars map[string]interface{}
		if err := json.Unmarshal([]byte(varsStr), &vars); err == nil {
			req.Variables = vars
		}
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

	// Handle introspection queries
	// Note: We check for "__schema" and "__type(" with parenthesis to avoid matching "__typename"
	// which is a standard field in GraphQL queries but not an introspection query
	if strings.Contains(query, "__schema") || strings.Contains(query, "__type(") || opName == "introspectionquery" {
		return GraphQLResponse{Data: getIntrospectionData()}
	}

	// Check for "me" query - match by operation name to avoid false positives
	// "reminders" contains "me" so we can't just use strings.Contains
	if opName == "me" || opName == "getme" {
		result, err := h.Resolver.Me(ctx)
		if err != nil {
			fmt.Printf("Me query error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// For non-nullable return type, return early with null data
			return GraphQLResponse{Data: nil, Errors: errs}
		} else {
			fmt.Printf("Me query result: %+v\n", result)
			data["me"] = result
		}
	}

	if opName == "devices" || opName == "getdevices" {
		result, err := h.Resolver.Devices(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			// For non-nullable return type, return early with null data
			return GraphQLResponse{Data: nil, Errors: errs}
		} else {
			data["devices"] = result
		}
	}

	if opName == "reminder" || opName == "getreminder" {
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

	if opName == "reminders" || opName == "getreminders" {
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
			// For non-nullable return type, don't set data["reminders"] = nil
			// GraphQL spec: if a non-nullable field errors, data should be null
			// So we return early with null data
			return GraphQLResponse{Data: nil, Errors: errs}
		} else {
			fmt.Printf("Reminders query result: %d edges\n", len(result.Edges))
			// Debug: print full JSON of result
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Reminders query result JSON:\n%s\n", string(resultJSON))
			data["reminders"] = result
		}
	}

	// ReminderList query - get single list by ID
	if opName == "reminderlist" || opName == "getreminderlist" {
		if idVar, ok := req.Variables["id"]; ok {
			if idStr, ok := idVar.(string); ok {
				id, err := uuid.Parse(idStr)
				if err == nil {
					result, err := h.Resolver.ReminderList(ctx, id)
					if err != nil {
						errs = append(errs, errorToGraphQLError(err))
						data["reminderList"] = nil
					} else {
						data["reminderList"] = result
					}
				}
			}
		}
	}

	// ReminderLists query - get all lists for current user
	if opName == "reminderlists" || opName == "getreminderlists" {
		result, err := h.Resolver.ReminderLists(ctx)
		if err != nil {
			fmt.Printf("ReminderLists query error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			return GraphQLResponse{Data: nil, Errors: errs}
		} else {
			fmt.Printf("ReminderLists query result: %d lists\n", len(result))
			data["reminderLists"] = result
		}
	}

	// Per GraphQL spec: if any error occurs on a non-nullable field,
	// data should be null. We handle this case above for reminders.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	fmt.Printf("executeQuery response: data keys=%v, errors=%d\n", keys, len(errs))

	// Debug: print full response JSON
	resp := GraphQLResponse{Data: data, Errors: errs}
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("executeQuery full response JSON:\n%s\n", string(respJSON))

	return resp
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

	if strings.Contains(query, "authenticatewithapple") {
		var input model.AuthenticateWithAppleInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			json.Unmarshal(inputBytes, &input)
		}
		result, err := h.Resolver.AuthenticateWithApple(ctx, input)
		if err != nil {
			fmt.Printf("AuthenticateWithApple error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			data["authenticateWithApple"] = result
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

	if strings.Contains(query, "deleteaccount") {
		result, err := h.Resolver.DeleteAccount(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["deleteAccount"] = nil
		} else {
			data["deleteAccount"] = result
		}
	}

	if strings.Contains(query, "restoreaccount") {
		result, err := h.Resolver.RestoreAccount(ctx)
		if err != nil {
			errs = append(errs, errorToGraphQLError(err))
			data["restoreAccount"] = nil
		} else {
			data["restoreAccount"] = result
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

	if strings.Contains(query, "createreminder") && !strings.Contains(query, "createreminderlist") {
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

	if strings.Contains(query, "updatereminder") && !strings.Contains(query, "updatereminderlist") {
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

	if strings.Contains(query, "deletereminder") && !strings.Contains(query, "deletereminderlist") {
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

	// Note: Check for "unregisterdevice" first to avoid false positive,
	// since "unregisterdevice" contains "registerdevice" as a substring
	if strings.Contains(query, "registerdevice") && !strings.Contains(query, "unregisterdevice") {
		fmt.Printf("RegisterDevice mutation detected\n")
		var input model.RegisterDeviceInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			fmt.Printf("RegisterDevice raw input: %s\n", string(inputBytes))
			if err := json.Unmarshal(inputBytes, &input); err != nil {
				fmt.Printf("RegisterDevice input unmarshal error: %v\n", err)
			}
		} else {
			fmt.Printf("RegisterDevice: no input variable found in request\n")
		}
		fmt.Printf("RegisterDevice parsed input: platform=%v, pushToken=%s\n", input.Platform, input.PushToken)
		result, err := h.Resolver.RegisterDevice(ctx, input)
		if err != nil {
			fmt.Printf("RegisterDevice error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			// Don't set data for non-nullable return type on error
		} else {
			fmt.Printf("RegisterDevice success: device ID=%s\n", result.ID)
			data["registerDevice"] = result
		}
	}

	// ReminderList mutations
	if strings.Contains(query, "createreminderlist") {
		var input model.CreateReminderListInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			json.Unmarshal(inputBytes, &input)
		}
		result, err := h.Resolver.CreateReminderList(ctx, input)
		if err != nil {
			fmt.Printf("CreateReminderList error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
		} else {
			data["createReminderList"] = result
		}
	}

	if strings.Contains(query, "updatereminderlist") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		var input model.UpdateReminderListInput
		if inputVar, ok := req.Variables["input"]; ok {
			inputBytes, _ := json.Marshal(inputVar)
			json.Unmarshal(inputBytes, &input)
		}
		result, err := h.Resolver.UpdateReminderList(ctx, id, input)
		if err != nil {
			fmt.Printf("UpdateReminderList error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
		} else {
			data["updateReminderList"] = result
		}
	}

	if strings.Contains(query, "deletereminderlist") {
		idStr, _ := req.Variables["id"].(string)
		id, _ := uuid.Parse(idStr)
		result, err := h.Resolver.DeleteReminderList(ctx, id)
		if err != nil {
			fmt.Printf("DeleteReminderList error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
			data["deleteReminderList"] = false
		} else {
			data["deleteReminderList"] = result
		}
	}

	if strings.Contains(query, "reorderreminderlists") {
		var ids []uuid.UUID
		if idsVar, ok := req.Variables["ids"].([]interface{}); ok {
			for _, idVar := range idsVar {
				if idStr, ok := idVar.(string); ok {
					if id, err := uuid.Parse(idStr); err == nil {
						ids = append(ids, id)
					}
				}
			}
		}
		result, err := h.Resolver.ReorderReminderLists(ctx, ids)
		if err != nil {
			fmt.Printf("ReorderReminderLists error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
		} else {
			data["reorderReminderLists"] = result
		}
	}

	if strings.Contains(query, "moveremindertolist") {
		reminderIDStr, _ := req.Variables["reminderId"].(string)
		reminderID, _ := uuid.Parse(reminderIDStr)
		listIDStr, _ := req.Variables["listId"].(string)
		listID, _ := uuid.Parse(listIDStr)
		result, err := h.Resolver.MoveReminderToList(ctx, reminderID, listID)
		if err != nil {
			fmt.Printf("MoveReminderToList error: %v\n", err)
			errs = append(errs, errorToGraphQLError(err))
		} else {
			data["moveReminderToList"] = result
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
  <title>Zolt GraphQL Playground</title>
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
      <span class="title">Zolt GraphQL</span>
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

// getIntrospectionData returns the schema introspection data
func getIntrospectionData() map[string]interface{} {
	return map[string]interface{}{
		"__schema": map[string]interface{}{
			"queryType":        map[string]interface{}{"name": "Query"},
			"mutationType":     map[string]interface{}{"name": "Mutation"},
			"subscriptionType": map[string]interface{}{"name": "Subscription"},
			"types":            getSchemaTypes(),
			"directives":       []interface{}{},
		},
	}
}

func getSchemaTypes() []map[string]interface{} {
	return []map[string]interface{}{
		// Scalars
		{"kind": "SCALAR", "name": "String", "description": "Built-in String"},
		{"kind": "SCALAR", "name": "Int", "description": "Built-in Int"},
		{"kind": "SCALAR", "name": "Float", "description": "Built-in Float"},
		{"kind": "SCALAR", "name": "Boolean", "description": "Built-in Boolean"},
		{"kind": "SCALAR", "name": "ID", "description": "Built-in ID"},
		{"kind": "SCALAR", "name": "DateTime", "description": "ISO 8601 datetime"},
		{"kind": "SCALAR", "name": "UUID", "description": "UUID string"},
		// Enums
		{
			"kind": "ENUM", "name": "Platform", "description": "Device platform",
			"enumValues": []map[string]interface{}{
				{"name": "IOS", "description": "iOS platform"},
				{"name": "ANDROID", "description": "Android platform"},
			},
		},
		{
			"kind": "ENUM", "name": "Priority", "description": "Reminder priority",
			"enumValues": []map[string]interface{}{
				{"name": "NONE", "description": "No priority"},
				{"name": "LOW", "description": "Low priority"},
				{"name": "NORMAL", "description": "Normal priority"},
				{"name": "HIGH", "description": "High priority"},
			},
		},
		{
			"kind": "ENUM", "name": "ReminderStatus", "description": "Reminder status",
			"enumValues": []map[string]interface{}{
				{"name": "ACTIVE", "description": "Active reminder"},
				{"name": "COMPLETED", "description": "Completed reminder"},
				{"name": "SNOOZED", "description": "Snoozed reminder"},
				{"name": "DISMISSED", "description": "Dismissed reminder"},
			},
		},
		{
			"kind": "ENUM", "name": "Frequency", "description": "Recurrence frequency",
			"enumValues": []map[string]interface{}{
				{"name": "HOURLY", "description": "Hourly"},
				{"name": "DAILY", "description": "Daily"},
				{"name": "WEEKLY", "description": "Weekly"},
				{"name": "MONTHLY", "description": "Monthly"},
				{"name": "YEARLY", "description": "Yearly"},
			},
		},
		{
			"kind": "ENUM", "name": "ChangeAction", "description": "Subscription change action",
			"enumValues": []map[string]interface{}{
				{"name": "CREATED", "description": "Created"},
				{"name": "UPDATED", "description": "Updated"},
				{"name": "DELETED", "description": "Deleted"},
			},
		},
		// Objects
		{
			"kind": "OBJECT", "name": "User", "description": "User account",
			"fields": []map[string]interface{}{
				{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}},
				{"name": "email", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "displayName", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "avatarUrl", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "timezone", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "isPremium", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "premiumUntil", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "createdAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
				{"name": "updatedAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "AuthPayload", "description": "Authentication response",
			"fields": []map[string]interface{}{
				{"name": "accessToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "refreshToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "expiresIn", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
				{"name": "user", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "User"}}},
				{"name": "accountPendingDeletion", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "Device", "description": "Registered device",
			"fields": []map[string]interface{}{
				{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}},
				{"name": "platform", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "Platform"}}},
				{"name": "pushToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "deviceName", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "appVersion", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "osVersion", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "lastSeenAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
				{"name": "createdAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "Reminder", "description": "Reminder item",
			"fields": []map[string]interface{}{
				{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}},
				{"name": "title", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "notes", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "priority", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "Priority"}}},
				{"name": "dueAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
				{"name": "allDay", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "recurrenceRule", "type": map[string]interface{}{"kind": "OBJECT", "name": "RecurrenceRule"}},
				{"name": "recurrenceEnd", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "status", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "ReminderStatus"}}},
				{"name": "completedAt", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "snoozedUntil", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "snoozeCount", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
				{"name": "localId", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "version", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
				{"name": "createdAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
				{"name": "updatedAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "RecurrenceRule", "description": "Recurrence rule",
			"fields": []map[string]interface{}{
				{"name": "frequency", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "Frequency"}}},
				{"name": "interval", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
				{"name": "daysOfWeek", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}}},
				{"name": "dayOfMonth", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "monthOfYear", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "endAfterOccurrences", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "endDate", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
			},
		},
		{
			"kind": "OBJECT", "name": "PageInfo", "description": "Pagination info",
			"fields": []map[string]interface{}{
				{"name": "hasNextPage", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "hasPreviousPage", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "startCursor", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "endCursor", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
			},
		},
		{
			"kind": "OBJECT", "name": "ReminderConnection", "description": "Paginated reminders",
			"fields": []map[string]interface{}{
				{"name": "edges", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "ReminderEdge"}}}}},
				{"name": "pageInfo", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "PageInfo"}}},
				{"name": "totalCount", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "ReminderEdge", "description": "Reminder edge",
			"fields": []map[string]interface{}{
				{"name": "node", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}}},
				{"name": "cursor", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
			},
		},
		{
			"kind": "OBJECT", "name": "ReminderChangeEvent", "description": "Subscription event",
			"fields": []map[string]interface{}{
				{"name": "action", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "ChangeAction"}}},
				{"name": "reminder", "type": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}},
				{"name": "reminderId", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}},
				{"name": "timestamp", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
			},
		},
		// Input types
		{
			"kind": "INPUT_OBJECT", "name": "PaginationInput", "description": "Pagination input",
			"inputFields": []map[string]interface{}{
				{"name": "first", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "after", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "last", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "before", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
			},
		},
		{
			"kind": "INPUT_OBJECT", "name": "RegisterDeviceInput", "description": "Register device input",
			"inputFields": []map[string]interface{}{
				{"name": "platform", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "Platform"}}},
				{"name": "pushToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "deviceName", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "appVersion", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "osVersion", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
			},
		},
		{
			"kind": "INPUT_OBJECT", "name": "RecurrenceRuleInput", "description": "Recurrence rule input",
			"inputFields": []map[string]interface{}{
				{"name": "frequency", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "ENUM", "name": "Frequency"}}},
				{"name": "interval", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}},
				{"name": "daysOfWeek", "type": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}}},
				{"name": "dayOfMonth", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "monthOfYear", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "endAfterOccurrences", "type": map[string]interface{}{"kind": "SCALAR", "name": "Int"}},
				{"name": "endDate", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
			},
		},
		{
			"kind": "INPUT_OBJECT", "name": "CreateReminderInput", "description": "Create reminder input",
			"inputFields": []map[string]interface{}{
				{"name": "title", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}},
				{"name": "notes", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "priority", "type": map[string]interface{}{"kind": "ENUM", "name": "Priority"}},
				{"name": "dueAt", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}}},
				{"name": "allDay", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "recurrenceRule", "type": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "RecurrenceRuleInput"}},
				{"name": "recurrenceEnd", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "localId", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
			},
		},
		{
			"kind": "INPUT_OBJECT", "name": "UpdateReminderInput", "description": "Update reminder input",
			"inputFields": []map[string]interface{}{
				{"name": "title", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "notes", "type": map[string]interface{}{"kind": "SCALAR", "name": "String"}},
				{"name": "priority", "type": map[string]interface{}{"kind": "ENUM", "name": "Priority"}},
				{"name": "dueAt", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "allDay", "type": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}},
				{"name": "recurrenceRule", "type": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "RecurrenceRuleInput"}},
				{"name": "recurrenceEnd", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "status", "type": map[string]interface{}{"kind": "ENUM", "name": "ReminderStatus"}},
			},
		},
		{
			"kind": "INPUT_OBJECT", "name": "ReminderFilter", "description": "Reminder filter",
			"inputFields": []map[string]interface{}{
				{"name": "status", "type": map[string]interface{}{"kind": "ENUM", "name": "ReminderStatus"}},
				{"name": "fromDate", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "toDate", "type": map[string]interface{}{"kind": "SCALAR", "name": "DateTime"}},
				{"name": "priority", "type": map[string]interface{}{"kind": "ENUM", "name": "Priority"}},
			},
		},
		// Query type
		{
			"kind": "OBJECT", "name": "Query", "description": "Root query type",
			"fields": []map[string]interface{}{
				{"name": "me", "description": "Get current user", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "User"}}},
				{"name": "reminder", "description": "Get reminder by ID", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}}, "type": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}},
				{"name": "reminders", "description": "Get reminders", "args": []map[string]interface{}{{"name": "filter", "type": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "ReminderFilter"}}, {"name": "pagination", "type": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "PaginationInput"}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "ReminderConnection"}}},
				{"name": "devices", "description": "Get user devices", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "LIST", "ofType": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Device"}}}}},
			},
		},
		// Mutation type
		{
			"kind": "OBJECT", "name": "Mutation", "description": "Root mutation type",
			"fields": []map[string]interface{}{
				{"name": "authenticateWithGoogle", "description": "Authenticate with Google", "args": []map[string]interface{}{{"name": "idToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "AuthPayload"}}},
				{"name": "refreshToken", "description": "Refresh access token", "args": []map[string]interface{}{{"name": "refreshToken", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "String"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "AuthPayload"}}},
				{"name": "logout", "description": "Logout", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "verifySubscription", "description": "Verify subscription", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "User"}}},
				{"name": "createReminder", "description": "Create reminder", "args": []map[string]interface{}{{"name": "input", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "CreateReminderInput"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}}},
				{"name": "updateReminder", "description": "Update reminder", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}, {"name": "input", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "UpdateReminderInput"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}}},
				{"name": "deleteReminder", "description": "Delete reminder", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "snoozeReminder", "description": "Snooze reminder", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}, {"name": "minutes", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Int"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}}},
				{"name": "completeReminder", "description": "Complete reminder", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Reminder"}}},
				{"name": "dismissReminder", "description": "Dismiss reminder", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "registerDevice", "description": "Register device", "args": []map[string]interface{}{{"name": "input", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "INPUT_OBJECT", "name": "RegisterDeviceInput"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "Device"}}},
				{"name": "unregisterDevice", "description": "Unregister device", "args": []map[string]interface{}{{"name": "id", "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "UUID"}}}}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "deleteAccount", "description": "Delete account", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
				{"name": "restoreAccount", "description": "Restore account after deletion", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "SCALAR", "name": "Boolean"}}},
			},
		},
		// Subscription type
		{
			"kind": "OBJECT", "name": "Subscription", "description": "Root subscription type",
			"fields": []map[string]interface{}{
				{"name": "reminderChanged", "description": "Subscribe to reminder changes", "args": []interface{}{}, "type": map[string]interface{}{"kind": "NON_NULL", "ofType": map[string]interface{}{"kind": "OBJECT", "name": "ReminderChangeEvent"}}},
			},
		},
	}
}

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
