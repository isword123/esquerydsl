package esquerydsl

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestBogusQueryType(t *testing.T) {
	_, err := json.Marshal(QueryDoc{
		Index: "some_index",
		Sort:  []map[string]string{{"id": "asc"}},
		And: []QueryItem{
			{
				Field: "some_index_id",
				Value: "some-long-key-id-value",
				Type:  100001,
			},
		},
	})

	var queryTypeErr *QueryTypeErr
	if !errors.As(err, &queryTypeErr) {
		t.Errorf("\nUnexpected error: %v", err)
	}
}

func TestQueryStringEsc(t *testing.T) {
	body, _ := json.Marshal(QueryDoc{
		Index: "some_index",
		And: []QueryItem{
			{
				Field: "user.id",
				Value: "kimchy!",
				Type:  QueryString,
			},
		},
	})

	expected := `{"query":{"bool":{"must":[{"query_string":{"analyze_wildcard":true,"fields":["user.id"],"query":"kimchy\\!"}}]}}}`
	if string(body) != expected {
		t.Errorf("\nWant: %q\nHave: %q", expected, string(body))
	}
}

func TestMultiSearchDoc(t *testing.T) {
	doc, _ := MultiSearchDoc([]QueryDoc{
		{
			Index: "index1",
			And: []QueryItem{
				{
					Field: "user.id",
					Value: "kimchy!",
					Type:  QueryString,
				},
			},
		},
		{
			Index: "index2",
			And: []QueryItem{
				{
					Field: "some_index_id",
					Value: "some-long-key-id-value",
					Type:  Match,
				},
			},
		},
	})

	expected := `{"index":"index1"}
{"query":{"bool":{"must":[{"query_string":{"analyze_wildcard":true,"fields":["user.id"],"query":"kimchy\\!"}}]}}}
{"index":"index2"}
{"query":{"bool":{"must":[{"match":{"some_index_id":"some-long-key-id-value"}}]}}}
`
	if string(doc) != expected {
		t.Errorf("\nWant: %q\nHave: %q", expected, string(doc))
	}
}

func TestAndQuery(t *testing.T) {
	body, _ := json.Marshal(QueryDoc{
		Index: "some_index",
		Sort:  []map[string]string{{"id": "asc"}},
		And: []QueryItem{
			{
				Field: "some_index_id",
				Value: "some-long-key-id-value",
				Type:  Match,
			},
		},
	})

	expected := `{"query":{"bool":{"must":[{"match":{"some_index_id":"some-long-key-id-value"}}]}},"sort":[{"id":"asc"}]}`
	if string(body) != expected {
		t.Errorf("\nWant: %q\nHave: %q", expected, string(body))
	}
}

func TestFilterQuery(t *testing.T) {
	body, _ := json.Marshal(QueryDoc{
		Index: "some_index",
		And: []QueryItem{
			{
				Field: "title",
				Value: "Search",
				Type:  Match,
			},
			{
				Field: "content",
				Value: "Elasticsearch",
				Type:  Match,
			},
		},
		Filter: []QueryItem{
			{
				Field: "status",
				Value: "published",
				Type:  Term,
			},
			{
				Field: "publish_date",
				Value: map[string]string{
					"gte": "2015-01-01",
				},
				Type: Range,
			},
		},
	})

	expected := `{"query":{"bool":{"must":[{"match":{"title":"Search"}},{"match":{"content":"Elasticsearch"}}],"filter":[{"term":{"status":"published"}},{"range":{"publish_date":{"gte":"2015-01-01"}}}]}}}`
	if string(body) != expected {
		t.Errorf("\nWant: %q\nHave: %q", expected, string(body))
	}
}

func TestNestedQuery(t *testing.T) {
	nestedItem := WrapQueryItems("and", []QueryItem{
		{
			Field: "date.year",
			Value: 2001,
			Type:  Term,
		},
		{
			Field: "date.month",
			Value: 1002,
			Type:  Term,
		},
	}...)
	doc := QueryDoc{
		Index: "123",
		NestDoc: &QueryDoc{
			And: []QueryItem{
				{
					Field: "name",
					Value: "name1",
					Type:  Term,
				},
				{
					Field: "password",
					Value: "password1",
					Type:  Term,
				},
			},
			NestPath: "hello",
		},
		And: []QueryItem{
			{
				Field: "user.country",
				Value: 1001,
				Type:  Term,
			},
			nestedItem,
		},
	}

	// bs, _ := json.MarshalIndent(doc, "", "  ")

	bs, _ := json.Marshal(doc)
	expected := `{"query":{"bool":{"must":[{"term":{"user.country":1001}},{"bool":{"must":[{"term":{"date.year":2001}},{"term":{"date.month":1002}}]}}]},"nested":{"path":"hello","query":{"bool":{"must":[{"term":{"name":"name1"}},{"term":{"password":"password1"}}]}}}}}`
	if string(bs) != expected {
		t.Errorf("\nWant: %q\nHave: %q", expected, string(bs))
	}

	subjectItem := QueryItem{Field: "subject.org_contact_name", Value: "xx", Type: Match}
	webItem := QueryItem{Field: "web.site_person_name", Value: "xx", Type: Match}
	nestedItem1 := QueryItem{
		NestedDoc: &QueryDoc{
			NestPath: "web",
			And:      []QueryItem{webItem},
		},
	}
	anotherDoc := &QueryDoc{
		Index: "hello",
		And:   []QueryItem{subjectItem, nestedItem1},
	}

	bs, _ = json.MarshalIndent(anotherDoc, "", "  ")
	fmt.Println(string(bs))
}
