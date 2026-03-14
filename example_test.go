package burrow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/oliverandrich/burrow"
)

func ExampleHandle() {
	handler := burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.Text(w, http.StatusOK, "hello")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler(w, r)

	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 200
	// hello
}

func ExampleHandle_error() {
	handler := burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.NewHTTPError(http.StatusNotFound, "item not found")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items/42", nil)
	handler(w, r)

	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 404
	// item not found
}

func ExampleJSON() {
	w := httptest.NewRecorder()
	_ = burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})

	fmt.Println(w.Header().Get("Content-Type"))
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// application/json
	// {"status":"ok"}
}

func ExampleBind() {
	type Input struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	body := `{"name":"Alice","email":"alice@example.com"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var input Input
	err := burrow.Bind(r, &input)

	fmt.Println(err)
	fmt.Println(input.Name, input.Email)
	// Output:
	// <nil>
	// Alice alice@example.com
}

func ExampleBind_validationError() {
	type Input struct {
		Email string `json:"email" validate:"required,email"`
	}

	body := `{"email":"not-an-email"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var input Input
	err := burrow.Bind(r, &input)

	fmt.Println(err)
	// Output:
	// validation failed: email is email
}

func ExampleNewHTTPError() {
	err := burrow.NewHTTPError(http.StatusForbidden, "access denied")
	fmt.Println(err.Code)
	fmt.Println(err.Error())
	// Output:
	// 403
	// access denied
}

func ExampleValidate() {
	type User struct {
		Name string `validate:"required"`
		Age  int    `validate:"gte=0,lte=150"`
	}

	err := burrow.Validate(&User{Name: "Alice", Age: 30})
	fmt.Println("valid:", err)

	err = burrow.Validate(&User{Name: "", Age: -1})
	fmt.Println("invalid:", err)
	// Output:
	// valid: <nil>
	// invalid: validation failed: Name is required; Age is gte
}

func ExampleValidationError_HasField() {
	type Form struct {
		Email string `validate:"required,email"`
		Name  string `validate:"required"`
	}

	err := burrow.Validate(&Form{})

	var ve *burrow.ValidationError
	if errors.As(err, &ve) {
		fmt.Println("has Email error:", ve.HasField("Email"))
		fmt.Println("has Age error:", ve.HasField("Age"))
	}
	// Output:
	// has Email error: true
	// has Age error: false
}

func ExampleParsePageRequest() {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?page=2&limit=10", nil)
	pr := burrow.ParsePageRequest(r)

	fmt.Println("page:", pr.Page)
	fmt.Println("limit:", pr.Limit)
	fmt.Println("offset:", pr.Offset())
	// Output:
	// page: 2
	// limit: 10
	// offset: 10
}

func ExampleOffsetResult() {
	pr := burrow.PageRequest{Limit: 10, Page: 2}
	result := burrow.OffsetResult(pr, 25)

	fmt.Println("page:", result.Page)
	fmt.Println("total_pages:", result.TotalPages)
	fmt.Println("total_count:", result.TotalCount)
	fmt.Println("has_more:", result.HasMore)
	// Output:
	// page: 2
	// total_pages: 3
	// total_count: 25
	// has_more: true
}

func ExamplePageResponse() {
	resp := burrow.PageResponse[string]{
		Items:      []string{"alice", "bob"},
		Pagination: burrow.OffsetResult(burrow.PageRequest{Limit: 2, Page: 1}, 5),
	}

	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
	// Output:
	// {"items":["alice","bob"],"pagination":{"has_more":true,"page":1,"total_pages":3,"total_count":5}}
}
