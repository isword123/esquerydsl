// Package esquerydsl exposes various structs and a json marshal-er that makes it easier
// to safely create complex ES Search Queries via the Query DSL
package esquerydsl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// QueryType is used to manage the various querydsl types supported by ES
// We use this type as an enum, essentially to more safely handle the various
// string tokens that denote various querying modes
type QueryType int

// These are the currently supported esquery types
const (
	Match QueryType = iota
	Term
	Terms
	Wildcard
	Range
	Exists
	QueryString
	Nested
)

// QueryTypeErr is a custom err returned if we are trying to stringify
// an unsupported QueryType int
type QueryTypeErr struct {
	typeVal QueryType
}

func (e *QueryTypeErr) Error() string {
	return fmt.Sprintf("Type %d is not supported", e.typeVal)
}

func (qt QueryType) String() (string, error) {
	convs := [...]string{
		"match",
		"term",
		"terms",
		"wildcard",
		"range",
		"exists",
		"query_string",
		"nested",
	}
	if int(qt) > len(convs) {
		return "", &QueryTypeErr{typeVal: qt}
	}

	return convs[qt], nil
}

// QueryDoc is the main public struct that ought to be used to
// construct our querydsl JSON bodies. This struct marshals into
// a spec complaint ES querydsl JSON string
type QueryDoc struct {
	Index       string
	From        int
	Size        int
	Sort        []map[string]string
	SearchAfter []string
	And         []QueryItem
	Not         []QueryItem
	Or          []QueryItem
	Filter      []QueryItem
	PageSize    int
	NestPath    string // 只在 NestDoc 下有效果
	NestDoc     *QueryDoc
	MatchAll    map[string]interface{}
	// https://www.elastic.co/guide/en/elasticsearch/reference/master/search-your-data.html#track-total-hits
	TrackTotalHits bool
}

// QueryItem is used to construct the specific query type json bodies
// for example if we want a "match" query, the Type attr should be "Match"
// the Field attr should be the document attr we want to query against
// and the Value attr should be the actual search term
type QueryItem struct {
	Field     string
	Value     interface{}
	Type      QueryType
	NestedDoc *QueryDoc // 专门针对 nested 节点, 其他字段都不重要
}

// WrapQueryItems is to build nested queries
func WrapQueryItems(itemType string, items ...QueryItem) QueryItem {
	queryDoc := QueryDoc{}
	switch strings.ToLower(itemType) {
	case "or":
		queryDoc.Or = items
	case "not":
		queryDoc.Not = items
	case "filter":
		queryDoc.Filter = items
	default:
		queryDoc.And = items
	}

	return QueryItem{
		Type:  Nested,
		Value: queryDoc,
	}
}

// Builds a JSON string as follows:
// {
//     "query": {
//         "bool": {
//             "must": [ ... ]
//             "should": [ ... ]
//             "filter": [ ... ]
//         }
//     }
// }
type queryReqDoc struct {
	Query          queryWrap           `json:"query,omitempty"`
	From           int                 `json:"from,omitempty"`
	Size           int                 `json:"size,omitempty"`
	Sort           []map[string]string `json:"sort,omitempty"`
	SearchAfter    []string            `json:"search_after,omitempty"`
	TrackTotalHits bool                `json:"track_total_hits,omitempty"`
}

type queryWrap struct {
	Bool     *boolWrap       `json:"bool,omitempty"`
	Nested   *nestedWrap     `json:"nested,omitempty"`
	MatchAll *MatchAllObject `json:"match_all,omitempty"`
}

type MatchAllObject map[string]interface{}

type nestedWrap struct {
	Path  string    `json:"path"`
	Query queryWrap `json:"query"`
}

type boolWrap struct {
	AndList    []leafQuery `json:"must,omitempty"`
	NotList    []leafQuery `json:"must_not,omitempty"`
	OrList     []leafQuery `json:"should,omitempty"`
	FilterList []leafQuery `json:"filter,omitempty"`
}

type leafQuery struct {
	Type      QueryType
	Name      string
	Value     interface{}
	NestedDoc *QueryDoc // 针对 nested 节点, 需要和其他叶子节点组合时的情况
}

func (q leafQuery) handleMarshalType(queryType string) ([]byte, error) {
	// lowercase wildcard queries
	if q.Type == Wildcard {
		if s, ok := q.Value.(string); ok {
			q.Value = strings.ToLower(s)
		}
	}

	if q.Type == QueryString {
		return q.handleMarshalQueryString(queryType)
	}

	// 针对 nested 节点
	if q.NestedDoc != nil {
		qw := queryWrap{}
		qw.Nested = &nestedWrap{
			Path:  q.NestedDoc.NestPath,
			Query: getWrappedQuery(*q.NestedDoc),
		}
		return json.Marshal(qw)
	}

	return json.Marshal(map[string]interface{}{
		(queryType): map[string]interface{}{
			(q.Name): q.Value,
		},
	})
}

func (q leafQuery) handleMarshalQueryString(queryType string) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		queryType: map[string]interface{}{
			"fields":           []string{q.Name},
			"query":            sanitizeElasticQueryField(q.Value.(string)),
			"analyze_wildcard": true, // TODO: make this configurable
		},
	})
}

func getWrappedQuery(query QueryDoc) queryWrap {
	boolDoc := &boolWrap{}
	if len(query.And) > 0 {
		boolDoc.AndList = updateList(query.And)
	}
	if len(query.Not) > 0 {
		boolDoc.NotList = updateList(query.Not)
	}
	if len(query.Or) > 0 {
		boolDoc.OrList = updateList(query.Or)
	}
	if len(query.Filter) > 0 {
		boolDoc.FilterList = updateList(query.Filter)
	}
	qw := queryWrap{}

	if query.NestDoc != nil {
		qw.Nested = &nestedWrap{
			Path:  query.NestDoc.NestPath,
			Query: getWrappedQuery(*query.NestDoc),
		}
	}

	if query.MatchAll != nil {
		match := MatchAllObject(query.MatchAll)
		qw.MatchAll = &match
	}

	if len(boolDoc.AndList) > 0 || len(boolDoc.OrList) > 0 || len(boolDoc.NotList) > 0 || len(boolDoc.FilterList) > 0 {
		qw.Bool = boolDoc
	}

	return qw
}

func (q leafQuery) MarshalJSON() ([]byte, error) {
	if q.Type == Nested {
		return json.Marshal(getWrappedQuery(q.Value.(QueryDoc)))
	}

	var queryType string
	var err error
	if queryType, err = q.Type.String(); err != nil {
		return []byte(""), err
	}

	return q.handleMarshalType(queryType)
}

func updateList(queryItems []QueryItem) []leafQuery {
	leafQueries := make([]leafQuery, 0)
	for _, item := range queryItems {
		leafQueries = append(leafQueries, leafQuery{
			Type:      item.Type,
			Name:      item.Field,
			Value:     item.Value,
			NestedDoc: item.NestedDoc,
		})
	}
	return leafQueries
}

// MarshalJSON will convert QueryDoc struct into
// valid and spec compliant JSON representation
func (query QueryDoc) MarshalJSON() ([]byte, error) {
	queryReq := queryReqDoc{
		Query:          getWrappedQuery(query),
		Size:           query.Size,
		From:           query.From,
		Sort:           query.Sort,
		SearchAfter:    query.SearchAfter,
		TrackTotalHits: query.TrackTotalHits,
	}

	requestBody, err := json.Marshal(queryReq)
	if err != nil {
		return nil, err
	}

	return requestBody, nil
}

// MultiSearchDoc constructs document format for multisearch functionality using Query DSL
func MultiSearchDoc(queries []QueryDoc) (string, error) {
	var requestBuilder strings.Builder
	for _, query := range queries {
		body, err := json.Marshal(query)
		if err != nil {
			return "", err
		}
		requestBuilder.WriteString(fmt.Sprintf(`{"index":"%s"}`, query.Index) + "\n")
		requestBuilder.WriteString(string(body) + "\n")
	}

	return requestBuilder.String(), nil
}

// Elasticsearch defines a set of "reserved keywords" that MUST be escaped
// in order to be queryable. More info can be found in the docs:
// BASE: https://www.elastic.co/guide/en/elasticsearch/reference/current ...
// /query-dsl-query-string-query.html#_reserved_characters
var reserved = []string{"\\", "+", "=", "&&", "||", "!", "(", ")", "{", "}", "[", "]", "^", "\"", "~", "*", "?", ":", "/"}

func sanitizeElasticQueryField(keyword string) string {
	sanitizedKeyword := keyword
	for _, char := range reserved {
		if strings.Contains(sanitizedKeyword, char) {
			replaceWith := `\` + char
			sanitizedKeyword = strings.ReplaceAll(sanitizedKeyword, char, replaceWith)
		}
	}
	return sanitizedKeyword
}
