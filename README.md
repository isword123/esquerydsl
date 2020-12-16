![build](https://github.com/mottaquikarim/esquerydsl/workflows/Build%20Status/badge.svg) [![GoDoc](https://godoc.org/github.com/mottaquikarim/esquerydsl?status.svg)](https://godoc.org/github.com/mottaquikarim/esquerydsl)
# [ES Query DSL](https://godoc.org/github.com/mottaquikarim/esquerydsl)
Structs and marshal-ers that help wrangle writing elastic search queries using the [ES query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html) spec

## Installation

```bash
go get github.com/mottaquikarim/esquerydsl
```

## Usage

```go

package main
import (
	"github.com/mottaquikarim/esquerydsl"
)

func main() {
	_, body, _ := esquerydsl.GetQueryBlock(QueryDoc{
		Index: "some_index",
		Sort:  []map[string]string{map[string]string{"id": "asc"}},
		And: []QueryItem{
			QueryItem{
				Field: "some_index_id",
				Value: "some-long-key-id-value",
				Type:  "match",
			},
		},
	})
	fmt.Println(body)
	// {"query":{"bool":{"must":[{"match":{"some_index_id":"some-long-key-id-value"}}]}},"sort":[{"id":"asc"}]}
}
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

### Run Tests

```bash
make test
```

### Format

```bash
make fmt
```

### Lint

```bash
make lint
```

## License
[MIT](https://choosealicense.com/licenses/mit/)
