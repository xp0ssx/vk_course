package main

import (
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	token      = "secret"
	wrongToken = "lies"
)

var (
	datasetPath      = "dataset.xml"
	wrongDatasetPath = "missing.txt"
)

type xmlUser struct {
	Id        int    `xml:"id"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Gender    string `xml:"gender"`
	About     string `xml:"about"`
}

type xmlDataset struct {
	Users []xmlUser `xml:"row"`
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != token {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	query, orderField, orderBy, offset, limit, flag := getParam(r.URL.Query(), w)
	if !flag {
		return
	}

	switch orderField {
	case "", "id", "age", "name":
	default:
		badRequest(w)
		return
	}

	switch orderBy {
	case OrderByDesc, OrderByAsIs, OrderByAsc:
	default:
		badRequest(w)
		return
	}

	f, err := os.Open(datasetPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	var dataset xmlDataset
	if err := xml.NewDecoder(f).Decode(&dataset); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	selectedEntries := make([]*User, 0, len(dataset.Users))
	for _, val := range dataset.Users {
		if query == "" || strings.Contains(val.FirstName+" "+val.LastName, query) || strings.Contains(val.About, query) {
			selectedEntries = append(selectedEntries, xmlToUser(&val))
		}
	}

	sortUsers(selectedEntries, orderField, orderBy)
	selectedEntries = limitUsers(selectedEntries, offset, limit)

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(selectedEntries); err != nil {
		return
	}

}

func xmlToUser(xml *xmlUser) *User {

	return &User{
		ID:     xml.Id,
		Name:   xml.FirstName + " " + xml.LastName,
		Age:    xml.Age,
		About:  xml.About,
		Gender: xml.Gender,
	}
}

func getParam(params url.Values, w http.ResponseWriter) (query, orderField string, orderBy, offset, limit int, flag bool) {
	query = params.Get("query")
	orderField = params.Get("order_field")

	orderByString := params.Get("order_by")
	orderBy, err := strconv.Atoi(orderByString)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	limitString := params.Get("limit")
	limit, err = strconv.Atoi(limitString)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	offsetString := params.Get("offset")
	offset, err = strconv.Atoi(offsetString)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	return query, orderField, orderBy, offset, limit, true
}

func badRequest(w http.ResponseWriter) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	err := json.NewEncoder(w).Encode(SearchErrorResponse{Error: ErrorBadOrderField})
	if err != nil {
		log.Printf("failed to encode error response: %v", err)
	}
}

func sortUsers(users []*User, orderField string, orderBy int) {
	if orderBy == OrderByAsIs {
		return
	}

	less := func(a, b *User) bool {
		switch orderField {
		case "id":
			return a.ID < b.ID
		case "age":
			return a.Age < b.Age
		case "", "name":
			return a.Name < b.Name
		default:
			return false
		}
	}

	sort.SliceStable(users, func(i, j int) bool {
		if orderBy == OrderByAsc {
			return less(users[i], users[j])
		}
		return less(users[j], users[i])
	})
}

func limitUsers(users []*User, offset, limit int) []*User {
	if offset >= len(users) {
		return []*User{}
	}
	end := min(offset+limit, len(users))
	return users[offset:end]
}
