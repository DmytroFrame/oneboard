// Package searchindex implements full-text card search on OpenSearch.
package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/board"
	"clodapp/backend/internal/usecase/project"
)

var Module = fx.Module("opensearch",
	fx.Provide(
		NewCardIndex,
		func(i *CardIndex) board.SearchIndex { return i },
		func(i *CardIndex) project.SearchIndex { return i },
	),
)

// Words are indexed as edge n-grams so that typing the beginning of any word matches
// ("лог" finds "логіном"); the query side uses the plain standard analyzer.
const indexSettings = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "filter": { "prefix": { "type": "edge_ngram", "min_gram": 1, "max_gram": 20 } },
      "analyzer": { "prefix": { "tokenizer": "standard", "filter": ["lowercase", "prefix"] } }
    }
  },
  "mappings": {
    "properties": {
      "project_id":  { "type": "long" },
      "key":         { "type": "text", "analyzer": "prefix", "search_analyzer": "standard" },
      "title":       { "type": "text", "analyzer": "prefix", "search_analyzer": "standard" },
      "description": { "type": "text", "analyzer": "prefix", "search_analyzer": "standard" }
    }
  }
}`

const bulkChunk = 500

type CardIndex struct {
	client *opensearchapi.Client
	index  string
}

type doc struct {
	ProjectID   int64  `json:"project_id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func NewCardIndex(cfg config.Config) (*CardIndex, error) {
	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses:          []string{cfg.OpenSearchURL},
			Username:           cfg.OpenSearchUser,
			Password:           cfg.OpenSearchPassword,
			InsecureSkipVerify: cfg.OpenSearchInsecure,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opensearch: %w", err)
	}
	return &CardIndex{client: client, index: cfg.OpenSearchIndex}, nil
}

func status(resp *opensearch.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func (i *CardIndex) Ensure(ctx context.Context) error {
	// OpenSearch takes a while to boot next to the app in docker compose.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for {
		resp, err := i.client.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{Indices: []string{i.index}})
		switch status(resp) {
		case http.StatusOK:
			return nil
		case http.StatusNotFound:
			_, err := i.client.Indices.Create(ctx, opensearchapi.IndicesCreateReq{Index: i.index, Body: strings.NewReader(indexSettings)})
			if err != nil {
				return fmt.Errorf("opensearch: create index: %w", err)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("opensearch: %w", err)
		case <-time.After(2 * time.Second):
		}
	}
}

func (i *CardIndex) Count(ctx context.Context) (int, error) {
	resp, err := i.client.Indices.Count(ctx, &opensearchapi.IndicesCountReq{Indices: []string{i.index}})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (i *CardIndex) Upsert(ctx context.Context, docs ...domain.SearchDoc) error {
	for start := 0; start < len(docs); start += bulkChunk {
		var body bytes.Buffer
		enc := json.NewEncoder(&body)
		for _, d := range docs[start:min(start+bulkChunk, len(docs))] {
			_ = enc.Encode(map[string]any{"index": map[string]string{"_id": strconv.FormatInt(d.ID, 10)}})
			_ = enc.Encode(doc{ProjectID: d.ProjectID, Key: d.Key, Title: d.Title, Description: d.Description})
		}
		resp, err := i.client.Bulk(ctx, opensearchapi.BulkReq{
			Index:  i.index,
			Body:   &body,
			Params: opensearchapi.BulkParams{Refresh: "wait_for"}, // visible to the next search
		})
		if err != nil {
			return err
		}
		if resp.Errors {
			return fmt.Errorf("opensearch: bulk index reported errors")
		}
	}
	return nil
}

func (i *CardIndex) Delete(ctx context.Context, cardID int64) error {
	resp, err := i.client.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      i.index,
		DocumentID: strconv.FormatInt(cardID, 10),
		Params:     opensearchapi.DocumentDeleteParams{Refresh: "wait_for"},
	})
	if err != nil && resp != nil && status(resp.Inspect().Response) == http.StatusNotFound {
		return nil
	}
	return err
}

func (i *CardIndex) DeleteProject(ctx context.Context, projectID int64) error {
	refresh := true
	body := fmt.Sprintf(`{"query":{"term":{"project_id":%d}}}`, projectID)
	_, err := i.client.Document.DeleteByQuery(ctx, opensearchapi.DocumentDeleteByQueryReq{
		Indices: []string{i.index},
		Body:    strings.NewReader(body),
		Params:  opensearchapi.DocumentDeleteByQueryParams{Refresh: &refresh},
	})
	return err
}

func (i *CardIndex) Search(ctx context.Context, projectID int64, query string, limit int) ([]int64, error) {
	body, _ := json.Marshal(map[string]any{
		"size":    limit,
		"_source": false,
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{map[string]any{"term": map[string]any{"project_id": projectID}}},
				"must": map[string]any{
					"multi_match": map[string]any{
						"query":    query,
						"fields":   []string{"key^5", "title^3", "description"},
						"operator": "and",
					},
				},
			},
		},
	})
	resp, err := i.client.Search(ctx, &opensearchapi.SearchReq{Indices: []string{i.index}, Body: bytes.NewReader(body)})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(resp.Hits.Hits))
	for _, h := range resp.Hits.Hits {
		if id, err := strconv.ParseInt(h.ID, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
