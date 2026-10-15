package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func NewSearchRequest(limit, offset int, query, orderField string, orderBy int) *SearchRequest {
	return &SearchRequest{
		Limit:      limit,
		Offset:     offset,
		Query:      query,
		OrderField: orderField,
		OrderBy:    orderBy,
	}
}

func NewSearchClient(accessToken, URL string) *SearchClient {
	return &SearchClient{
		AccessToken: accessToken,
		URL:         URL,
	}
}

type TestCase struct {
	name             string
	request          SearchRequest
	expectedIDs      []int
	expectedNextPage bool
}

func TestSearchServerRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	testCases := []TestCase{
		{
			name: "search by name",
			request: SearchRequest{
				Limit:      1,
				Query:      "Boyd",
				OrderField: "name",
				OrderBy:    OrderByAsc,
			},
			expectedIDs: []int{0},
		},
		{
			name: "search by about",
			request: SearchRequest{
				Limit:      1,
				Query:      "consectetur minim amet ex",
				OrderField: "name",
				OrderBy:    OrderByAsc,
			},
			expectedIDs: []int{1},
		},
		{
			name: "sort by age ascending",
			request: SearchRequest{
				Limit:      2,
				OrderField: "age",
				OrderBy:    OrderByAsc,
			},
			expectedIDs:      []int{1, 15},
			expectedNextPage: true,
		},
		{
			name: "sort by age descending",
			request: SearchRequest{
				Limit:      2,
				OrderField: "age",
				OrderBy:    OrderByDesc,
			},
			expectedIDs:      []int{13, 32},
			expectedNextPage: true,
		},
		{
			name: "pagination",
			request: SearchRequest{
				Limit:      2,
				Offset:     2,
				OrderField: "id",
				OrderBy:    OrderByAsc,
			},
			expectedIDs:      []int{2, 3},
			expectedNextPage: true,
		},
		{
			name: "keep source order",
			request: SearchRequest{
				Limit:   2,
				OrderBy: OrderByAsIs,
			},
			expectedIDs:      []int{0, 1},
			expectedNextPage: true,
		},
		{
			name: "sort by name",
			request: SearchRequest{
				Limit:      2,
				OrderField: "name",
				OrderBy:    OrderByAsc,
			},
			expectedIDs:      []int{15, 16},
			expectedNextPage: true,
		},
		{
			name: "offset past result",
			request: SearchRequest{
				Limit:      2,
				Offset:     100,
				OrderField: "id",
				OrderBy:    OrderByAsc,
			},
			expectedIDs: []int{},
		},
	}

	client := NewSearchClient(token, ts.URL)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := client.FindUsers(testCase.request)
			if err != nil {
				t.Fatalf("FindUsers returned an error: %v", err)
			}

			ids := make([]int, 0, len(result.Users))
			for _, user := range result.Users {
				ids = append(ids, user.ID)
			}

			if !reflect.DeepEqual(ids, testCase.expectedIDs) {
				t.Fatalf("unexpected IDs: got %v, want %v", ids, testCase.expectedIDs)
			}
			if result.NextPage != testCase.expectedNextPage {
				t.Fatalf("unexpected NextPage: got %v, want %v",
					result.NextPage, testCase.expectedNextPage)
			}
		})
	}
}

func TestSearchServerErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	testCases := []struct {
		name        string
		accessToken string
		request     SearchRequest
		expectedErr string
	}{
		{
			name:        "wrong access token",
			accessToken: wrongToken,
			request:     SearchRequest{Limit: 1},
			expectedErr: "bad AccessToken",
		},
		{
			name:        "wrong order field",
			accessToken: token,
			request: SearchRequest{
				Limit:      1,
				OrderField: "email",
			},
			expectedErr: "OrderFeld email invalid",
		},
		{
			name:        "wrong order by",
			accessToken: token,
			request: SearchRequest{
				Limit:   1,
				OrderBy: 42,
			},
			expectedErr: "OrderFeld  invalid",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := NewSearchClient(testCase.accessToken, ts.URL)
			_, err := client.FindUsers(testCase.request)
			if err == nil {
				t.Fatal("expected an error")
			}
			if err.Error() != testCase.expectedErr {
				t.Fatalf("unexpected error: got %q, want %q",
					err.Error(), testCase.expectedErr)
			}
		})
	}
}

func TestFindUsersValidation(t *testing.T) {
	client := NewSearchClient(token, "http://invalid")

	testCases := []struct {
		name        string
		request     SearchRequest
		expectedErr string
	}{
		{
			name:        "negative limit",
			request:     SearchRequest{Limit: -1},
			expectedErr: "limit must be > 0",
		},
		{
			name:        "negative offset",
			request:     SearchRequest{Offset: -1},
			expectedErr: "offset must be > 0",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := client.FindUsers(testCase.request)
			if err == nil {
				t.Fatal("expected an error")
			}
			if err.Error() != testCase.expectedErr {
				t.Fatalf("unexpected error: got %q, want %q",
					err.Error(), testCase.expectedErr)
			}
		})
	}
}

func TestFindUsersLimitCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "26" {
			t.Errorf("unexpected limit: got %q, want %q", got, "26")
		}
		_ = json.NewEncoder(w).Encode([]User{})
	}))
	defer ts.Close()

	client := NewSearchClient(token, ts.URL)
	if _, err := client.FindUsers(SearchRequest{Limit: 26}); err != nil {
		t.Fatalf("FindUsers returned an error: %v", err)
	}
}

func TestFindUsersConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()

	_, err := NewSearchClient(token, url).FindUsers(SearchRequest{Limit: 1})
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if got := err.Error(); got == "bad AccessToken" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestFindUsersTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1100 * time.Millisecond)
	}))
	defer ts.Close()

	_, err := NewSearchClient(token, ts.URL).FindUsers(SearchRequest{Limit: 1})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if len(err.Error()) < len("timeout for ") ||
		err.Error()[:len("timeout for ")] != "timeout for " {
		t.Fatalf("unexpected error: %q", err)
	}
}

func TestFindUsersMalformedResponses(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		body        string
		expectedErr string
	}{
		{
			name:        "malformed bad request",
			status:      http.StatusBadRequest,
			body:        "not json",
			expectedErr: "cant unpack error json:",
		},
		{
			name:        "unknown bad request",
			status:      http.StatusBadRequest,
			body:        `{"Error":"other error"}`,
			expectedErr: "unknown bad request error: other error",
		},
		{
			name:        "malformed success",
			status:      http.StatusOK,
			body:        "not json",
			expectedErr: "cant unpack result json:",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(testCase.status)
				_, _ = io.WriteString(w, testCase.body)
			}))
			defer ts.Close()

			_, err := NewSearchClient(token, ts.URL).FindUsers(SearchRequest{Limit: 1})
			if err == nil {
				t.Fatal("expected an error")
			}
			if len(err.Error()) < len(testCase.expectedErr) ||
				err.Error()[:len(testCase.expectedErr)] != testCase.expectedErr {
				t.Fatalf("unexpected error: got %q, want prefix %q",
					err.Error(), testCase.expectedErr)
			}
		})
	}
}

func TestSearchServerInvalidParameters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	for _, parameter := range []string{"order_by", "limit", "offset"} {
		t.Run(parameter, func(t *testing.T) {
			req, err := http.NewRequest(
				http.MethodGet,
				ts.URL+"?order_by=1&limit=1&offset=0",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			query := req.URL.Query()
			query.Set(parameter, "invalid")
			req.URL.RawQuery = query.Encode()
			req.Header.Set("AccessToken", token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("unexpected status: got %d, want %d",
					resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestSearchServerMalformedDataset(t *testing.T) {
	file, err := os.CreateTemp("", "dataset-*.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString("<root><row>"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	oldPath := datasetPath
	datasetPath = file.Name()
	t.Cleanup(func() {
		datasetPath = oldPath
	})

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	_, err = NewSearchClient(token, ts.URL).FindUsers(SearchRequest{Limit: 1})
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "SearchServer fatal error" {
		t.Fatalf("unexpected error: %q", err)
	}
}
